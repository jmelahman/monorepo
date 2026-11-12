// The radio link for FBleTrainer: Bluetooth LE through BlueZ, over D-Bus (-RideTrainer=ble).
// libdbus is loaded when the link starts, not linked, so the game runs where it's missing; then,
// or with no system bus or no adapter, the link says so in its name and never has a trainer.
//
// A worker thread does the talking: it scans, connects the first fitness machine (or the one
// picked) and the first heart-rate strap, subscribes to their measurements and the control
// point, and writes control point commands with a response. The game thread only swaps queues.
#pragma once

#include "CoreMinimal.h"
#include "BleTrainer.h"

class FBlueZLink : public IBleLink
{
public:
	FBlueZLink();
	virtual ~FBlueZLink() override;

	virtual FString Name() const override;
	virtual void Poll(Veloverse::Ble::FSession& Session, double Now) override;
	virtual void Write(const Veloverse::Ble::Bytes& ControlPoint) override;
	virtual bool HasTrainer() const override;
	virtual TArray<FString> Devices() const override;
	virtual FString Picked() const override;
	virtual void Pick(const FString& Device) override;

	struct FImpl; // the D-Bus side, in BlueZLink.cpp

private:
	TUniquePtr<FImpl> Impl;
};
