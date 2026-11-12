// Checks RideStats.h without the engine. Run with tests/run.sh.
#include "RideStats.h"

#include <cmath>
#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

// Seconds of riding at Watts each, appended to Samples at 10 m/s on the flat.
static void Ride(std::vector<FSample>& Samples, int Seconds, double Watts, double Climb = 0.0)
{
	for (int I = 0; I < Seconds; ++I)
	{
		FSample S;
		S.Seconds = static_cast<double>(Samples.size() + 1);
		S.Metres = S.Seconds * 10.0;
		S.Watts = Watts;
		S.Cadence = Watts > 0.0 ? 90.0 : 0.0;
		S.Elevation = Samples.empty() ? 0.0 : Samples.back().Elevation + Climb;
		Samples.push_back(S);
	}
}

static bool Near(double A, double B, double Tol = 1e-6) { return std::abs(A - B) <= Tol; }

int main()
{
	// An hour at FTP is 100 TSS, intensity 1, NP equal to the power.
	std::vector<FSample> Hour;
	Ride(Hour, 3600, 250.0);
	const FSummary H = Summarize(Hour, 250.0);
	Check(Near(H.Tss, 100.0, 1e-6), "an hour at FTP is 100 TSS", H.Tss);
	Check(Near(H.Intensity, 1.0), "an hour at FTP is intensity 1", H.Intensity);
	Check(Near(H.NormalizedWatts, 250.0), "steady power normalizes to itself", H.NormalizedWatts);
	Check(Near(H.Kilojoules, 900.0), "250 W for an hour is 900 kJ", H.Kilojoules);
	Check(Near(H.ZoneSeconds[3], 3600.0), "FTP is zone 4, threshold", H.ZoneSeconds[3]);
	Check(Near(H.Best[4], 250.0), "the best hour of an hour at 250 W", H.Best[4]);
	Check(Near(H.AverageSpeed, 10.0), "average speed is distance over time", H.AverageSpeed);

	// Intervals: NP sits above the average, and the best minute finds the hard minute.
	std::vector<FSample> Mixed;
	for (int Rep = 0; Rep < 10; ++Rep)
	{
		Ride(Mixed, 60, 400.0);
		Ride(Mixed, 120, 100.0);
	}
	const FSummary M = Summarize(Mixed, 250.0);
	Check(Near(M.AverageWatts, 200.0), "1 on, 2 off averages 200 W", M.AverageWatts);
	Check(M.NormalizedWatts > M.AverageWatts + 30.0, "intervals normalize above their average", M.NormalizedWatts);
	Check(Near(M.Best[1], 400.0), "the best minute is a 400 W minute", M.Best[1]);
	Check(Near(M.Best[0], 400.0), "the best 5 s is 400 W", M.Best[0]);
	Check(M.Best[4] == 0.0, "no best hour in a 30-minute ride", M.Best[4]);
	Check(Near(M.MaxWatts, 400.0), "max power", M.MaxWatts);
	Check(Near(M.ZoneSeconds[0], 1200.0) && Near(M.ZoneSeconds[6], 600.0), "zones: 100 W recovery, 400 W sprint",
	      M.ZoneSeconds[6]);

	// Coasting doesn't drag the average cadence down; climbing adds up.
	std::vector<FSample> Hill;
	Ride(Hill, 100, 200.0, 0.5);
	Ride(Hill, 100, 0.0, -0.5);
	const FSummary C = Summarize(Hill, 250.0);
	Check(Near(C.AverageCadence, 90.0), "cadence averages only while pedalling", C.AverageCadence);
	Check(Near(C.Climbed, 49.5), "99 half-metre rises climb 49.5 m", C.Climbed);
	Check(C.AverageHeartRate == 0.0, "no strap, no heart rate", C.AverageHeartRate);

	// A short ride normalizes to its average, and an empty one to nothing.
	std::vector<FSample> Short;
	Ride(Short, 10, 300.0);
	Check(Near(Summarize(Short, 250.0).NormalizedWatts, 300.0), "under 30 s, NP is the average", 300.0);
	Check(Summarize({}, 250.0).Seconds == 0.0, "an empty ride is all zeros", 0.0);
	Check(PowerZone(137.4, 250.0) == 0 && PowerZone(137.5, 250.0) == 1, "zone 2 starts at 55% of FTP", 137.5);

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
