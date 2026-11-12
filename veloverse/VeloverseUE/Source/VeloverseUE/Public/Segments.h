// Segments: timed stretches of a route (its climbs, and flat sprints), an effort timer that runs as
// the rider passes through them, and a ghost of the rider's best effort to race. Distances are
// metres along the track; a closed track's are taken unwrapped, so laps count on. Plain C++, tested
// in tests/segments_test.cpp.
#pragma once

#include <algorithm>
#include <cmath>
#include <optional>
#include <string>
#include <vector>

namespace Veloverse
{
struct FSegment
{
	std::string Name;
	double Start = 0.0, End = 0.0; // metres along the track; End < Start wraps a closed track's seam
	bool bClimb = false;
	double Rise = 0.0;    // metres, a climb's foot to its top
	double Length(double TrackLength) const { return End >= Start ? End - Start : End + TrackLength - Start; }
	double Grade(double TrackLength) const
	{
		const double L = Length(TrackLength);
		return L > 0.0 ? Rise / L : 0.0;
	}
};

// One ride through a segment: its time, and when it was at each Step metres along it (for a ghost).
struct FSegmentEffort
{
	double Seconds = 0.0;
	double AverageWatts = 0.0;
	double Step = 10.0;
	std::vector<double> Splits; // Splits[i]: seconds from the start to i * Step metres in

	// Seconds the effort took to reach Metres into the segment, interpolated.
	double TimeAt(double Metres) const
	{
		if (Splits.empty())
		{
			return 0.0;
		}
		const double F = std::max(Metres, 0.0) / Step;
		const std::size_t I = static_cast<std::size_t>(F);
		if (I + 1 >= Splits.size())
		{
			return Splits.back();
		}
		return Splits[I] + (Splits[I + 1] - Splits[I]) * (F - I);
	}

	// Metres into the segment the effort had ridden after At seconds.
	double MetresAt(double At) const
	{
		if (Splits.size() < 2)
		{
			return 0.0;
		}
		const auto It = std::upper_bound(Splits.begin(), Splits.end(), At);
		if (It == Splits.begin())
		{
			return 0.0;
		}
		if (It == Splits.end())
		{
			return Step * (Splits.size() - 1);
		}
		const std::size_t I = static_cast<std::size_t>(It - Splits.begin()) - 1;
		const double Span = Splits[I + 1] - Splits[I];
		return Step * (I + (Span > 0.0 ? (At - Splits[I]) / Span : 0.0));
	}
};

// ---- Finding the segments ------------------------------------------------------------------------

// The climbs in a height profile sampled every SampleM metres: from a foot to a top that rises at
// least MinRise at an average of MinGrade or more, ignoring dips shallower than Dip on the way up.
// Named "<Prefix> climb N" in order along the track.
inline std::vector<FSegment> FindClimbs(const std::vector<double>& Heights, double SampleM, bool bClosed,
                                        double MinRise = 30.0, double MinGrade = 0.03, double Dip = 8.0)
{
	std::vector<FSegment> Out;
	const std::size_t N = Heights.size();
	if (N < 3)
	{
		return Out;
	}
	// Walk once along the track, tracking the lowest point since the last top and the highest since
	// that foot. Round a loop the walk starts at the lowest point, which no climb can pass through,
	// and goes once round, so a climb over the seam is found whole.
	const std::size_t Origin =
		bClosed ? static_cast<std::size_t>(std::min_element(Heights.begin(), Heights.end()) - Heights.begin()) : 0;
	const std::size_t Steps = bClosed ? N + 1 : N;
	auto H = [&](std::size_t I) { return Heights[(Origin + I) % N]; };
	std::size_t Foot = 0, Top = 0;
	for (std::size_t I = 1; I < Steps; ++I)
	{
		if (H(I) > H(Top))
		{
			Top = I;
		}
		if (H(Top) - H(I) >= Dip || I + 1 == Steps)
		{
			// The climb from Foot to Top is over: keep it if it's big and steep enough.
			// Trim a gentle lead-in: the foot is where the road starts to climb in earnest.
			constexpr std::size_t Look = 5;
			while (Foot + Look <= Top && (H(Foot + Look) - H(Foot)) / (Look * SampleM) < MinGrade * 0.5)
			{
				++Foot;
			}
			const double Rise = H(Top) - H(Foot);
			const double Run = (Top - Foot) * SampleM;
			if (Top > Foot && Rise >= MinRise && Run > 0.0 && Rise / Run >= MinGrade)
			{
				FSegment S;
				S.bClimb = true;
				S.Rise = Rise;
				S.Start = static_cast<double>((Origin + Foot) % N) * SampleM;
				S.End = static_cast<double>((Origin + Top) % N) * SampleM;
				Out.push_back(S);
			}
			Foot = Top = I;
		}
		if (H(I) <= H(Foot)) // along the flat too: a climb starts where the flat ends
		{
			Foot = Top = I;
		}
	}
	std::sort(Out.begin(), Out.end(), [](const FSegment& A, const FSegment& B) { return A.Start < B.Start; });
	for (std::size_t I = 0; I < Out.size(); ++I)
	{
		Out[I].Name = "Climb " + std::to_string(I + 1);
	}
	return Out;
}

// ---- Timing an effort ----------------------------------------------------------------------------

// Watches the rider's unwrapped distance and times each pass through one segment. Update returns a
// finished effort the frame the rider crosses the end.
class FSegmentTimer
{
public:
	FSegmentTimer() = default;
	FSegmentTimer(FSegment InSegment, double InTrackLength, bool bInClosed, double InStep = 10.0)
		: Segment(std::move(InSegment)), TrackLength(InTrackLength), bClosed(bInClosed), Step(InStep)
	{
	}

	const FSegment& Seg() const { return Segment; }
	bool Running() const { return bRunning; }
	double Elapsed() const { return Clock - StartedAt; }
	double Into() const { return bRunning ? Last - StartM : 0.0; } // metres into the segment

	// Distance is metres along the track, unwrapped (it keeps growing lap on lap); Now is ride seconds.
	std::optional<FSegmentEffort> Update(double Distance, double Now, double Watts)
	{
		std::optional<FSegmentEffort> Finished;
		if (HasLast && Distance > Last)
		{
			const double Length = Segment.Length(TrackLength);
			if (!bRunning)
			{
				// The start of the next pass at or after Last.
				const double Start = NextStart(Last);
				if (Distance >= Start && Start >= Last)
				{
					bRunning = true;
					StartM = Start;
					StartedAt = Cross(Start, Distance, Now);
					Joules = 0.0;
					Effort = FSegmentEffort();
					Effort.Step = Step;
					Effort.Splits.push_back(0.0);
				}
			}
			if (bRunning)
			{
				const double From = std::max(Last, StartM);
				const double To = std::min(Distance, StartM + Length);
				const double Share = (Distance - Last) > 0.0 ? (To - From) / (Distance - Last) : 0.0;
				Joules += Watts * (Now - LastTime) * std::max(Share, 0.0);
				// A split for every Step metres passed.
				while (StartM + Step * Effort.Splits.size() <= std::min(Distance, StartM + Length))
				{
					Effort.Splits.push_back(Cross(StartM + Step * Effort.Splits.size(), Distance, Now) - StartedAt);
				}
				if (Distance >= StartM + Length)
				{
					Effort.Seconds = Cross(StartM + Length, Distance, Now) - StartedAt;
					if (Effort.Splits.back() < Effort.Seconds - 1e-9)
					{
						Effort.Splits.push_back(Effort.Seconds); // the end, if it isn't on a step
					}
					Effort.AverageWatts = Effort.Seconds > 0.0 ? Joules / Effort.Seconds : 0.0;
					Finished = Effort;
					bRunning = false;
				}
			}
		}
		Last = Distance;
		LastTime = Now;
		Clock = Now;
		HasLast = true;
		return Finished;
	}

	// Lost the thread (a jump to another start): drop any effort under way.
	void Reset()
	{
		bRunning = false;
		HasLast = false;
	}

private:
	FSegment Segment;
	double TrackLength = 0.0;
	bool bClosed = false;
	double Step = 10.0;

	bool HasLast = false;
	double Last = 0.0, LastTime = 0.0, Clock = 0.0;
	bool bRunning = false;
	double StartM = 0.0, StartedAt = 0.0, Joules = 0.0;
	FSegmentEffort Effort;

	// The time the rider was at Metres, between the last update and this one.
	double Cross(double Metres, double Distance, double Now) const
	{
		const double F = Distance > Last ? (Metres - Last) / (Distance - Last) : 1.0;
		return LastTime + (Now - LastTime) * std::clamp(F, 0.0, 1.0);
	}

	// The first start of the segment at or after Metres (unwrapped).
	double NextStart(double Metres) const
	{
		if (!bClosed || TrackLength <= 0.0)
		{
			return Segment.Start;
		}
		const double Lap = std::floor((Metres - Segment.Start) / TrackLength);
		double Start = Segment.Start + Lap * TrackLength;
		if (Start < Metres)
		{
			Start += TrackLength;
		}
		return Start;
	}
};

// ---- The ghost -----------------------------------------------------------------------------------

// Seconds ahead (negative) or behind (positive) the ghost, at Metres into the segment after Elapsed.
inline double GhostGap(const FSegmentEffort& Ghost, double Metres, double Elapsed)
{
	return Elapsed - Ghost.TimeAt(Metres);
}

// ---- History -------------------------------------------------------------------------------------

struct FSegmentRecord
{
	std::string When; // ISO 8601
	double Seconds = 0.0;
	double AverageWatts = 0.0;
};

constexpr double TieSeconds = 0.05;

// Where a new time ranks among the past ones (1 is a personal best), and the best before it.
struct FRanking
{
	int Rank = 1;
	int Of = 1;
	std::optional<double> PreviousBest;
	// A personal record: faster than a best there was (a first time isn't one, nor a tie).
	bool Pr(double Seconds) const { return PreviousBest && Seconds < *PreviousBest - TieSeconds; }
};

inline FRanking Rank(const std::vector<FSegmentRecord>& Past, double Seconds)
{
	FRanking R;
	R.Of = static_cast<int>(Past.size()) + 1;
	for (const FSegmentRecord& P : Past)
	{
		if (P.Seconds < Seconds - TieSeconds) // history keeps tenths: a tie isn't a loss
		{
			++R.Rank;
		}
		if (!R.PreviousBest || P.Seconds < *R.PreviousBest)
		{
			R.PreviousBest = P.Seconds;
		}
	}
	return R;
}
} // namespace Veloverse
