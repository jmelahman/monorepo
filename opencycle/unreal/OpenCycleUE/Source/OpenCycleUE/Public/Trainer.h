// Where the rider's effort comes from: the keyboard now, a smart trainer or power meter later.
// The ride asks a trainer for a reading every frame and tells it the grade under the wheels.
#pragma once

#include "CoreMinimal.h"

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
};

// Power from the keyboard: hold Up or W to push harder and Down or S to ease off. Cadence follows
// power, roughly.
class FKeyboardTrainer : public ITrainer
{
public:
	double Watts = 180.0;
	double Rate = 80.0; // watts per second while a key is held
	int Direction = 0;  // +1 harder, -1 easier, 0 steady

	virtual FString Name() const override { return TEXT("Keyboard"); }
	virtual void Tick(float Dt) override { Watts = FMath::Clamp(Watts + Direction * Rate * Dt, 0.0, 1200.0); }
	virtual FTrainerReading Read() const override
	{
		FTrainerReading Reading;
		Reading.Watts = Watts;
		Reading.CadenceRpm = Watts > 0.0 ? FMath::Clamp(70.0 + Watts / 20.0, 70.0, 105.0) : 0.0;
		return Reading;
	}
};
