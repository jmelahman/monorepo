// Sets MPC_Ride (the rain falling and the wet ground) from wherever the camera is on the track. A
// Movie Render Queue render runs its own game mode, so there is no ARidePawn to do it, and MRQ
// ignores a sequence's MPC track: make_coast_ride.py spawns one of these in the stills' sequence.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Actor.h"
#include "RideWeatherSync.generated.h"

class URideTrack;

UCLASS()
class VELOVERSEUE_API ARideWeatherSync : public AActor
{
	GENERATED_BODY()

public:
	ARideWeatherSync();
	virtual void Tick(float Dt) override;

protected:
	virtual void BeginPlay() override;

private:
	UPROPERTY(Transient) TObjectPtr<URideTrack> Track;
};
