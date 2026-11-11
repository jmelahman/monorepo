// Rides FSession against FSimulatedTrainer: the bytes go both ways, as over the radio.
#include "BleSession.h"
#include "SimulatedTrainer.h"

#include <cstdio>

using namespace Veloverse::Ble;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got = 0.0)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

int main()
{
	FSimulatedTrainer Bike;
	FSession Session;
	double Now = 0.0;
	int Writes = 0;
	// One radio tick: the trainer's notifications in, the session's writes out and answered.
	auto Step = [&](double Grade, double Dt, bool Answer = true) {
		Now += Dt;
		Session.Notify(IndoorBikeData, Bike.IndoorBike(30.0), Now);
		if (auto Write = Session.Tick(Grade, Now))
		{
			++Writes;
			const Bytes Reply = Bike.Control(*Write);
			if (Answer) Session.Reply(Reply);
		}
	};

	Step(0.02, 0.25);
	Check(Writes == 0, "nothing is written before a trainer connects", Writes);
	Check(Session.Read(Now).Watts == 200.0 && Session.Read(Now).PowerSource == "trainer", "trainer power is read");

	Session.SetTrainer(true);
	for (int I = 0; I < 8; ++I) Step(0.02, 0.25);
	Check(Bike.bControlled && Bike.bStarted && Session.Controlling(), "control requested, then started");
	Check(std::abs(Bike.Grade - 0.02) < 1e-9, "the trainer has the grade", Bike.Grade * 100);

	// Small wobbles in grade don't flood the trainer; a real change gets through within a second.
	const int Before = Writes;
	for (int I = 0; I < 8; ++I) Step(0.02 + (I % 2) * 0.0004, 0.25);
	Check(Writes == Before, "a wobble under the step sends nothing", Writes - Before);
	for (int I = 0; I < 8; ++I) Step(0.065, 0.25);
	Check(std::abs(Bike.Grade - 0.065) < 1e-9, "a new grade reaches the trainer", Bike.Grade * 100);
	for (int I = 0; I < 40; ++I) Step(-0.03 + I * 0.002, 0.1);
	Check(Writes - Before <= 6, "steady climbing grades at most once a second", Writes - Before);

	// A lost reply doesn't stall the session for good.
	Step(0.10, 1.5, false);
	for (int I = 0; I < 20; ++I) Step(0.10, 0.25);
	Check(std::abs(Bike.Grade - 0.10) < 1e-9, "a lost reply is retried", Bike.Grade * 100);

	// A power meter outranks the trainer, a strap gives heart rate, and stale sources fall away.
	Bike.Watts = 210;
	FSimulatedTrainer Meter;
	Meter.Watts = 222;
	Meter.CadenceRpm = 88;
	for (int I = 0; I < 20; ++I)
	{
		Step(0.0, 0.25);
		Session.Notify(CyclingPowerMeasurement, Meter.CyclingPower(0.25), Now);
		Session.Notify(HeartRateMeasurement, Meter.HeartRate(141), Now);
	}
	FReading Reading = Session.Read(Now);
	Check(Reading.Watts == 222.0 && Reading.PowerSource == "power meter", "the power meter outranks the trainer",
	      Reading.Watts);
	Check(std::abs(Reading.CadenceRpm - 88.0) < 1.5, "cadence from the meter's crank", Reading.CadenceRpm);
	Check(Reading.HeartRateBpm == 141.0, "heart rate from the strap", Reading.HeartRateBpm);
	for (int I = 0; I < 16; ++I) Step(0.0, 0.25);
	Reading = Session.Read(Now);
	Check(Reading.Watts == 210.0 && Reading.PowerSource == "trainer" && Reading.HeartRateBpm == 0.0,
	      "a silent meter and strap give way", Reading.Watts);
	Reading = Session.Read(Now + 10.0);
	Check(Reading.Watts == 0.0 && Reading.PowerSource.empty(), "silence reads as no power", Reading.Watts);

	// A trainer that refuses control is left alone.
	FSession Locked;
	Locked.SetTrainer(true);
	auto Write = Locked.Tick(0.0, 0.0);
	Locked.Reply({0x80, 0x00, 0x05});
	Check(Write && Locked.Refused() && !Locked.Tick(0.05, 5.0), "a refusal stops the writes");

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
