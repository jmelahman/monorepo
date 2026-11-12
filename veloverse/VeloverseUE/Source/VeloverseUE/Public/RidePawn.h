// The rider: carried along the track at the speed their power gives, with the camera where the
// stills put it (keeping left, eye height, looking a few metres up the road), the weather following
// where they are, and the rain shells carried round them.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Pawn.h"
#include "RidePhysics.h"
#include "RideRecord.h"
#include "Trainer.h"
#include "RideSegments.h"
#include "Workout.h"
#include "RidePawn.generated.h"

class UCameraComponent;
class UStaticMeshComponent;
class URideTrack;
struct FPostProcessSettings;
class UDirectionalLightComponent;
class USkyLightComponent;
class UExponentialHeightFogComponent;
class APostProcessVolume;
class APlayerController;

UCLASS()
class VELOVERSEUE_API ARidePawn : public APawn
{
	GENERATED_BODY()

public:
	ARidePawn();

	virtual void BeginPlay() override;
	virtual void Tick(float Dt) override;
	virtual void EndPlay(const EEndPlayReason::Type Reason) override;

	UPROPERTY(VisibleAnywhere) TObjectPtr<UCameraComponent> Camera;
	UPROPERTY(VisibleAnywhere) TObjectPtr<UStaticMeshComponent> Rain;
	UPROPERTY(EditAnywhere) TSoftObjectPtr<URideTrack> TrackAsset;
	UPROPERTY(Transient) TObjectPtr<URideTrack> Track;
	// The level's routes, its default track first; Track is one of them.
	UPROPERTY(Transient) TArray<TObjectPtr<URideTrack>> Routes;
	// Rides Routes[Index] from its start. Only before the ride starts.
	void UseRoute(int32 Index);
	int32 RouteIndex() const { return Routes.IndexOfByKey(Track); }

	UPROPERTY(EditAnywhere) float EyeHeight = 150.f; // cm above the road
	UPROPERTY(EditAnywhere) float StartMetres = 0.f;

	// Where a ride can start, for the start screen: the track's start, then each stretch's. The
	// first is where it starts unless the rider picks another: the track's StartAt stretch, or its
	// own start. -RideStart=METRES and -RideFrom=STRETCH choose too (the first of that name).
	struct FStart
	{
		FString Name;
		float Metres = 0.f;
	};
	TArray<FStart> Starts() const;
	int32 StartIndex() const;
	// Moves the rider to Starts()[Index]. Only before the ride starts.
	void UseStart(int32 Index);
	// cm ridden since the start, wherever it was.
	double Ridden() const { return Distance - StartMetres * 100.0; }

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

	// The rider's FTP and weight, kept in Saved/Veloverse/profile.json. -RideFtp= overrides the FTP
	// for one ride. SetProfile saves it and rides with it from now.
	FRideProfile Profile;
	void SetProfile(const FRideProfile& Changed);
	// The ride so far, a sample a second, saved when it ends: at the finish, on "End ride", or when
	// the level goes (back to the start, quit). Unattended rides aren't kept unless -RideRecord.
	FRideRecorder Recorder;
	// Ends the ride and saves it, once; returns the file, or "" when it wasn't kept.
	FString EndRide();
	FString SavedRide;
	bool bRideSaved = false;
	float TrackLength() const;

	// The route's climbs, timed against the rider's best; saved with the ride.
	FRideSegments Segments;

	// A structured workout, if one is on: each step's target goes to the trainer in ERG mode (or, with
	// ERG off, is only shown). StartWorkout replaces any running one; Bias is the rider's own
	// adjustment, a share of the target ([ and ] in 5% steps); N skips a step, E toggles ERG.
	// -RideWorkout=NAME starts a built-in one by name, or a .zwo/.erg/.mrc file.
	Veloverse::FWorkoutPlayer Workout;
	float WorkoutBias = 0.f;
	bool bErg = true;
	void StartWorkout(const Veloverse::FWorkout& Chosen);
	void EndWorkout();
	TOptional<double> WorkoutTarget() const;
	// The workouts to choose from: the built-in ones, then any in Saved/Veloverse/Workouts.
	static TArray<Veloverse::FWorkout> WorkoutLibrary(float RiderFtp);
	static FString WorkoutsFolder();

	// Where power comes from: "keyboard", or "sim", a Bluetooth trainer played from the keys.
	static const TArray<FString> TrainerSources;
	FString TrainerSource;
	void UseTrainer(const FString& Source);

	// An unattended ride, for tests and stills: any of -RideStart, -RideFrom, -RideWatts, -RideShots or
	// -RideNoMenu skips the start screen and the menus.
	static bool Automated();

private:
	// The route and start picked on the start screen, for the reloaded level after "Back to the
	// start". ChosenStart is metres, negative for none.
	static FString ChosenRoute;
	static float ChosenStart;
	bool bStartGiven = false; // -RideStart or -RideFrom on the command line
	float DefaultStart() const;
	TUniquePtr<ITrainer> Trainer;
	// The keys always drive these legs: as the rider's power, or as a simulated trainer's rider.
	FKeyboardTrainer Legs;
	Veloverse::FRider Rider;
	float WeatherClock = 0.f;

	// Unattended rides for tests and stills, from the command line: -RideStart=METRES or
	// -RideFrom=STRETCH,
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
	// roll into a bend, eased, both in degrees. The sway's size follows the effort. Off unless the
	// ride starts with -RideSway: a steady view is easier to watch for an hour.
	bool bSway = false;
	float Crank = 0.f;
	float Sway = 0.f;
	float Lean = 0.f;
	// The mouse's look round, degrees on top of the road's view, and how long it has rested.
	float LookYaw = 0.f;
	float LookPitch = 0.f;
	float LookRest = 0.f;
	static constexpr float LookSpeed = 0.12f; // degrees per mouse count
	void MouseLook(APlayerController& Player);
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
