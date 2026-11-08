// The Bluetooth GATT messages a smart trainer, power meter, speed/cadence sensor and heart-rate strap
// send, and the FTMS commands that set a trainer's resistance. Plain C++ with no engine types, so it
// is tested on its own (tests/ble_protocol_test.cpp); the BLE transport sits on top.
//
// Little-endian throughout, as the Bluetooth specs have it:
//   Heart Rate Measurement   0x2A37 (service 0x180D)
//   Cycling Power Measurement 0x2A63 (service 0x1818)
//   CSC Measurement          0x2A5B (service 0x1816)
//   Indoor Bike Data         0x2AD2 (FTMS, service 0x1826)
//   Fitness Machine Control Point 0x2AD9
#pragma once

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <optional>
#include <vector>

namespace OpenCycle::Ble
{
constexpr uint16_t HeartRateService = 0x180D, HeartRateMeasurement = 0x2A37;
constexpr uint16_t CyclingPowerService = 0x1818, CyclingPowerMeasurement = 0x2A63;
constexpr uint16_t SpeedCadenceService = 0x1816, SpeedCadenceMeasurement = 0x2A5B;
constexpr uint16_t FitnessMachineService = 0x1826, IndoorBikeData = 0x2AD2, ControlPoint = 0x2AD9;

using Bytes = std::vector<uint8_t>;

// Reads little-endian fields off the front of a packet; a short packet makes it !Ok, never overrun.
class FReader
{
public:
	explicit FReader(const Bytes& In) : Data(In) {}
	bool Ok() const { return Good; }
	uint32_t Unsigned(int Size)
	{
		if (At + Size > Data.size())
		{
			Good = false;
			At = Data.size();
			return 0;
		}
		uint32_t Value = 0;
		for (int I = 0; I < Size; ++I)
		{
			Value |= uint32_t(Data[At + I]) << (8 * I);
		}
		At += Size;
		return Value;
	}
	int32_t Signed(int Size)
	{
		const uint32_t Value = Unsigned(Size);
		const uint32_t Sign = 1u << (8 * Size - 1);
		return Size < 4 && (Value & Sign) ? int32_t(Value) - int32_t(Sign << 1) : int32_t(Value);
	}
	void Skip(int Size) { Unsigned(Size); }

private:
	const Bytes& Data;
	size_t At = 0;
	bool Good = true;
};

inline void Put(Bytes& Out, int64_t Value, int Size)
{
	for (int I = 0; I < Size; ++I)
	{
		Out.push_back(uint8_t((uint64_t(Value) >> (8 * I)) & 0xFF));
	}
}

// Cadence (or wheel speed) from cumulative revolutions and the time of the last one, as power meters
// and CSC sensors send them. Both counters wrap; a packet with no new revolution keeps the last
// cadence until StaleSeconds pass without one, then reads zero.
class FRevolutions
{
public:
	explicit FRevolutions(double TicksPerSecond, double StaleSeconds = 3.0)
	    : Ticks(TicksPerSecond), Stale(StaleSeconds)
	{
	}
	// Feeds a reading taken at Now (seconds, any clock); returns revolutions per minute.
	double Add(uint32_t Revs, uint16_t EventTime, double Now, uint32_t RevsMask = 0xFFFF)
	{
		if (Seen)
		{
			const uint32_t NewRevs = (Revs - LastRevs) & RevsMask;
			const uint16_t Elapsed = uint16_t(EventTime - LastTime);
			if (NewRevs > 0 && Elapsed > 0)
			{
				Rpm = 60.0 * NewRevs * Ticks / Elapsed;
				LastNew = Now;
			}
			else if (Now - LastNew > Stale)
			{
				Rpm = 0.0;
			}
		}
		else
		{
			LastNew = Now;
		}
		Seen = true;
		LastRevs = Revs;
		LastTime = EventTime;
		return Rpm;
	}
	double Value() const { return Rpm; }

private:
	double Ticks, Stale;
	bool Seen = false;
	uint32_t LastRevs = 0;
	uint16_t LastTime = 0;
	double LastNew = 0.0;
	double Rpm = 0.0;
};

// What one packet says. Fields a device doesn't send stay empty.
struct FMeasurement
{
	std::optional<double> Watts;
	std::optional<double> CadenceRpm;
	std::optional<double> SpeedKmh;
	std::optional<double> HeartRateBpm;
	std::optional<uint32_t> CrankRevs; // cumulative, for FRevolutions
	std::optional<uint16_t> CrankTime; // 1/1024 s
	std::optional<uint32_t> WheelRevs;
	std::optional<uint16_t> WheelTime; // 1/1024 s for CSC, 1/2048 s for cycling power
};

inline std::optional<FMeasurement> ParseHeartRate(const Bytes& In)
{
	FReader R(In);
	const uint32_t Flags = R.Unsigned(1);
	FMeasurement M;
	M.HeartRateBpm = double(R.Unsigned(Flags & 0x01 ? 2 : 1));
	return R.Ok() ? std::optional(M) : std::nullopt;
}

inline std::optional<FMeasurement> ParseCyclingPower(const Bytes& In)
{
	FReader R(In);
	const uint32_t Flags = R.Unsigned(2);
	FMeasurement M;
	M.Watts = double(R.Signed(2));
	if (Flags & 0x0001) R.Skip(1); // pedal power balance
	if (Flags & 0x0004) R.Skip(2); // accumulated torque
	if (Flags & 0x0010)
	{
		M.WheelRevs = R.Unsigned(4);
		M.WheelTime = uint16_t(R.Unsigned(2));
	}
	if (Flags & 0x0020)
	{
		M.CrankRevs = R.Unsigned(2);
		M.CrankTime = uint16_t(R.Unsigned(2));
	}
	return R.Ok() ? std::optional(M) : std::nullopt;
}

inline std::optional<FMeasurement> ParseSpeedCadence(const Bytes& In)
{
	FReader R(In);
	const uint32_t Flags = R.Unsigned(1);
	FMeasurement M;
	if (Flags & 0x01)
	{
		M.WheelRevs = R.Unsigned(4);
		M.WheelTime = uint16_t(R.Unsigned(2));
	}
	if (Flags & 0x02)
	{
		M.CrankRevs = R.Unsigned(2);
		M.CrankTime = uint16_t(R.Unsigned(2));
	}
	return R.Ok() ? std::optional(M) : std::nullopt;
}

// FTMS Indoor Bike Data. Bit 0 set means "more data": the speed field is left out.
inline std::optional<FMeasurement> ParseIndoorBike(const Bytes& In)
{
	FReader R(In);
	const uint32_t Flags = R.Unsigned(2);
	FMeasurement M;
	if (!(Flags & 0x0001)) M.SpeedKmh = R.Unsigned(2) * 0.01;
	if (Flags & 0x0002) R.Skip(2); // average speed
	if (Flags & 0x0004) M.CadenceRpm = R.Unsigned(2) * 0.5;
	if (Flags & 0x0008) R.Skip(2); // average cadence
	if (Flags & 0x0010) R.Skip(3); // total distance
	if (Flags & 0x0020) R.Skip(2); // resistance level
	if (Flags & 0x0040) M.Watts = double(R.Signed(2));
	if (Flags & 0x0080) R.Skip(2); // average power
	if (Flags & 0x0100) R.Skip(5); // expended energy: total, per hour, per minute
	if (Flags & 0x0200) M.HeartRateBpm = double(R.Unsigned(1));
	return R.Ok() ? std::optional(M) : std::nullopt;
}

// FTMS control point commands. A trainer obeys only after RequestControl succeeds.
namespace Command
{
inline Bytes RequestControl() { return {0x00}; }
inline Bytes Reset() { return {0x01}; }
inline Bytes Start() { return {0x07}; }
inline Bytes TargetPower(double Watts)
{
	Bytes Out{0x05};
	Put(Out, std::lround(Watts), 2);
	return Out;
}
// Simulation mode: the trainer sets its own resistance for the grade, wind and rolling resistance.
// Grade is rise over run (0.05 for 5%); CwKgPerM is 0.5 * air density * CdA.
inline Bytes Simulation(double Grade, double WindMs = 0.0, double Crr = 0.004, double CwKgPerM = 0.51)
{
	Bytes Out{0x11};
	Put(Out, std::lround(WindMs * 1000.0), 2);
	Put(Out, std::lround(std::clamp(Grade, -0.4, 0.4) * 10000.0), 2);
	Put(Out, std::lround(std::clamp(Crr, 0.0, 0.0255) * 10000.0), 1);
	Put(Out, std::lround(std::clamp(CwKgPerM, 0.0, 2.55) * 100.0), 1);
	return Out;
}
} // namespace Command

// The trainer's reply on the control point: 0x80, the op code it answers, and a result (1 = success).
inline bool Succeeded(const Bytes& Reply, uint8_t OpCode)
{
	return Reply.size() >= 3 && Reply[0] == 0x80 && Reply[1] == OpCode && Reply[2] == 0x01;
}
} // namespace OpenCycle::Ble
