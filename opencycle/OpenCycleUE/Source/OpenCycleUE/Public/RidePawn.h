// The rider: carried along the track at the speed their power gives, with the camera where the
// stills put it (keeping left, eye height, looking a few metres up the road), the weather following
// where they are, and the rain shells carried round them.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Pawn.h"
#include "RidePhysics.h"
#include "Trainer.h"
#include "RidePawn.generated.h"

class UCameraComponent;
class UStaticMeshComponent;
class URideTrack;
struct FPostProcessSettings;
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
	// cm along the track, not wrapped: laps add up. Double: 25 km is 2.5 million cm, where a float
	// resolves only a quarter of a centimetre against a physics substep's few.
	double Distance = 0.0;
	float Speed = 0.f;    // m/s
	float Grade = 0.f;    // rise over run
	float Elapsed = 0.f;  // seconds riding
	bool bPaused = false;
	bool bFinished = false;
	bool bStarted = false; // off the start screen
	FTrainerReading Reading;
	float ClimbedMetres = 0.f;
	double Joules = 0.0;
	// Power zones as shares of FTP (-RideFtp=, 250 W by default), Coggan's seven: Zone gives 0..6.
	float Ftp = 250.f;
	int32 Zone(float Watts) const;
	float ZoneSeconds[7] = {};
	FString TrainerName() const;
	float TrackLength() const;

	// Where power comes from: "keyboard", or "sim", a Bluetooth trainer played from the keys.
	static const TArray<FString> TrainerSources;
	FString TrainerSource;
	void UseTrainer(const FString& Source);

	// An unattended ride, for tests and stills: any of -RideStart, -RideWatts, -RideShots or
	// -RideNoMenu skips the start screen and the menus.
	static bool Automated();

private:
	TUniquePtr<ITrainer> Trainer;
	// The keys always drive these legs: as the rider's power, or as a simulated trainer's rider.
	FKeyboardTrainer Legs;
	OpenCycle::FRider Rider;
	float WeatherClock = 0.f;

	// Unattended rides for tests and stills, from the command line: -RideStart=METRES,
	// -RideWatts=W (held), -RideShots=SECONDS,SECONDS,... (screenshots with the HUD, into
	// -RideShotDir=), then quit. -RideTrainer=keyboard|sim picks where power comes from, and is
	// only the first choice on the start screen. A status line is logged every few seconds.
	TArray<float> ShotTimes;
	int32 NextShot = 0;
	FString ShotDir;
	float LogClock = 0.f;
	// Frames since the last status line: the slowest, and how many took over 50 ms (a hitch the eye
	// sees as the ride stepping, whatever the average).
	float SlowestFrame = 0.f;
	int32 Hitches = 0;
	int32 Frames = 0;
	// The head's rotation, eased toward the road's: the track is straight between samples a metre
	// apart, so following it rigidly turns the view in small kinks several times a second.
	FRotator Head;
	bool bHeadSet = false;
	// The body on the bike: the crank's angle (radians), which drives a pedalling sway, and the
	// roll into a bend, eased, both in degrees. The sway's size follows the effort.
	float Crank = 0.f;
	float Sway = 0.f;
	float Lean = 0.f;
	float QuitAt = -1.f;
	float ProfileAt = -1.f;
	void ReadCommandLine();
	void Automate(float Dt);

	TWeakObjectPtr<UDirectionalLightComponent> Sun;
	TWeakObjectPtr<USkyLightComponent> SkyLight;
	TWeakObjectPtr<UExponentialHeightFogComponent> Fog;
	TWeakObjectPtr<APostProcessVolume> Post;

	static void CameraLook(FPostProcessSettings& Look);
	void ReadKeys();
	void Ride(float Dt);
	void Place();
	void FindWeather();
	void ApplyWeather();
};
