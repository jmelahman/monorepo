// The rider: carried along the track at the speed their power gives, with the camera where the
// stills put it (keeping left, eye height, looking a few metres up the road, swaying with the
// pedals), the weather following where they are, and the rain shells carried round them.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Pawn.h"
#include "RidePhysics.h"
#include "Trainer.h"
#include "RidePawn.generated.h"

class UCameraComponent;
class UStaticMeshComponent;
class URideTrack;
class UDirectionalLightComponent;
class USkyLightComponent;
class UExponentialHeightFogComponent;
class APostProcessVolume;

UCLASS()
class OPENCYCLEUE_API ARidePawn : public APawn
{
	GENERATED_BODY()

public:
	ARidePawn();

	virtual void BeginPlay() override;
	virtual void Tick(float Dt) override;

	UPROPERTY(VisibleAnywhere) TObjectPtr<UCameraComponent> Camera;
	UPROPERTY(VisibleAnywhere) TObjectPtr<UStaticMeshComponent> Rain;
	UPROPERTY(EditAnywhere) TSoftObjectPtr<URideTrack> TrackAsset;
	UPROPERTY(Transient) TObjectPtr<URideTrack> Track;

	UPROPERTY(EditAnywhere) float EyeHeight = 150.f; // cm above the road
	UPROPERTY(EditAnywhere) float StartMetres = 0.f;

	// What the HUD shows.
	float Distance = 0.f; // cm along the track, not wrapped: laps add up
	float Speed = 0.f;    // m/s
	float Grade = 0.f;    // rise over run
	float Elapsed = 0.f;  // seconds riding
	bool bPaused = false;
	bool bFinished = false;
	FTrainerReading Reading;
	FString TrainerName() const;
	float TrackLength() const;

private:
	TUniquePtr<ITrainer> Trainer;
	// The keys always drive these legs: as the rider's power, or as a simulated trainer's rider.
	FKeyboardTrainer Legs;
	OpenCycle::FRider Rider;
	float WeatherClock = 0.f;

	// Unattended rides for tests and stills, from the command line: -RideStart=METRES,
	// -RideWatts=W (held), -RideShots=SECONDS,SECONDS,... (screenshots with the HUD, into
	// -RideShotDir=), then quit. -RideTrainer=keyboard|sim picks where power comes from. A status line is logged every few seconds either way.
	TArray<float> ShotTimes;
	int32 NextShot = 0;
	FString ShotDir;
	float LogClock = 0.f;
	float QuitAt = -1.f;
	void ReadCommandLine();
	void Automate(float Dt);

	TWeakObjectPtr<UDirectionalLightComponent> Sun;
	TWeakObjectPtr<USkyLightComponent> SkyLight;
	TWeakObjectPtr<UExponentialHeightFogComponent> Fog;
	TWeakObjectPtr<APostProcessVolume> Post;

	void ReadKeys();
	void Ride(float Dt);
	void Place();
	void FindWeather();
	void ApplyWeather();
};
