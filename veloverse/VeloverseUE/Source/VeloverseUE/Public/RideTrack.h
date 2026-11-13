// The ride's road as data: its centreline every metre and its weather along the way, written by
// scripts/ride_track.py from coast_layout.py when the level is built. The game rides exactly the
// road the level was built round, with no layout code of its own.
#pragma once

#include "CoreMinimal.h"
#include "Engine/DataAsset.h"
#include "RideTrack.generated.h"

USTRUCT(BlueprintType)
struct FRideWeather
{
	GENERATED_BODY()

	UPROPERTY(EditAnywhere) float Metres = 0.f;
	UPROPERTY(EditAnywhere) float Lux = 90000.f;     // the sun's intensity
	UPROPERTY(EditAnywhere) float Elevation = 34.f;  // the sun's height, degrees
	UPROPERTY(EditAnywhere) float Sky = 1.f;         // the sky light's intensity
	UPROPERTY(EditAnywhere) float Fog = 0.012f;      // fog density
	UPROPERTY(EditAnywhere) float Rain = 0.f;        // 0 dry .. 1 full rain
	UPROPERTY(EditAnywhere) float Wet = 0.f;         // the ground: the rain, drying after it stops
};

// Where a stretch of the ride starts (town, paddies, jungle, ...): it runs to the next one's start.
USTRUCT(BlueprintType)
struct FRideStretch
{
	GENERATED_BODY()

	UPROPERTY(EditAnywhere) FString Name;
	UPROPERTY(EditAnywhere) float Metres = 0.f;
};

UCLASS(BlueprintType)
class VELOVERSEUE_API URideTrack : public UDataAsset
{
	GENERATED_BODY()

public:
	// What the start screen calls the route, and a line about it. Empty in older tracks.
	UPROPERTY(EditAnywhere) FString Title;
	UPROPERTY(EditAnywhere) FString Blurb;
	// The level's other routes, on its default track (DA_RideTrack) only: the start screen offers
	// this track and then these, and -RideRoute=TITLE picks one by its title.
	UPROPERTY(EditAnywhere) TArray<TSoftObjectPtr<URideTrack>> Routes;
	// Centreline points in world cm, Step apart along the road, heights included.
	UPROPERTY(EditAnywhere) TArray<FVector> Centre;
	UPROPERTY(EditAnywhere) float Step = 100.f;
	// A loop runs from the last point back into the first.
	UPROPERTY(EditAnywhere) bool bClosed = false;
	// Riders keep left: how far left of the centreline, in cm.
	UPROPERTY(EditAnywhere) float KeepLeft = 45.f;
	// The sun's compass bearing, which doesn't change along the ride.
	UPROPERTY(EditAnywhere) float SunAzimuth = -62.f;
	// Weather rows in order of Metres.
	UPROPERTY(EditAnywhere) TArray<FRideWeather> Weather;
	// Stretches in order of Metres; empty in a track written before they were.
	UPROPERTY(EditAnywhere) TArray<FRideStretch> Stretches;
	// The stretch rides start in unless the rider picks another (the first of that name); empty,
	// or a name no stretch has, starts at the track's own start.
	UPROPERTY(EditAnywhere) FString StartAt;
	// The rain shells carried round the camera; their material shows streaks only where it rains.
	UPROPERTY(EditAnywhere) TObjectPtr<class UStaticMesh> RainShell;
	// MPC_Ride, whose Rain and Wet the materials read: the shells' streaks and the wet ground.
	UPROPERTY(EditAnywhere) TObjectPtr<class UMaterialParameterCollection> Collection;

	float Length() const;
	float Wrap(float S) const;
	// The centreline at S (cm): position, and yaw in degrees along the road.
	void At(float S, FVector& OutPosition, float& OutYaw) const;
	// Lateral cm right of the centreline at S, raised by Up.
	FVector Point(float S, float Lateral, float Up = 0.f) const;
	float Height(float S) const;
	// Rise over run across Span cm centred on S.
	float Grade(float S, float Span = 800.f) const;
	FRideWeather WeatherAt(float Metres) const;
	// The distance along the road (cm) of the centreline point nearest Where, across the ground.
	float Nearest(const FVector& Where) const;

private:
	int32 Index(int32 I) const;
};
