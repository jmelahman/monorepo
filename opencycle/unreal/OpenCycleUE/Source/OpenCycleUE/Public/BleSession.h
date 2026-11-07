// Turns the notifications of a rider's Bluetooth devices into one reading, and keeps a smart
// trainer's resistance in step with the road. It knows nothing of the radio: the transport hands it
// each notification and control point reply, and writes whatever Tick returns to the control point.
// Plain C++, tested against FSimulatedTrainer in unreal/tests/ble_session_test.cpp.
#pragma once

#include "BleProtocol.h"

#include <string>

namespace OpenCycle::Ble
{
struct FReading
{
	double Watts = 0.0;
	double CadenceRpm = 0.0;
	double SpeedKmh = 0.0;     // as the trainer reckons it; the ride uses its own physics
	double HeartRateBpm = 0.0; // 0 without a heart-rate source
	std::string PowerSource;   // "power meter", "trainer" or "" when nothing has spoken lately
};

class FSession
{
public:
	// Readings older than this read as zero: a rider who stops pedalling stops getting power.
	double StaleSeconds = 3.0;
	// The trainer gets a new grade when it moves this much, but no more often than MinInterval.
	double GradeStep = 0.001;
	double MinInterval = 1.0;
	double Resend = 5.0; // and again this often, in case a write was lost
	double ReplyTimeout = 2.0; // a write with no reply by then is given up on

	// A notification from any device, by characteristic UUID, received at Now (seconds).
	void Notify(uint16_t Characteristic, const Bytes& Value, double Now)
	{
		switch (Characteristic)
		{
		case IndoorBikeData:
			if (auto M = ParseIndoorBike(Value))
			{
				if (M->Watts) Trainer = {*M->Watts, Now};
				if (M->CadenceRpm) TrainerCadence = {*M->CadenceRpm, Now};
				if (M->SpeedKmh) Speed = {*M->SpeedKmh, Now};
				if (M->HeartRateBpm && *M->HeartRateBpm > 0) TrainerHeart = {*M->HeartRateBpm, Now};
			}
			break;
		case CyclingPowerMeasurement:
			if (auto M = ParseCyclingPower(Value))
			{
				Meter = {*M->Watts, Now};
				if (M->CrankRevs) MeterCadence = {MeterCrank.Add(*M->CrankRevs, *M->CrankTime, Now), Now};
			}
			break;
		case SpeedCadenceMeasurement:
			if (auto M = ParseSpeedCadence(Value); M && M->CrankRevs)
			{
				SensorCadence = {SensorCrank.Add(*M->CrankRevs, *M->CrankTime, Now), Now};
			}
			break;
		case HeartRateMeasurement:
			if (auto M = ParseHeartRate(Value)) Heart = {*M->HeartRateBpm, Now};
			break;
		default:
			break;
		}
	}

	// A reply on the trainer's control point.
	void Reply(const Bytes& Value)
	{
		if (Succeeded(Value, 0x00)) State = EState::Controlled;
		else if (Value.size() >= 2 && Value[0] == 0x80 && Value[1] == 0x00) State = EState::Refused;
		if (Succeeded(Value, 0x07) && State == EState::Controlled) State = EState::Started;
		Waiting = false;
	}

	// Call every frame with the grade under the wheels; returns a control point write, if one is due.
	std::optional<Bytes> Tick(double Grade, double Now)
	{
		if (Waiting && Now - SentAt > ReplyTimeout) Waiting = false;
		if (!HasTrainer || Waiting || State == EState::Refused) return std::nullopt;
		if (State == EState::Idle) return Send(Command::RequestControl(), Now);
		if (State == EState::Controlled) return Send(Command::Start(), Now);
		const bool Moved = std::abs(Grade - SentGrade) >= GradeStep && Now - SentAt >= MinInterval;
		if (Moved || Now - SentAt >= Resend)
		{
			SentGrade = Grade;
			return Send(Command::Simulation(Grade), Now);
		}
		return std::nullopt;
	}

	// Whether a controllable trainer is connected; until then Tick writes nothing.
	void SetTrainer(bool Connected)
	{
		HasTrainer = Connected;
		State = EState::Idle;
		Waiting = false;
	}

	FReading Read(double Now) const
	{
		FReading R;
		if (Fresh(Meter, Now))
		{
			R.Watts = Meter.Value;
			R.PowerSource = "power meter";
		}
		else if (Fresh(Trainer, Now))
		{
			R.Watts = Trainer.Value;
			R.PowerSource = "trainer";
		}
		for (const FSample* Cadence : {&MeterCadence, &SensorCadence, &TrainerCadence})
		{
			if (Fresh(*Cadence, Now))
			{
				R.CadenceRpm = Cadence->Value;
				break;
			}
		}
		R.SpeedKmh = Fresh(Speed, Now) ? Speed.Value : 0.0;
		R.HeartRateBpm = Fresh(Heart, Now) ? Heart.Value : Fresh(TrainerHeart, Now) ? TrainerHeart.Value : 0.0;
		return R;
	}

	bool Controlling() const { return State == EState::Started; }
	bool Refused() const { return State == EState::Refused; }
	double LastGrade() const { return SentGrade; }

private:
	struct FSample
	{
		double Value = 0.0;
		double At = -1e9;
	};
	enum class EState { Idle, Refused, Controlled, Started };

	bool Fresh(const FSample& S, double Now) const { return Now - S.At <= StaleSeconds; }
	std::optional<Bytes> Send(Bytes Command, double Now)
	{
		Waiting = true;
		SentAt = Now;
		return Command;
	}

	FSample Trainer, Meter, TrainerCadence, MeterCadence, SensorCadence, Speed, Heart, TrainerHeart;
	FRevolutions MeterCrank{1024.0}, SensorCrank{1024.0};
	bool HasTrainer = false;
	bool Waiting = false;
	EState State = EState::Idle;
	double SentGrade = 1e9;
	double SentAt = -1e9;
};
} // namespace OpenCycle::Ble
