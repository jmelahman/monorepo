// Checks RidePhysics.h without the engine: g++ -std=c++20 -I../OpenCycleUE/Source/OpenCycleUE/Public.
// Run with tests/run.sh.
#include "RidePhysics.h"

#include <cmath>
#include <cstdio>

using namespace OpenCycle;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

// Rides Watts on Grade for Seconds from a standstill, in the pawn's 1/120 s steps.
static double Ride(double Watts, double Grade, double Seconds, double Start = 0.0)
{
	const FRider Rider;
	double Speed = Start;
	for (double T = 0.0; T < Seconds; T += 1.0 / 120.0)
	{
		Speed = Step(Rider, Speed, Watts, Grade, 1.0 / 120.0);
	}
	return Speed;
}

int main()
{
	const FRider Rider;
	// 200 W on the flat holds a club rider's pace, about 32 km/h on the hoods.
	const double Flat = SteadySpeed(Rider, 200.0, 0.0) * 3.6;
	Check(Flat > 29.0 && Flat < 35.0, "200 W on the flat is 29..35 km/h", Flat);
	// A 6% climb at 250 W with 85 kg is about 15 km/h.
	const double Climb = SteadySpeed(Rider, 250.0, 0.06) * 3.6;
	Check(Climb > 13.0 && Climb < 17.0, "250 W up 6% is 13..17 km/h", Climb);
	// Riding long enough settles on the steady speed.
	const double Settled = Ride(200.0, 0.0, 120.0);
	Check(std::abs(Settled - SteadySpeed(Rider, 200.0, 0.0)) < 0.05, "riding settles on the steady speed", Settled);
	// More power is never slower.
	Check(SteadySpeed(Rider, 300.0, 0.02) > SteadySpeed(Rider, 200.0, 0.02), "more power, more speed",
	      SteadySpeed(Rider, 300.0, 0.02));
	// Freewheeling down 5% runs on to a terminal speed well above walking pace.
	const double Coast = Ride(0.0, -0.05, 120.0) * 3.6;
	Check(Coast > 40.0 && Coast < 70.0, "coasting down 5% is 40..70 km/h", Coast);
	// Stopping pedalling on a steep climb stops the rider; they never roll back.
	const double Stalled = Ride(0.0, 0.12, 30.0, 4.0);
	Check(Stalled == 0.0, "no power up 12% comes to a stop, not backwards", Stalled);
	// A standing start doesn't blow up.
	const double FirstStep = Step(Rider, 0.0, 400.0, 0.0, 1.0 / 120.0);
	Check(std::isfinite(FirstStep) && FirstStep > 0.0 && FirstStep < 0.1, "a standing start is gentle", FirstStep);
	// Resistance is all rolling at a standstill on the flat.
	Check(std::abs(Resistance(Rider, 0.0, 0.0) - Rider.MassKg * Gravity * Rider.Crr) < 1e-9,
	      "standing on the flat only rolls", Resistance(Rider, 0.0, 0.0));
	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
