// Where the rider's effort comes from: the keyboard now, a smart trainer or power meter later.
// The ride asks a trainer for a reading every frame and tells it the grade under the wheels.
#pragma once

#include "CoreMinimal.h"

// A Bluetooth device the rider can use: a smart trainer, a power meter or a heart-rate strap.
struct FSensor
{
	enum class EKind : uint8
	{
		Trainer,
		Power,
		Heart
	};
	FString Name;
	EKind Kind = EKind::Trainer;
	bool bInUse = false;     // the one the ride reads, of its kind
	bool bConnected = false; // and connected now
};

struct FTrainerReading
{
	double Watts = 0.0;
	double CadenceRpm = 0.0;
	double HeartRateBpm = 0.0; // 0 when there's no heart-rate strap
};

class ITrainer
{
public:
	virtual ~ITrainer() = default;
	virtual FString Name() const = 0;
	virtual void Tick(float Dt) {}
	virtual FTrainerReading Read() const = 0;
	// The grade under the wheels, rise over run, for a trainer that can resist like the road.
	virtual void SetGrade(double Grade) {}
	// ERG mode: a power to hold, as a workout asks, or nullopt to ride the road again.
	virtual void SetTargetPower(TOptional<double> Watts) { Target = Watts; }
	TOptional<double> TargetPower() const { return Target; }
	// A Bluetooth trainer's choice of devices: the names found, the one picked ("" for the first).
	virtual TArray<FString> Devices() const { return {}; }
	virtual FString Picked() const { return FString(); }
	virtual void Pick(const FString& Device) {}
	// Every device found, trainers and straps, and the strap picked ("" for the first).
	virtual TArray<FSensor> Sensors() const { return {}; }
	virtual void PickStrap(const FString& Device) {}

protected:
	TOptional<double> Target;
};

// Power from the keyboard: hold Up or W to push harder and Down or S to ease off. Cadence follows
// power, roughly. In ERG mode it eases to the target the way a smart trainer does, and the keys
// nudge it.
class FKeyboardTrainer : public ITrainer
{
public:
	double Watts = 180.0;
	double Rate = 80.0; // watts per second while a key is held
	int Direction = 0;  // +1 harder, -1 easier, 0 steady

	virtual FString Name() const override { return TEXT("Keyboard"); }
	virtual void Tick(float Dt) override
	{
		if (Target && Direction == 0)
		{
			Watts += (*Target - Watts) * (1.0 - FMath::Exp(-Dt / 1.5));
		}
		Watts = FMath::Clamp(Watts + Direction * Rate * Dt, 0.0, 1200.0);
	}
	virtual FTrainerReading Read() const override
	{
		FTrainerReading Reading;
		Reading.Watts = Watts;
		Reading.CadenceRpm = Watts > 0.0 ? FMath::Clamp(70.0 + Watts / 20.0, 70.0, 105.0) : 0.0;
		return Reading;
	}
};
