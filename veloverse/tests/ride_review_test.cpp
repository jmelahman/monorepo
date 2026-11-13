// Checks RideReview.h without the engine. Run with tests/run.sh.
#include "RideReview.h"

#include <cmath>
#include <cstdio>
#include <string>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

static void Ride(std::vector<FSample>& Samples, int Seconds, double Watts, double Rpm = 90.0, double Bpm = 0.0)
{
	for (int I = 0; I < Seconds; ++I)
	{
		FSample S;
		S.Seconds = static_cast<double>(Samples.size() + 1);
		S.Metres = S.Seconds * 10.0;
		S.Watts = Watts;
		S.Cadence = Watts > 0.0 ? Rpm : 0.0;
		S.HeartRate = Bpm;
		S.Speed = 10.0;
		S.Elevation = S.Seconds * 0.1;
		Samples.push_back(S);
	}
}

int main()
{
	std::vector<FSample> Samples;
	Ride(Samples, 600, 200.0, 90.0, 140.0);
	Ride(Samples, 60, 400.0, 100.0, 175.0);
	Ride(Samples, 300, 0.0, 0.0, 120.0);

	const FCurve Curve = PowerCurve(Samples);
	Check(Curve[0] == 400.0, "the 1 s best is the hardest second", Curve[0]);
	Check(Curve[4] == 400.0, "the 1 min best is the 60 s surge", Curve[4]);
	Check(std::abs(Curve[6] - (60 * 400.0 + 240 * 200.0) / 300.0) < 1e-9, "5 min takes the surge and the 4 min before",
	      Curve[6]);
	Check(Curve[10] == 0.0, "an hour is past a 16 minute ride", Curve[10]);
	FCurve Other = {};
	Other[10] = 180.0;
	const FCurve Both = Best(Curve, Other);
	Check(Both[0] == 400.0 && Both[10] == 180.0, "the best of two curves takes each duration's better", Both[10]);

	Check(HeartZone(100.0, 190.0) == 0, "100 of 190 bpm is zone 1", HeartZone(100.0, 190.0));
	Check(HeartZone(140.0, 190.0) == 2, "140 of 190 bpm is zone 3", HeartZone(140.0, 190.0));
	Check(HeartZone(175.0, 190.0) == 4, "175 of 190 bpm is zone 5", HeartZone(175.0, 190.0));
	const auto Hz = HeartZoneSeconds(Samples, 190.0);
	Check(Hz[2] == 600.0 && Hz[4] == 60.0 && Hz[1] == 300.0, "time in heart zones", Hz[2]);
	std::vector<FSample> Strapless;
	Ride(Strapless, 60, 200.0);
	Check(HeartZoneSeconds(Strapless, 190.0)[0] == 0.0, "no strap, no heart zones", 0.0);

	const FCadenceBands Bands = CadenceHistogram(Samples);
	Check(Bands.Count() == 12, "60 to 120 rpm in 5s is 12 bands", Bands.Count());
	Check(Bands.Seconds[6] == 600.0, "90 rpm lands in the 90 band", Bands.Seconds[6]);
	Check(Bands.Seconds[8] == 60.0, "100 rpm lands in the 100 band", Bands.Seconds[8]);
	Check(Bands.Peak() == 6 && Bands.Low(Bands.Peak()) == 90.0, "the peak band is 90 to 95", Bands.Peak());
	std::vector<FSample> Spin;
	Ride(Spin, 10, 100.0, 140.0);
	Ride(Spin, 10, 100.0, 30.0);
	const FCadenceBands Edges = CadenceHistogram(Spin);
	Check(Edges.Seconds.front() == 10.0 && Edges.Seconds.back() == 10.0, "out of range goes to the end bands", 10.0);
	Check(CadenceHistogram({}).Peak() == -1, "nothing pedalled has no peak", -1);

	const FTraces T = Traces(Samples, 96);
	Check(T.Watts.size() == 96 && std::abs(T.Metres - 9590.0) < 1e-9, "96 columns over the ride's length", T.Metres);
	Check(T.Watts.front() == 200.0 && T.Watts.back() == 0.0, "the traces start at 200 W and end coasting", T.Watts[0]);
	Check(T.HeartRate.back() == 120.0, "and the heart's trace with them", T.HeartRate.back());
	std::vector<FSample> Sparse;
	Ride(Sparse, 3, 150.0);
	const FTraces Wide = Traces(Sparse, 10);
	bool Filled = true;
	for (const double W : Wide.Watts)
	{
		Filled = Filled && W == 150.0;
	}
	Check(Filled, "columns between samples take a neighbour's value", Wide.Watts[5]);

	std::vector<double> Daily(42, 100.0);
	const FLoad L = TrainingLoad(Daily);
	Check(std::abs(L.Fitness.back() - 100.0 * (1.0 - std::exp(-1.0))) < 1e-9, "42 days at 100 TSS is 63% fitness",
	      L.Fitness.back());
	Check(L.Fatigue.back() > 99.0, "and fatigue near 100", L.Fatigue.back());
	Check(std::string(FormWord(L.Form())) == "very tired", "six weeks hard from nothing is very tired", L.Form());
	std::vector<double> Block(120, 50.0);
	Block.insert(Block.end(), 14, 80.0);
	const double Built = TrainingLoad(Block).Form();
	Check(std::string(FormWord(Built)) == "training", "a harder fortnight on a base reads as training", Built);
	std::vector<double> Steady(200, 60.0);
	Check(std::string(FormWord(TrainingLoad(Steady).Form())) == "neutral", "a steady load is neutral",
	      TrainingLoad(Steady).Form());
	Daily.assign(14, 0.0);
	std::vector<double> Taper(42, 100.0);
	Taper.insert(Taper.end(), Daily.begin(), Daily.end());
	Check(TrainingLoad(Taper).Form() > 5.0, "two weeks off leaves the rider fresh", TrainingLoad(Taper).Form());
	Check(TrainingLoad({}).Form() == 0.0, "no rides, no form", 0.0);

	std::printf("%s\n", Failures ? "FAILED" : "all passed");
	return Failures ? 1 : 0;
}
