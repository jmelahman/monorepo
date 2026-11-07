// A trainer that answers like a real FTMS smart trainer with a power meter, byte for byte, for tests
// and for riding without hardware: it takes control point commands, keeps the grade it was sent,
// and sends Indoor Bike Data and Cycling Power packets for whatever power its rider puts out.
#pragma once

#include "BleProtocol.h"

namespace OpenCycle::Ble
{
class FSimulatedTrainer
{
public:
	double Watts = 200.0;      // what the rider puts out
	double CadenceRpm = 85.0;
	double Grade = 0.0;        // last grade set in simulation mode
	std::optional<double> TargetWatts;
	bool bControlled = false;
	bool bStarted = false;

	// Answers a control point write the way FTMS does: 0x80, the op code, then a result code.
	Bytes Control(const Bytes& Command)
	{
		if (Command.empty())
		{
			return {0x80, 0x00, 0x03}; // invalid parameter
		}
		const uint8_t Op = Command[0];
		if (Op != 0x00 && !bControlled)
		{
			return {0x80, Op, 0x05}; // control not permitted
		}
		FReader R(Command);
		R.Skip(1);
		switch (Op)
		{
		case 0x00:
			bControlled = true;
			break;
		case 0x01:
			Grade = 0.0;
			TargetWatts.reset();
			bControlled = bStarted = false;
			break;
		case 0x05:
			TargetWatts = double(R.Signed(2));
			break;
		case 0x07:
			bStarted = true;
			break;
		case 0x11:
			R.Skip(2); // wind
			Grade = R.Signed(2) / 10000.0;
			TargetWatts.reset();
			break;
		default:
			return {0x80, Op, 0x02}; // not supported
		}
		return {0x80, Op, uint8_t(R.Ok() ? 0x01 : 0x03)};
	}

	// Indoor Bike Data with speed, cadence and power.
	Bytes IndoorBike(double SpeedKmh) const
	{
		Bytes Out;
		Put(Out, 0x0044, 2);
		Put(Out, std::lround(SpeedKmh * 100.0), 2);
		Put(Out, std::lround(CadenceRpm * 2.0), 2);
		Put(Out, std::lround(Watts), 2);
		return Out;
	}

	// A Cycling Power Measurement with crank revolutions, Dt seconds after the last one.
	Bytes CyclingPower(double Dt)
	{
		Crank += CadenceRpm / 60.0 * Dt;
		Clock += Dt;
		// The event time is when the last whole revolution happened.
		const double LastRev = Clock - (Crank - std::floor(Crank)) / std::max(CadenceRpm / 60.0, 1e-6);
		Bytes Out;
		Put(Out, 0x0020, 2);
		Put(Out, std::lround(Watts), 2);
		Put(Out, int64_t(std::floor(Crank)) & 0xFFFF, 2);
		Put(Out, int64_t(std::llround(LastRev * 1024.0)) & 0xFFFF, 2);
		return Out;
	}

	Bytes HeartRate(int Bpm) const { return {0x00, uint8_t(Bpm)}; }

private:
	double Crank = 0.0;
	double Clock = 0.0;
};
} // namespace OpenCycle::Ble
