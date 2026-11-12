// Checks Segments.h without the engine. Run with tests/run.sh.
#include "Segments.h"

#include <cmath>
#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got = 0.0)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

static bool Near(double A, double B, double Tol = 1e-6) { return std::abs(A - B) <= Tol; }

int main()
{
	// A 10 km loop sampled every 10 m: flat, a 1 km climb at 6% from 2 km, flat, and a climb over
	// the seam (8 km to the end at 4%, then on to 0.5 km), down to the lowest point at 1.5 km.
	std::vector<double> H(1000, 0.0);
	for (int I = 0; I < 1000; ++I)
	{
		const double M = I * 10.0;
		double Z = 20.0;
		if (M >= 2000.0 && M < 3000.0) Z = 20.0 + (M - 2000.0) * 0.06;
		else if (M >= 3000.0 && M < 4000.0) Z = 80.0 - (M - 3000.0) * 0.06;
		else if (M >= 8000.0) Z = 20.0 + (M - 8000.0) * 0.04;
		else if (M < 500.0) Z = 100.0 + M * 0.04;
		else if (M < 1500.0) Z = 120.0 - (M - 500.0) * 0.105;
		else if (M < 2000.0) Z = 15.0 + (M - 1500.0) * 0.01;
		H[I] = Z;
	}
	const std::vector<FSegment> Climbs = FindClimbs(H, 10.0, true);
	Check(Climbs.size() == 2, "two climbs on the loop", static_cast<double>(Climbs.size()));
	if (Climbs.size() == 2)
	{
		Check(Climbs[0].Start >= 1940.0 && Climbs[0].Start <= 2000.0 && Near(Climbs[0].End, 2990.0, 10.0),
		      "the 6% climb, 2 to 3 km", Climbs[0].Start);
		Check(Near(Climbs[0].Grade(10000.0), 0.06, 0.006), "at 6%", Climbs[0].Grade(10000.0) * 100);
		Check(Climbs[1].Start >= 7990.0 && Near(Climbs[1].End, 490.0, 10.0), "the climb over the seam is whole",
		      Climbs[1].End);
		Check(Near(Climbs[1].Length(10000.0), 2500.0, 20.0), "and 2.5 km long", Climbs[1].Length(10000.0));
		Check(Climbs[0].Name == "Climb 1" && Climbs[1].Name == "Climb 2", "named in order");
	}
	Check(FindClimbs(std::vector<double>(100, 5.0), 10.0, false).empty(), "a flat road has no climbs");

	// Timing: a 1 km segment at 2 km, ridden at 10 m/s in uneven frames, twice round a 10 km loop.
	FSegment Seg;
	Seg.Name = "Test";
	Seg.Start = 2000.0;
	Seg.End = 3000.0;
	FSegmentTimer Timer(Seg, 10000.0, true);
	std::vector<FSegmentEffort> Efforts;
	double T = 0.0, D = 1500.0;
	for (int Frame = 0; D < 13500.0; ++Frame)
	{
		const double Dt = Frame % 3 == 0 ? 0.033 : 0.017;
		T += Dt;
		const double Speed = D < 12000.0 ? 10.0 : 12.5; // faster on the second lap
		D += Speed * Dt;
		if (auto E = Timer.Update(D, T, Speed * 20.0))
		{
			Efforts.push_back(*E);
		}
	}
	Check(Efforts.size() == 2, "one effort a lap", static_cast<double>(Efforts.size()));
	if (Efforts.size() == 2)
	{
		Check(Near(Efforts[0].Seconds, 100.0, 1e-6), "1 km at 10 m/s is 100 s", Efforts[0].Seconds);
		Check(Near(Efforts[1].Seconds, 80.0, 0.01), "and at 12.5 m/s, 80 s", Efforts[1].Seconds);
		Check(Near(Efforts[0].AverageWatts, 200.0, 0.5), "its average power", Efforts[0].AverageWatts);
		Check(Efforts[0].Splits.size() == 101, "a split every 10 m, ends included",
		      static_cast<double>(Efforts[0].Splits.size()));
		Check(Near(Efforts[0].TimeAt(500.0), 50.0, 1e-6), "halfway at 50 s", Efforts[0].TimeAt(500.0));
		Check(Near(Efforts[0].MetresAt(25.0), 250.0, 1e-6), "250 m in after 25 s", Efforts[0].MetresAt(25.0));
		// Racing the first lap's ghost on the second: at 400 m in after 32 s, 8 s up on it.
		Check(Near(GhostGap(Efforts[0], 400.0, 32.0), -8.0, 1e-6), "8 s ahead of the ghost",
		      GhostGap(Efforts[0], 400.0, 32.0));
		Check(Near(Efforts[0].MetresAt(1e9), 1000.0, 1e-6), "a ghost waits at the finish",
		      Efforts[0].MetresAt(1e9));
	}

	// A segment over a loop's seam, and an open track that never wraps.
	FSegment Seam;
	Seam.Start = 9500.0;
	Seam.End = 500.0;
	FSegmentTimer Wrap(Seam, 10000.0, true);
	int Done = 0;
	for (double M = 9000.0; M < 11000.0; M += 5.0)
	{
		if (auto E = Wrap.Update(M, M / 10.0, 100.0))
		{
			++Done;
			Check(Near(E->Seconds, 100.0, 1e-6), "a segment over the seam times through it", E->Seconds);
		}
	}
	Check(Done == 1, "and finishes once", Done);
	FSegmentTimer Open(Seg, 10000.0, false);
	Done = 0;
	for (double M = 0.0; M < 10000.0; M += 7.0)
	{
		Done += Open.Update(M, M, 0.0) ? 1 : 0;
	}
	Check(Done == 1, "an open track's segment once", Done);
	// Starting inside a segment doesn't time a partial effort.
	FSegmentTimer Inside(Seg, 10000.0, true);
	Done = 0;
	for (double M = 2500.0; M < 9000.0; M += 5.0)
	{
		Done += Inside.Update(M, M, 0.0) ? 1 : 0;
	}
	Check(Done == 0 && !Inside.Running(), "no effort from a start inside the segment", Done);

	// History: a new time ranks against the old ones.
	const std::vector<FSegmentRecord> Past = {{"a", 120.0, 0}, {"b", 100.0, 0}, {"c", 110.0, 0}};
	const FRanking Pr = Rank(Past, 95.0);
	Check(Pr.Rank == 1 && Pr.Of == 4 && Pr.PreviousBest && *Pr.PreviousBest == 100.0, "a PR, 5 s under the old one",
	      Pr.Rank);
	Check(Rank(Past, 115.0).Rank == 3, "third of four", Rank(Past, 115.0).Rank);
	Check(Rank({}, 115.0).Rank == 1 && !Rank({}, 115.0).PreviousBest, "the first time is a PR");

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
