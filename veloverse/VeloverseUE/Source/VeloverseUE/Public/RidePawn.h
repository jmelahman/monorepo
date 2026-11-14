// The rider: carried along the track at the speed their power gives, with the camera where the
// stills put it (keeping left, eye height, looking a few metres up the road), the weather following
// where they are, and the rain shells carried round them.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/Pawn.h"
#include "RidePhysics.h"
#include "RideRecord.h"
#include "Trainer.h"
#include "Progress.h"
#include "RideSegments.h"
#include "Workout.h"
#include "Dock.h"
#include "RidePawn.generated.h"

class UCameraComponent;
class UStaticMeshComponent;
class UStaticMesh;
class USkeletalMeshComponent;
class UAnimSequence;
class URideTrack;
struct FPostProcessSettings;
class UDirectionalLightComponent;
class USkyLightComponent;
class UExponentialHeightFogComponent;
class APostProcessVolume;
class APlayerController;

// Where the camera rides. Eyes: the rider's, with the body's motion (a line that wanders across the
// lane and takes the inside of bends, the lean into them, the pedalling and the pull of a surge).
// Cockpit: the same over the bike's bars. Chase: behind the bike. Steady: the eyes on rails, none
// of the motion. Cockpit and Chase need the bike's meshes (CockpitMesh and BikeMesh). The values
// are saved in the profile, so new ones go at the end.
enum class ERideView : uint8
{
	Eyes,
	Cockpit,
	Chase,
	Steady,
};

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
	// The bike seen from behind, on the road under the rider, and its bars seen from the saddle,
	// fixed to the camera. Each is loaded from its path if it's there (/Game/Rider).
	UPROPERTY(VisibleAnywhere) TObjectPtr<UStaticMeshComponent> Bike;
	UPROPERTY(VisibleAnywhere) TObjectPtr<UStaticMeshComponent> Cockpit;
	static constexpr const TCHAR* BikeMesh = TEXT("/Game/Rider/SM_Bike.SM_Bike");
	static constexpr const TCHAR* CockpitMesh = TEXT("/Game/Rider/SM_Cockpit.SM_Cockpit");
	static constexpr const TCHAR* RiderMesh = TEXT("/Game/Rider/Body/SK_Rider.SK_Rider");
	static constexpr const TCHAR* RiderAnim = TEXT("/Game/Rider/Body/A_Rider_Pedal.A_Rider_Pedal");
	// The Blender bike's parts (/Game/Rider/Bike, import_bike_parts.py), under Bike in place of
	// its one mesh when all are there: the wheels turn with the distance, the cranks with Crank
	// and the pedals stay level. In EBikePart's order.
	UPROPERTY(Transient) TArray<TObjectPtr<UStaticMeshComponent>> BikeParts;
	// The Blender rider (/Game/Rider/Body, import_rider.py) on the parts, seen from behind: its
	// one Pedal loop (a crank revolution) scrubbed to Crank, so the feet stay on the pedals. Over
	// the bars it is hidden but casts its shadow, and RiderArms shows instead: a copy without the
	// head or legs, tipped back ArmsTilt degrees about the hands, so the hands stay on the hoods
	// while the shoulders drop below the frame and the forearms reach in from its foot.
	UPROPERTY(Transient) TObjectPtr<USkeletalMeshComponent> RiderBody;
	UPROPERTY(Transient) TObjectPtr<USkeletalMeshComponent> RiderArms;
	float ArmsTilt = 57.f;
	UPROPERTY(Transient) TObjectPtr<UAnimSequence> RiderPedal;
	UPROPERTY(EditAnywhere) TSoftObjectPtr<URideTrack> TrackAsset;
	UPROPERTY(Transient) TObjectPtr<URideTrack> Track;
	// The level's routes, its default track first; Track is one of them.
	UPROPERTY(Transient) TArray<TObjectPtr<URideTrack>> Routes;
	// Rides Routes[Index] from its start. Mid-ride the ride carries on there: its time, distance,
	// segments and recording go on, and it's saved under both routes' names.
	void UseRoute(int32 Index);
	bool bChangedRoute = false;
	FString RideRoutes; // "Coast Loop, then Vael", once the route has changed
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
	// Moves the rider to Starts()[Index]; mid-ride, the ride carries on from there.
	void UseStart(int32 Index);
	// cm ridden since the start, wherever it was, across any jumps and changes of route.
	double Ridden() const { return RiddenBefore + Distance - StartMetres * 100.0; }

	// The views there are meshes for, in the order V steps through them, and the one in use
	// (Profile.View, if it's there; the eyes otherwise). CycleView steps to the next (V).
	TArray<ERideView> Views() const;
	ERideView View() const;
	void CycleView();

	// The dock along the foot of the screen (Dock.h): it rises when the mouse moves, and the mouse
	// turns the head while a button is held. In free look (Tab, or the dock's button) the mouse
	// turns the head on its own, with no cursor or dock, until Tab brings them back.
	Veloverse::FDockReveal Dock;
	int32 DockHover = -1; // the button under the pointer, or -1
	bool bFreeLook = false;
	void SetFreeLook(bool bOn);
	void PressDock(Veloverse::EDock Button);

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
	// The start screen's view: a spot in Greenwood's woods, whatever the route. False if no route
	// reaches Greenwood, and the rider's own start serves.
	bool Backdrop(const URideTrack*& On, double& At) const;
	bool bOnBackdrop = false;
	float RiderFov = 0.f; // the rider's lens while the start screen's view has the board's
	FTrainerReading Reading;
	float ClimbedMetres = 0.f;
	double Joules = 0.0;
	// Power zones as shares of FTP (-RideFtp=, 250 W by default), Coggan's seven: Zone gives 0..6.
	float Ftp = 250.f;
	int32 Zone(float Watts) const;
	float ZoneSeconds[7] = {};
	FString TrainerName() const;

	// The rider's FTP and weight, kept in ~/.config/Veloverse/profile.json. -RideFtp= overrides the FTP
	// for one ride. SetProfile saves it and rides with it from now.
	FRideProfile Profile;
	void SetProfile(const FRideProfile& Changed);
	// The ride so far, a sample a second, saved when it ends: at the finish, on "End ride", or when
	// the level goes (back to the start, quit). Unattended rides aren't kept unless -RideRecord.
	FRideRecorder Recorder;
	// Ends the ride and saves it, once; returns the file, or "" when it wasn't kept.
	FString EndRide();
	// Clears any ERG target and flattens the road on the trainer: on pause, at the end, on leaving.
	void ReleaseTrainer();
	FString SavedRide;
	bool bRideSaved = false;
	// What the kept ride earned toward the rider's level, and the XP before it.
	Veloverse::FRideXp EarnedXp;
	int64 XpBefore = 0;
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
	// The workouts to choose from: the built-in ones, then any in ~/.config/Veloverse/Workouts.
	static TArray<Veloverse::FWorkout> WorkoutLibrary(float RiderFtp);
	static FString WorkoutsFolder();

	// Where power comes from: "keyboard", or "sim", a Bluetooth trainer played from the keys.
	static const TArray<FString> TrainerSources;
	FString TrainerSource;
	void UseTrainer(const FString& Source);
	ITrainer* CurrentTrainer() const { return Trainer.Get(); }

	// An unattended ride, for tests and stills: any of -RideStart, -RideFrom, -RideWatts, -RideShots or
	// -RideNoMenu skips the start screen and the menus.
	static bool Automated();

private:
	// The route and start picked on the start screen, for the reloaded level after "Back to the
	// start". ChosenStart is metres, negative for none.
	static FString ChosenRoute;
	static float ChosenStart;
	bool bStartGiven = false; // -RideStart or -RideFrom on the command line
	double RiddenBefore = 0.0; // cm ridden before the last jump or change of route
	FVector2f MouseAt = FVector2f(-1.f, -1.f); // in the HUD's frame
	bool bPressOnDock = false; // the button went down on the dock, so the drag doesn't look
	void Docking(APlayerController& Player);
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
	// roll into a bend, eased, both in degrees. The sway's size follows the effort. In every view
	// but Steady; tests and stills ride steady unless they start with -RideSway.
	bool bSway = false;
	float Crank = 0.f;
	float Sway = 0.f;
	float Lean = 0.f;
	// The rider's line: cm toward the inside of the bend, eased, on top of the drift across the
	// lane (RideLine), and the speed's change, eased, which tips the head back on a surge.
	float Apex = 0.f;
	float Surge = 0.f;
	float LastSpeed = -1.f;
	float RainTilt = 0.f; // degrees the rain slants from ahead, eased
	float Motion = 0.f; // 0 steady to 1 moving, eased, so changing view doesn't jump the line
	bool Moves() const;
	// Cm right of the centreline the rider rides at S: KeepLeft, wandering with the distance.
	float RideLine(const class URideTrack& On, float S) const;
	// The mouse's look round, degrees on top of the road's view, and how long it has rested.
	float LookYaw = 0.f;
	float LookPitch = 0.f;
	float LookRest = 0.f;
	static constexpr float LookSpeed = 0.12f; // degrees per mouse count, at the profile's 100%
	void MouseLook(APlayerController& Player);
	float QuitAt = -1.f;
	float ProfileAt = -1.f;
	void ReadCommandLine();
	void Automate(float Dt);

	TWeakObjectPtr<UDirectionalLightComponent> Sun;
	float SunAngle = -1.f; // the level's own sun disc, degrees, read on the first weather
	TWeakObjectPtr<USkyLightComponent> SkyLight;
	TWeakObjectPtr<UExponentialHeightFogComponent> Fog;
	TWeakObjectPtr<class USkyAtmosphereComponent> Atmosphere;
	float Rayleigh = 0.f, Mie = 0.f, Absorption = 0.f;
	TWeakObjectPtr<APostProcessVolume> Post;

	static void CameraLook(FPostProcessSettings& Look);
	void ReadKeys();
	void Ride(float Dt);
	void Place();
	void PlaceRain(float Dt);
	void MakeBikeParts();
	void MakeRider();
	void PoseBike();
	bool HasBike() const;
	void FindWeather();
	void ApplyWeather();
	// The profile's settings: the units, the view, ERG and the grass's reach.
	void UseSettings();
	void UseGrassReach(); // grass.CullDistanceScale from the Grass setting, net of r.ViewDistanceScale
	float GrassViewScale = -1.f; // the r.ViewDistanceScale it was set against
};
