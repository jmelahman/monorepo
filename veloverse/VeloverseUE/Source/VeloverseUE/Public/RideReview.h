// The numbers the ride review and the stats page draw, beyond a ride's summary (RideStats.h): the
// power curve, time in heart-rate zones, a cadence histogram, a ride's traces by distance, and
// fitness and fatigue from daily training load. Plain C++ with no engine types, so it can be tested
// on its own (tests/ride_review_test.cpp).
#pragma once

#include "RideStats.h"

#include <algorithm>
#include <array>
#include <cmath>
#include <cstddef>
#include <vector>

namespace Veloverse
{
// The power curve's durations, in seconds: its x axis.
constexpr std::array<int, 12> CurveDurations = {1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600, 7200};
using FCurve = std::array<double, CurveDurations.size()>;

// The best average power over each of CurveDurations, 0 past the ride's length.
inline FCurve PowerCurve(const std::vector<FSample>& Samples)
{
	FCurve Out = {};
	for (std::size_t I = 0; I < CurveDurations.size(); ++I)
	{
		Out[I] = BestAverage(Samples, static_cast<std::size_t>(CurveDurations[I]));
	}
	return Out;
}

// The better of two curves at every duration.
inline FCurve Best(const FCurve& A, const FCurve& B)
{
	FCurve Out;
	for (std::size_t I = 0; I < A.size(); ++I)
	{
		Out[I] = std::max(A[I], B[I]);
	}
	// A best held for longer also stands for every shorter duration, which fills the gaps older
	// rides leave (they kept only RideStats.h's BestDurations) so the curve never climbs again.
	for (std::size_t I = Out.size() - 1; I-- > 0;)
	{
		Out[I] = std::max(Out[I], Out[I + 1]);
	}
	return Out;
}

// Five heart-rate zones as shares of the rider's maximum: the top of zones 1 to 4.
constexpr std::array<double, 4> HeartZoneTops = {0.60, 0.70, 0.80, 0.90};

inline int HeartZone(double Bpm, double MaxBpm)
{
	int Z = 0;
	while (Z < 4 && Bpm >= HeartZoneTops[Z] * MaxBpm)
	{
		++Z;
	}
	return Z;
}

// Seconds in each heart-rate zone; seconds without a reading don't count.
inline std::array<double, 5> HeartZoneSeconds(const std::vector<FSample>& Samples, double MaxBpm)
{
	std::array<double, 5> Out = {};
	for (const FSample& S : Samples)
	{
		if (S.HeartRate > 0.0)
		{
			Out[HeartZone(S.HeartRate, std::max(MaxBpm, 1.0))] += 1.0;
		}
	}
	return Out;
}

// Seconds pedalled in each cadence band, From to To rpm in Width steps; the first and last bands
// take everything below and above. Coasting doesn't count.
struct FCadenceBands
{
	double From = 60.0, To = 120.0, Width = 5.0;
	std::vector<double> Seconds;
	int Count() const { return static_cast<int>(Seconds.size()); }
	// The band with the most time, -1 when nothing was pedalled.
	int Peak() const
	{
		int Best = -1;
		for (int I = 0; I < Count(); ++I)
		{
			if (Seconds[I] > 0.0 && (Best < 0 || Seconds[I] > Seconds[Best]))
			{
				Best = I;
			}
		}
		return Best;
	}
	double Low(int Band) const { return From + Band * Width; }
};

inline FCadenceBands CadenceHistogram(const std::vector<FSample>& Samples, double From = 60.0, double To = 120.0,
                                      double Width = 5.0)
{
	FCadenceBands Out;
	Out.From = From;
	Out.To = To;
	Out.Width = Width;
	const int Count = std::max(1, static_cast<int>(std::round((To - From) / Width)));
	Out.Seconds.assign(Count, 0.0);
	for (const FSample& S : Samples)
	{
		if (S.Cadence > 0.0)
		{
			const int Band = std::clamp(static_cast<int>(std::floor((S.Cadence - From) / Width)), 0, Count - 1);
			Out.Seconds[Band] += 1.0;
		}
	}
	return Out;
}

// A ride's traces against distance: each column the average of the samples whose metres fall in
// it, for drawing. A column no sample fell in takes its neighbour's value.
struct FTraces
{
	double Metres = 0.0; // the ride's length, the traces' width
	std::vector<double> Watts, HeartRate, Cadence, Speed, Elevation;
};

inline FTraces Traces(const std::vector<FSample>& Samples, int Columns)
{
	FTraces Out;
	if (Samples.empty() || Columns <= 0)
	{
		return Out;
	}
	const double Start = Samples.front().Metres;
	Out.Metres = std::max(Samples.back().Metres - Start, 1.0);
	std::vector<double> Count(Columns, 0.0);
	for (auto* Column : {&Out.Watts, &Out.HeartRate, &Out.Cadence, &Out.Speed, &Out.Elevation})
	{
		Column->assign(Columns, 0.0);
	}
	for (const FSample& S : Samples)
	{
		const int C = std::clamp(static_cast<int>((S.Metres - Start) / Out.Metres * Columns), 0, Columns - 1);
		Out.Watts[C] += S.Watts;
		Out.HeartRate[C] += S.HeartRate;
		Out.Cadence[C] += S.Cadence;
		Out.Speed[C] += S.Speed;
		Out.Elevation[C] += S.Elevation;
		Count[C] += 1.0;
	}
	int Last = -1;
	for (int C = 0; C < Columns; ++C)
	{
		for (auto* Column : {&Out.Watts, &Out.HeartRate, &Out.Cadence, &Out.Speed, &Out.Elevation})
		{
			if (Count[C] > 0.0)
			{
				(*Column)[C] /= Count[C];
			}
			else if (Last >= 0)
			{
				(*Column)[C] = (*Column)[Last];
			}
		}
		if (Count[C] > 0.0)
		{
			Last = C;
		}
		else if (Last < 0)
		{
			// Columns before the first sample take the first that has one, once it's found.
			int Next = C + 1;
			while (Next < Columns && Count[Next] == 0.0)
			{
				++Next;
			}
			if (Next < Columns)
			{
				for (auto* Column : {&Out.Watts, &Out.HeartRate, &Out.Cadence, &Out.Speed, &Out.Elevation})
				{
					(*Column)[C] = (*Column)[Next] / Count[Next];
				}
			}
		}
	}
	return Out;
}

// Fitness (chronic training load, 42 days) and fatigue (acute, 7 days): exponentially weighted
// averages of daily TSS. Form is fitness less fatigue, taken the day before, as is usual.
struct FLoad
{
	std::vector<double> Fitness, Fatigue; // a value a day, after that day's riding
	double Form() const
	{
		return Fitness.size() < 2 ? 0.0 : Fitness[Fitness.size() - 2] - Fatigue[Fatigue.size() - 2];
	}
};

inline FLoad TrainingLoad(const std::vector<double>& DailyTss)
{
	FLoad Out;
	double Ctl = 0.0, Atl = 0.0;
	for (const double Tss : DailyTss)
	{
		Ctl += (Tss - Ctl) * (1.0 - std::exp(-1.0 / 42.0));
		Atl += (Tss - Atl) * (1.0 - std::exp(-1.0 / 7.0));
		Out.Fitness.push_back(Ctl);
		Out.Fatigue.push_back(Atl);
	}
	return Out;
}

// What a form number means, in a word.
inline const char* FormWord(double Form)
{
	return Form > 5.0 ? "fresh" : Form > -10.0 ? "neutral" : Form > -30.0 ? "training" : "very tired";
}
} // namespace Veloverse
