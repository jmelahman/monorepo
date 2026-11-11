// Checks BleProtocol.h and SimulatedTrainer.h without the engine. Run with tests/run.sh.
#include "BleProtocol.h"
#include "SimulatedTrainer.h"

#include <cmath>
#include <cstdio>

using namespace Veloverse::Ble;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got = 0.0)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

static bool Near(std::optional<double> Got, double Want, double Tolerance = 1e-6)
{
	return Got && std::abs(*Got - Want) <= Tolerance;
}

int main()
{
	// Heart rate in one byte and in two.
	Check(Near(ParseHeartRate({0x00, 0x8C})->HeartRateBpm, 140), "heart rate, 8 bit", 140);
	Check(Near(ParseHeartRate({0x01, 0x2C, 0x01})->HeartRateBpm, 300), "heart rate, 16 bit", 300);
	Check(!ParseHeartRate({0x01, 0x2C}), "a short heart rate packet is refused");

	// Cycling power: signed watts, then skipped balance and torque before the crank data.
	const auto Power = ParseCyclingPower({0x25, 0x00, 0xFA, 0x00, 0x32, 0x10, 0x00, 0x07, 0x00, 0x00, 0x04});
	Check(Power && Near(Power->Watts, 250), "cycling power watts past balance and torque", Power ? *Power->Watts : -1);
	Check(Power && Power->CrankRevs == 7u && Power->CrankTime == 1024, "cycling power crank data");
	Check(Near(ParseCyclingPower({0x00, 0x00, 0xF6, 0xFF})->Watts, -10), "negative power reads as negative");

	// CSC with wheel and crank data.
	const auto Csc = ParseSpeedCadence({0x03, 0x10, 0, 0, 0, 0x00, 0x08, 0x20, 0x00, 0x00, 0x0C});
	Check(Csc && Csc->WheelRevs == 16u && Csc->WheelTime == 2048 && Csc->CrankRevs == 32u, "speed and cadence fields");

	// Indoor Bike Data: speed in 0.01 km/h, cadence in half rpm, power; and the "more data" bit.
	const auto Bike = ParseIndoorBike({0x44, 0x00, 0xC4, 0x09, 0xAA, 0x00, 0xC8, 0x00});
	Check(Bike && Near(Bike->SpeedKmh, 25.0) && Near(Bike->CadenceRpm, 85.0) && Near(Bike->Watts, 200.0),
	      "indoor bike speed, cadence, power");
	const auto More = ParseIndoorBike({0x41, 0x02, 0x2C, 0x01, 0x96});
	Check(More && !More->SpeedKmh && Near(More->Watts, 300) && Near(More->HeartRateBpm, 150),
	      "more-data packet has no speed, and heart rate after power");
	Check(!ParseIndoorBike({0x44, 0x00, 0xC4}), "a short indoor bike packet is refused");

	// Cadence from crank revolutions, across the 16-bit wrap of both counters.
	FRevolutions Cadence(1024.0);
	Cadence.Add(65534, 65000, 0.0);
	const double Rpm = Cadence.Add(1, uint16_t(65000 + 2048), 2.0); // 3 revs in 2 s
	Check(std::abs(Rpm - 90.0) < 1e-9, "cadence across counter wrap", Rpm);
	Cadence.Add(1, uint16_t(65000 + 2048), 3.0);
	Check(Cadence.Value() == 90.0, "a repeat packet keeps the cadence", Cadence.Value());
	Cadence.Add(1, uint16_t(65000 + 2048), 6.0);
	Check(Cadence.Value() == 0.0, "no revolutions for 3 s reads zero", Cadence.Value());

	// Commands, byte for byte.
	Check(Command::Simulation(0.05) == Bytes{0x11, 0, 0, 0xF4, 0x01, 0x28, 0x33}, "simulation command for 5%");
	Check(Command::Simulation(-0.025)[3] == 0x06 && Command::Simulation(-0.025)[4] == 0xFF, "negative grade");
	Check(Command::TargetPower(250) == Bytes{0x05, 0xFA, 0x00}, "target power command");

	// The simulated trainer refuses until it is controlled, then keeps the grade.
	FSimulatedTrainer Trainer;
	Check(!Succeeded(Trainer.Control(Command::Simulation(0.03)), 0x11), "no control, no grade");
	Check(Succeeded(Trainer.Control(Command::RequestControl()), 0x00), "control granted");
	Check(Succeeded(Trainer.Control(Command::Simulation(0.034)), 0x11) && std::abs(Trainer.Grade - 0.034) < 1e-9,
	      "grade set in simulation mode", Trainer.Grade);

	// Its packets read back as what it was told to send, and its crank gives its cadence.
	Trainer.Watts = 231;
	Trainer.CadenceRpm = 92;
	const auto Echo = ParseIndoorBike(Trainer.IndoorBike(31.4));
	Check(Echo && Near(Echo->Watts, 231) && Near(Echo->CadenceRpm, 92) && Near(Echo->SpeedKmh, 31.4, 0.006),
	      "simulated indoor bike data round trip");
	FRevolutions Crank(1024.0);
	double Now = 0.0;
	for (int I = 0; I < 200; ++I)
	{
		Now += 0.25;
		const auto Packet = ParseCyclingPower(Trainer.CyclingPower(0.25));
		Crank.Add(*Packet->CrankRevs, *Packet->CrankTime, Now);
	}
	Check(std::abs(Crank.Value() - 92.0) < 1.5, "simulated crank gives its cadence", Crank.Value());

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
