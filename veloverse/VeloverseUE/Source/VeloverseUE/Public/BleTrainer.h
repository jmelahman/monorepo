// A Bluetooth rider as an ITrainer: FSession merges their devices and steers the trainer, over a
// link that carries the bytes. FSimulatedLink is a trainer in software, driven by the keyboard, so
// the whole path runs without hardware (-RideTrainer=sim); the radio link is a BlueZ one.
#pragma once

#include "BleSession.h"
#include "SimulatedTrainer.h"
#include "Trainer.h"

// What carries bytes between FSession and the devices.
class IBleLink
{
public:
	virtual ~IBleLink() = default;
	virtual FString Name() const = 0;
	// Hands the session every notification and control point reply since the last poll.
	virtual void Poll(Veloverse::Ble::FSession& Session, double Now) = 0;
	virtual void Write(const Veloverse::Ble::Bytes& ControlPoint) = 0;
	virtual bool HasTrainer() const = 0;
	// The trainers found, by name, for the rider to pick from; and the one picked ("" for any).
	virtual TArray<FString> Devices() const { return {}; }
	virtual FString Picked() const { return FString(); }
	virtual void Pick(const FString& Device) {}
};

class FBleTrainer : public ITrainer
{
public:
	explicit FBleTrainer(TUniquePtr<IBleLink> InLink) : Link(MoveTemp(InLink)) {}

	virtual FString Name() const override
	{
		const Veloverse::Ble::FReading R = Session.Read(Clock);
		FString Text = Link->Name();
		if (!R.PowerSource.empty())
		{
			Text += FString::Printf(TEXT(", power from %hs"), R.PowerSource.c_str());
		}
		if (Session.Controlling() && Session.TargetPower())
		{
			Text += FString::Printf(TEXT(", ERG %.0f W"), *Session.TargetPower());
		}
		else if (Session.Controlling())
		{
			Text += FString::Printf(TEXT(", grade %+.1f%%"), Session.LastGrade() * 100.0);
		}
		else if (Session.Refused())
		{
			Text += TEXT(", trainer refused control");
		}
		return Text;
	}

	virtual void Tick(float Dt) override
	{
		Clock += Dt;
		if (Link->HasTrainer() != bTrainer)
		{
			bTrainer = Link->HasTrainer();
			Session.SetTrainer(bTrainer);
		}
		Link->Poll(Session, Clock);
		if (auto Write = Session.Tick(Grade, Clock))
		{
			Link->Write(*Write);
		}
	}

	virtual FTrainerReading Read() const override
	{
		const Veloverse::Ble::FReading R = Session.Read(Clock);
		FTrainerReading Reading;
		Reading.Watts = FMath::Max(R.Watts, 0.0);
		Reading.CadenceRpm = R.CadenceRpm;
		Reading.HeartRateBpm = R.HeartRateBpm;
		return Reading;
	}

	virtual TArray<FString> Devices() const override { return Link->Devices(); }
	virtual FString Picked() const override { return Link->Picked(); }
	virtual void Pick(const FString& Device) override { Link->Pick(Device); }

	virtual void SetGrade(double InGrade) override { Grade = InGrade; }
	virtual void SetTargetPower(TOptional<double> Watts) override
	{
		ITrainer::SetTargetPower(Watts);
		Session.SetTargetPower(Watts ? std::optional(*Watts) : std::nullopt);
	}

private:
	TUniquePtr<IBleLink> Link;
	Veloverse::Ble::FSession Session;
	double Clock = 0.0;
	double Grade = 0.0;
	bool bTrainer = false;
};

// A smart trainer and heart-rate strap in software. Its rider's effort comes from Legs (the
// keyboard), and it notifies four times a second, as real trainers do.
class FSimulatedLink : public IBleLink
{
public:
	explicit FSimulatedLink(const FKeyboardTrainer* InLegs) : Legs(InLegs) {}

	virtual FString Name() const override { return TEXT("Simulated trainer"); }
	virtual bool HasTrainer() const override { return true; }

	virtual void Poll(Veloverse::Ble::FSession& Session, double Now) override
	{
		for (const Veloverse::Ble::Bytes& Reply : Replies)
		{
			Session.Reply(Reply);
		}
		Replies.Reset();
		if (Now - Sent < 0.25)
		{
			return;
		}
		Sent = Now;
		const FTrainerReading Effort = Legs->Read();
		if (Bike.TargetWatts && Bike.bStarted && Effort.CadenceRpm > 0.0)
		{
			// ERG: the brake brings the rider to the target over a couple of seconds.
			Bike.Watts += (*Bike.TargetWatts - Bike.Watts) * 0.15;
		}
		else
		{
			Bike.Watts = Effort.Watts;
		}
		Bike.CadenceRpm = Effort.CadenceRpm;
		Session.Notify(Veloverse::Ble::IndoorBikeData, Bike.IndoorBike(0.0), Now);
		// The heart follows the effort, slowly.
		Heart += (90.0 + Effort.Watts * 0.3 - Heart) * 0.01;
		Session.Notify(Veloverse::Ble::HeartRateMeasurement, Bike.HeartRate(FMath::RoundToInt(Heart)), Now);
	}

	virtual void Write(const Veloverse::Ble::Bytes& ControlPoint) override { Replies.Add(Bike.Control(ControlPoint)); }

private:
	const FKeyboardTrainer* Legs;
	Veloverse::Ble::FSimulatedTrainer Bike;
	TArray<Veloverse::Ble::Bytes> Replies;
	double Sent = -1.0;
	double Heart = 90.0;
};
