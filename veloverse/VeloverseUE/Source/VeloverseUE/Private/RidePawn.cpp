#include "RidePawn.h"

#include "BleTrainer.h"
#include "BlueZLink.h"

#include "Camera/CameraComponent.h"
#include "Components/DirectionalLightComponent.h"
#include "Components/ExponentialHeightFogComponent.h"
#include "Components/SkyLightComponent.h"
#include "Components/StaticMeshComponent.h"
#include "Engine/DirectionalLight.h"
#include "Engine/ExponentialHeightFog.h"
#include "Engine/PostProcessVolume.h"
#include "Engine/SkyLight.h"
#include "EngineUtils.h"
#include "GameFramework/PlayerController.h"
#include "Kismet/KismetSystemLibrary.h"
#include "Materials/MaterialParameterCollection.h"
#include "Materials/MaterialParameterCollectionInstance.h"
#include "HAL/FileManager.h"
#include "HAL/IConsoleManager.h"
#include "Misc/CommandLine.h"
#include "Misc/FileHelper.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "RidePlayerController.h"
#include "RHI.h"
#include "RideTrack.h"
#include "UnrealClient.h"
#include "Units.h"

namespace
{
constexpr float LookAhead = 600.f; // cm: the camera looks this far up the road
constexpr float GradeSpan = 800.f; // cm over which the camera's pitch follows the grade

// The rider's weather and time of day in place of the route's, as the coast route's rows set them:
// clear sun, the overcast before its shower, and the shower itself.
void Overrule(FRideWeather& W, float& Azimuth, int32 Weather, int32 TimeOfDay)
{
	if (TimeOfDay > 0)
	{
		static constexpr float Heights[3] = {15.f, 48.f, 8.f};
		static constexpr float Turns[3] = {-70.f, 0.f, 70.f}; // degrees round from the route's own sun
		W.Elevation = Heights[TimeOfDay - 1];
		Azimuth += Turns[TimeOfDay - 1];
	}
	if (Weather == 1)
	{
		W.Lux = W.Elevation < 12.f ? 70000.f : 90000.f;
		W.Sky = 1.f;
		W.Fog = 0.006f;
		W.Rain = W.Wet = 0.f;
	}
	else if (Weather == 2)
	{
		W.Lux = 25000.f;
		W.Sky = 0.8f;
		W.Fog = 0.025f;
		W.Rain = W.Wet = 0.f;
	}
	else if (Weather == 3)
	{
		W.Lux = 4000.f;
		W.Sky = 0.55f;
		W.Fog = 0.08f;
		W.Rain = W.Wet = 1.f;
	}
}
constexpr float SensorWidth = 23.76f;
constexpr float FocalLength = 20.f; // mm: a 61° view; the stills' 16 mm (73°) made the ride look fast
constexpr float PhysicsStep = 1.f / 120.f;
} // namespace

// A camera's look rather than a renderer's: a little over-exposed, highlights that roll off and
// lose colour as film and sensors do, shadows lifted a touch and cooled, light warmed, a soft
// vignette and a trace of lens fringing. All of it is in the tonemapper's one pass, so it costs
// nothing a frame. scripts/make_coast_ride.py gives the stills' camera the same.
void ARidePawn::CameraLook(FPostProcessSettings& Look)
{
	Look.bOverride_AutoExposureBias = true;
	Look.AutoExposureBias = 0.55f; // the level's volume has 0.3
	Look.bOverride_BloomIntensity = true;
	Look.BloomIntensity = 0.4f;
	Look.bOverride_FilmSlope = true;
	Look.FilmSlope = 0.84f;
	Look.bOverride_FilmToe = true;
	Look.FilmToe = 0.6f;
	Look.bOverride_FilmShoulder = true;
	Look.FilmShoulder = 0.34f;
	Look.bOverride_ColorSaturationHighlights = true;
	Look.ColorSaturationHighlights = FVector4(0.9f, 0.9f, 0.9f, 1.f);
	Look.bOverride_ColorGainHighlights = true;
	Look.ColorGainHighlights = FVector4(1.03f, 1.01f, 0.96f, 1.f);
	Look.bOverride_ColorOffsetShadows = true;
	Look.ColorOffsetShadows = FVector4(0.002f, 0.004f, 0.008f, 0.f);
	Look.bOverride_ColorSaturation = true;
	Look.ColorSaturation = FVector4(1.04f, 1.04f, 1.04f, 1.f);
	Look.bOverride_VignetteIntensity = true;
	Look.VignetteIntensity = 0.36f;
	Look.bOverride_SceneFringeIntensity = true;
	Look.SceneFringeIntensity = 0.25f;
}

const TArray<FString> ARidePawn::TrainerSources = {TEXT("keyboard"), TEXT("sim"), TEXT("ble")};

bool ARidePawn::Automated()
{
	const TCHAR* Line = FCommandLine::Get();
	float Unused;
	FString Text;
	return FParse::Value(Line, TEXT("RideStart="), Unused) || FParse::Value(Line, TEXT("RideFrom="), Text) ||
	       FParse::Value(Line, TEXT("RideWatts="), Unused) || FParse::Value(Line, TEXT("RideShots="), Text) ||
	       FParse::Param(Line, TEXT("RideNoMenu"));
}

ARidePawn::ARidePawn()
{
	PrimaryActorTick.bCanEverTick = true;
	Camera = CreateDefaultSubobject<UCameraComponent>(TEXT("Camera"));
	RootComponent = Camera;
	Camera->SetFieldOfView(FMath::RadiansToDegrees(2.f * FMath::Atan(SensorWidth / (2.f * FocalLength))));
	Camera->bConstrainAspectRatio = false;
	FPostProcessSettings& Look = Camera->PostProcessSettings;
	Look.bOverride_AutoExposureMethod = true;
	Look.AutoExposureMethod = AEM_Histogram;
	Look.bOverride_AutoExposureApplyPhysicalCameraExposure = true;
	Look.AutoExposureApplyPhysicalCameraExposure = false;
	Look.bOverride_BloomIntensity = true;
	Look.BloomIntensity = 0.25f;
	Look.bOverride_MotionBlurAmount = true;
	Look.MotionBlurAmount = 0.f;
	CameraLook(Look);

	Rain = CreateDefaultSubobject<UStaticMeshComponent>(TEXT("Rain"));
	Rain->SetupAttachment(Camera);
	// The shells hang level round the rider whichever way the camera looks.
	Rain->SetUsingAbsoluteRotation(true);
	Rain->SetCastShadow(false);
	Rain->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	Rain->SetMobility(EComponentMobility::Movable);

}

void ARidePawn::BeginPlay()
{
	Super::BeginPlay();
	// Each level keeps its track beside it: /Game/Coast/L_Coast rides /Game/Coast/Ride/DA_RideTrack.
	const FString Folder = FPackageName::GetLongPackagePath(UWorld::RemovePIEPrefix(GetWorld()->GetOutermost()->GetName()));
	TrackAsset = TSoftObjectPtr<URideTrack>(FSoftObjectPath(Folder + TEXT("/Ride/DA_RideTrack.DA_RideTrack")));
	Track = TrackAsset.LoadSynchronous();
	if (!Track || Track->Length() <= 0.f)
	{
		UE_LOG(LogTemp, Error, TEXT("RIDE no track at %s: run scripts/ride_track.py"), *TrackAsset.ToString());
	}
	else
	{
		Routes.Add(Track);
		for (const TSoftObjectPtr<URideTrack>& Other : Track->Routes)
		{
			if (URideTrack* Loaded = Other.LoadSynchronous(); Loaded && Loaded->Length() > 0.f)
			{
				Routes.Add(Loaded);
			}
		}
		// The route chosen last, kept through "Back to the start", which reloads the level; else
		// -RideRoute=TITLE.
		FString Wanted = ChosenRoute;
		FParse::Value(FCommandLine::Get(), TEXT("RideRoute="), Wanted);
		for (URideTrack* Route : Routes)
		{
			if (!Wanted.IsEmpty() && Route->Title.Equals(Wanted, ESearchCase::IgnoreCase))
			{
				Track = Route;
			}
		}
		UE_LOG(LogTemp, Display, TEXT("RIDE route %s of %d: %.1f km"), *Track->Title, Routes.Num(),
		       Track->Length() / 100000.f);
		if (Track->RainShell)
		{
			Rain->SetStaticMesh(Track->RainShell);
		}
	}
	ReadCommandLine();
	bStarted = bStarted || Automated();
	if (!bStartGiven)
	{
		// The start chosen before "Back to the start", if this route has it, else the track's own.
		StartMetres = DefaultStart();
		for (const FStart& Each : Starts())
		{
			StartMetres = FMath::IsNearlyEqual(Each.Metres, ChosenStart) ? ChosenStart : StartMetres;
		}
	}
	Distance = StartMetres * 100.0;
	Segments.Reset();
	FindWeather();
	ApplyWeather();
	Place();
}

FString ARidePawn::TrainerName() const { return Trainer ? Trainer->Name() : TEXT("None"); }

int32 ARidePawn::Zone(float Watts) const
{
	static constexpr float Tops[] = {0.55f, 0.75f, 0.90f, 1.05f, 1.20f, 1.50f};
	int32 Z = 0;
	while (Z < 6 && Watts >= Tops[Z] * Ftp)
	{
		++Z;
	}
	return Z;
}

FString ARidePawn::ChosenRoute;
float ARidePawn::ChosenStart = -1.f;

TArray<ARidePawn::FStart> ARidePawn::Starts() const
{
	TArray<FStart> Out;
	if (!Track)
	{
		return Out;
	}
	const TArray<FRideStretch>& Stretches = Track->Stretches;
	if (Stretches.IsEmpty() || Stretches[0].Metres > 0.f)
	{
		Out.Add({TEXT("Start"), 0.f});
	}
	for (const FRideStretch& Stretch : Stretches)
	{
		if (Stretch.Metres * 100.f < Track->Length())
		{
			Out.Add({Stretch.Name, Stretch.Metres});
		}
	}
	return Out;
}

int32 ARidePawn::StartIndex() const
{
	return Starts().IndexOfByPredicate([this](const FStart& Each)
	                                   { return FMath::IsNearlyEqual(Each.Metres, StartMetres, 1.f); });
}

void ARidePawn::UseStart(int32 Index)
{
	const TArray<FStart> All = Starts();
	if (bStarted || !All.IsValidIndex(Index))
	{
		return;
	}
	StartMetres = All[Index].Metres;
	ChosenStart = StartMetres;
	Distance = StartMetres * 100.0;
	Segments.Reset();
	bHeadSet = false;
	ApplyWeather();
	Place();
}

float ARidePawn::DefaultStart() const
{
	if (Track && !Track->StartAt.IsEmpty())
	{
		for (const FRideStretch& Stretch : Track->Stretches)
		{
			if (Stretch.Name.Equals(Track->StartAt, ESearchCase::IgnoreCase))
			{
				return Stretch.Metres;
			}
		}
	}
	return 0.f;
}

void ARidePawn::UseRoute(int32 Index)
{
	if (bStarted || !Routes.IsValidIndex(Index) || Routes[Index] == Track)
	{
		return;
	}
	Track = Routes[Index];
	ChosenRoute = Track->Title;
	if (!bStartGiven)
	{
		StartMetres = DefaultStart();
		ChosenStart = -1.f;
	}
	Distance = StartMetres * 100.0;
	Segments.Reset();
	if (Track->RainShell)
	{
		Rain->SetStaticMesh(Track->RainShell);
	}
	bHeadSet = false;
	ApplyWeather();
	Place();
}

float ARidePawn::TrackLength() const { return Track ? Track->Length() : 0.f; }

void ARidePawn::Tick(float Dt)
{
	Super::Tick(Dt);
	ReadKeys();
	if (!Track || Track->Length() <= 0.f)
	{
		return;
	}
	if (!bPaused && !bFinished)
	{
		Ride(Dt);
	}
	else if (Trainer)
	{
		// Paused or done: keep the link polled, but let the legs go: no ERG target, a flat road.
		// Riding again sets the workout's target afresh.
		Trainer->Tick(Dt);
		Reading = Trainer->Read();
		ReleaseTrainer();
	}
	Place();
	Automate(Dt);
	WeatherClock += Dt;
	if (WeatherClock > 0.2f)
	{
		WeatherClock = 0.f;
		ApplyWeather();
	}
}

void ARidePawn::ReleaseTrainer()
{
	if (!Trainer)
	{
		return;
	}
	if (Trainer->TargetPower())
	{
		Trainer->SetTargetPower(TOptional<double>());
	}
	Trainer->SetGrade(0.0);
}

void ARidePawn::EndPlay(const EEndPlayReason::Type Reason)
{
	if (bStarted && !bRideSaved)
	{
		EndRide();
	}
	// Leaving: the trainer shouldn't hold a workout's watts or a climb after the game has gone.
	if (Trainer)
	{
		ReleaseTrainer();
		Trainer->Tick(0.f); // sends it, if the link isn't holding back (most trainers drop ERG on disconnect too)
	}
	Super::EndPlay(Reason);
}

FString ARidePawn::EndRide()
{
	bFinished = true;
	ReleaseTrainer();
	UE_LOG(LogTemp, Display, TEXT("RIDE trainer released: %s"), Trainer ? *Trainer->Name() : TEXT("none"));
	if (bRideSaved)
	{
		return SavedRide;
	}
	bRideSaved = true;
	if (!Automated() || FParse::Param(FCommandLine::Get(), TEXT("RideRecord")))
	{
		FRideProfile Used = Profile;
		Used.Ftp = Ftp;
		// What the ride earned first, so its file keeps it for the review.
		const Veloverse::FSummary S = Recorder.Summary(Ftp);
		int32 Prs = 0;
		TArray<FRideClimb> Climbs;
		for (const FSegmentResult& R : Segments.Results())
		{
			const bool bPr = R.Ranking.Pr(R.Effort.Seconds);
			Prs += bPr ? 1 : 0;
			Climbs.Add({R.Name, R.Effort.Seconds, R.Effort.AverageWatts, R.Ranking.Rank, R.Ranking.Of, bPr});
		}
		const Veloverse::FRideXp Earned = Veloverse::RideXp(S.Metres, S.Climbed, Segments.Results().Num(), Prs, Workout.Done());
		SavedRide = Recorder.Save(Track ? Track->Title : FString(), StartMetres, Used, Climbs, Earned.Total());
		Segments.Save(FDateTime::Now().ToIso8601());
		if (!SavedRide.IsEmpty())
		{
			EarnedXp = Earned;
			XpBefore = Profile.Xp;
			Profile.Xp += EarnedXp.Total();
			Profile.Save();
			UE_LOG(LogTemp, Display, TEXT("RIDE xp +%d, now %lld (level %d)"), EarnedXp.Total(), Profile.Xp,
			       Veloverse::LevelFor(Profile.Xp).Level);
		}
		UE_LOG(LogTemp, Display, TEXT("RIDE saved %s (%d samples)"),
		       SavedRide.IsEmpty() ? TEXT("nothing: too short") : *SavedRide, static_cast<int32>(Recorder.Samples().size()));
	}
	return SavedRide;
}

void ARidePawn::SetProfile(const FRideProfile& Changed)
{
	Profile = Changed;
	Profile.Save();
	Ftp = Profile.Ftp;
	Rider.MassKg = Profile.WeightKg + Profile.BikeKg;
	UseSettings();
	ApplyWeather();
}

void ARidePawn::UseSettings()
{
	Veloverse::Units::bImperial = Profile.bImperial || FParse::Param(FCommandLine::Get(), TEXT("RideImperial"));
	bErg = Profile.bErg;
	// The automated rides and stills keep the lens and the grass they were framed with.
	if (Automated())
	{
		return;
	}
	Camera->SetFieldOfView(Profile.Fov);
	if (IConsoleVariable* Reach = IConsoleManager::Get().FindConsoleVariable(TEXT("grass.CullDistanceScale")))
	{
		Reach->Set(Profile.Grass / 100.f, ECVF_SetByCode);
	}
}

void ARidePawn::ReadCommandLine()
{
	const TCHAR* Line = FCommandLine::Get();
	bStartGiven = FParse::Value(Line, TEXT("RideStart="), StartMetres);
	FString From;
	if (FParse::Value(Line, TEXT("RideFrom="), From))
	{
		for (const FStart& Each : Starts())
		{
			if (Each.Name.Equals(From, ESearchCase::IgnoreCase))
			{
				StartMetres = Each.Metres;
				bStartGiven = true;
				break;
			}
		}
		UE_CLOG(!bStartGiven, LogTemp, Warning, TEXT("RIDE no stretch %s to start from"), *From);
	}
	FParse::Value(Line, TEXT("RideProfileAt="), ProfileAt);
	Profile = FRideProfile::Load();
	Ftp = Profile.Ftp;
	Rider.MassKg = Profile.WeightKg + Profile.BikeKg;
	UseSettings();
	FParse::Value(Line, TEXT("RideFtp="), Ftp);
	bSway = FParse::Param(Line, TEXT("RideSway"));
	Ftp = FMath::Max(Ftp, 50.f);
	FString Source = TrainerSources[0];
	FParse::Value(Line, TEXT("RideTrainer="), Source);
	UseTrainer(TrainerSources.Contains(Source) ? Source : TrainerSources[0]);
	FString WorkoutName;
	if (FParse::Value(Line, TEXT("RideWorkout="), WorkoutName))
	{
		for (const Veloverse::FWorkout& W : WorkoutLibrary(Ftp))
		{
			if (FString(UTF8_TO_TCHAR(W.Name.c_str())).Contains(WorkoutName))
			{
				StartWorkout(W);
				break;
			}
		}
		if (!Workout.Active())
		{
			UE_LOG(LogTemp, Warning, TEXT("RIDE no workout matches %s"), *WorkoutName);
		}
	}
	float Watts = 0.f;
	if (FParse::Value(Line, TEXT("RideWatts="), Watts))
	{
		Legs.Watts = Watts;
	}
	FString Shots;
	if (FParse::Value(Line, TEXT("RideShots="), Shots, false))
	{
		TArray<FString> Parts;
		Shots.ParseIntoArray(Parts, TEXT(","));
		for (const FString& Part : Parts)
		{
			ShotTimes.Add(FCString::Atof(*Part));
		}
		ShotTimes.Sort();
	}
	if (!FParse::Value(Line, TEXT("RideShotDir="), ShotDir))
	{
		ShotDir = FPaths::ProjectSavedDir() / TEXT("Ride");
	}
}

TOptional<double> ARidePawn::WorkoutTarget() const
{
	const std::optional<double> T = Workout.Target(Ftp, WorkoutBias);
	return T ? TOptional<double>(FMath::RoundToDouble(*T)) : TOptional<double>();
}

void ARidePawn::StartWorkout(const Veloverse::FWorkout& Chosen)
{
	Workout = Veloverse::FWorkoutPlayer(Chosen);
	WorkoutBias = 0.f;
	UE_LOG(LogTemp, Display, TEXT("RIDE workout %hs: %d steps, %.0f min"), Chosen.Name.c_str(),
	       static_cast<int32>(Chosen.Steps.size()), Chosen.Seconds() / 60.0);
}

void ARidePawn::EndWorkout()
{
	Workout = Veloverse::FWorkoutPlayer();
	if (Trainer)
	{
		Trainer->SetTargetPower(TOptional<double>());
	}
}

FString ARidePawn::WorkoutsFolder() { return FRideProfile::Folder() / TEXT("Workouts"); }

TArray<Veloverse::FWorkout> ARidePawn::WorkoutLibrary(float RiderFtp)
{
	TArray<Veloverse::FWorkout> Out;
	for (Veloverse::FWorkout& W : Veloverse::BuiltInWorkouts())
	{
		Out.Add(MoveTemp(W));
	}
	TArray<FString> Files;
	IFileManager::Get().FindFiles(Files, *(WorkoutsFolder() / TEXT("*")), true, false);
	Files.Sort();
	for (const FString& Name : Files)
	{
		FString Text;
		if (!FFileHelper::LoadFileToString(Text, *(WorkoutsFolder() / Name)))
		{
			continue;
		}
		if (std::optional<Veloverse::FWorkout> W =
		        Veloverse::WorkoutImport::Read(TCHAR_TO_UTF8(*Name), TCHAR_TO_UTF8(*Text), RiderFtp))
		{
			Out.Add(MoveTemp(*W));
		}
	}
	return Out;
}

void ARidePawn::UseTrainer(const FString& Source)
{
	TrainerSource = Source;
	if (Source == TEXT("sim"))
	{
		Trainer = MakeUnique<FBleTrainer>(MakeUnique<FSimulatedLink>(&Legs));
		return;
	}
	if (Source == TEXT("ble"))
	{
		Trainer = MakeUnique<FBleTrainer>(MakeUnique<FBlueZLink>());
		FString Device;
		if (FParse::Value(FCommandLine::Get(), TEXT("RideBleDevice="), Device))
		{
			Trainer->Pick(Device);
		}
		return;
	}
	// The keyboard trainer reads the legs; this proxy keeps one owner for both cases.
	struct FLegs : ITrainer
	{
		FKeyboardTrainer* Legs;
		explicit FLegs(FKeyboardTrainer* In) : Legs(In) {}
		virtual FString Name() const override
		{
			return Target ? FString::Printf(TEXT("Keyboard, ERG %.0f W"), *Target) : Legs->Name();
		}
		virtual FTrainerReading Read() const override { return Legs->Read(); }
		virtual void SetTargetPower(TOptional<double> Watts) override
		{
			ITrainer::SetTargetPower(Watts);
			Legs->SetTargetPower(Watts);
		}
	};
	Trainer = MakeUnique<FLegs>(&Legs);
}

void ARidePawn::Automate(float Dt)
{
	LogClock += Dt;
	SlowestFrame = FMath::Max(SlowestFrame, Dt);
	Hitches += Dt > 0.05f ? 1 : 0;
	++Frames;
	if (LogClock >= 5.f)
	{
		UE_LOG(LogTemp, Display,
		       TEXT("RIDE t=%.1f km=%.3f speed=%.1f km/h watts=%.0f grade=%+.1f%% fps=%.1f slowest=%.0fms "
		            "hitches=%d game=%.1fms draw=%.1fms gpu=%.1fms"),
		       Elapsed, Distance / 100000.0, Speed * 3.6f, Reading.Watts, Grade * 100.f, Frames / LogClock,
		       SlowestFrame * 1000.f, Hitches, FPlatformTime::ToMilliseconds(GGameThreadTime),
		       FPlatformTime::ToMilliseconds(GRenderThreadTime),
		       FPlatformTime::ToMilliseconds(RHIGetGPUFrameCycles()));
		LogClock = 0.f;
		SlowestFrame = 0.f;
		Hitches = 0;
		Frames = 0;
	}
	// -RideProfileAt=SECONDS logs one frame's GPU passes (ProfileGPU) there, to see what costs.
	if (ProfileAt >= 0.f && Elapsed >= ProfileAt)
	{
		ProfileAt = -1.f;
		if (APlayerController* Player = GetController<APlayerController>())
		{
			Player->ConsoleCommand(TEXT("ProfileGPU"));
		}
	}
	if (NextShot < ShotTimes.Num() && Elapsed >= ShotTimes[NextShot])
	{
		const FString File = ShotDir / FString::Printf(TEXT("ride_%02d.png"), NextShot);
		FScreenshotRequest::RequestScreenshot(File, true, false);
		UE_LOG(LogTemp, Display, TEXT("RIDE shot %s at t=%.1f km=%.3f"), *File, Elapsed, Distance / 100000.0);
		if (++NextShot == ShotTimes.Num())
		{
			QuitAt = Elapsed + 2.f;
		}
	}
	if (QuitAt >= 0.f && Elapsed >= QuitAt)
	{
		QuitAt = -1.f;
		UKismetSystemLibrary::QuitGame(this, GetController<APlayerController>(), EQuitPreference::Quit, false);
	}
}

void ARidePawn::ReadKeys()
{
	APlayerController* Player = GetController<APlayerController>();
	if (!Player)
	{
		return;
	}
	const bool Up = Player->IsInputKeyDown(EKeys::Up) || Player->IsInputKeyDown(EKeys::W);
	const bool Down = Player->IsInputKeyDown(EKeys::Down) || Player->IsInputKeyDown(EKeys::S);
	Legs.Direction = (Up ? 1 : 0) - (Down ? 1 : 0);
	MouseLook(*Player);
	if (Workout.Active() && !bPaused)
	{
		if (Player->WasInputKeyJustPressed(EKeys::RightBracket))
		{
			WorkoutBias = FMath::Min(WorkoutBias + 0.05f, 0.5f);
		}
		if (Player->WasInputKeyJustPressed(EKeys::LeftBracket))
		{
			WorkoutBias = FMath::Max(WorkoutBias - 0.05f, -0.5f);
		}
		if (Player->WasInputKeyJustPressed(EKeys::N))
		{
			Workout.Skip();
		}
		if (Player->WasInputKeyJustPressed(EKeys::E))
		{
			bErg = !bErg;
			Profile.bErg = bErg;
			Profile.Save();
		}
	}
	const bool bMenuKey = Player->WasInputKeyJustPressed(EKeys::Escape) || Player->WasInputKeyJustPressed(EKeys::P) ||
	                      Player->WasInputKeyJustPressed(EKeys::SpaceBar) ||
	                      Player->WasInputKeyJustPressed(EKeys::Gamepad_Special_Right);
	ARidePlayerController* Rides = Cast<ARidePlayerController>(Player);
	if (bMenuKey && Rides && !Automated())
	{
		Rides->ShowPause();
	}
	else if (Player->WasInputKeyJustPressed(EKeys::Escape))
	{
		UKismetSystemLibrary::QuitGame(this, Player, EQuitPreference::Quit, false);
	}
}

void ARidePawn::MouseLook(APlayerController& Player)
{
	// The mouse as a head: it turns the view up to a little past the shoulder and up or down, and
	// after a couple of seconds' rest the eyes drift back to the road. Not in a menu, where the
	// mouse is the menu's.
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	float Dx = 0.f, Dy = 0.f;
	if (!bPaused)
	{
		Player.GetInputMouseDelta(Dx, Dy);
	}
	if (!FMath::IsNearlyZero(Dx) || !FMath::IsNearlyZero(Dy))
	{
		LookYaw = FMath::Clamp(LookYaw + Dx * LookSpeed, -110.f, 110.f);
		LookPitch = FMath::Clamp(LookPitch + Dy * LookSpeed, -45.f, 35.f);
		LookRest = 0.f;
		return;
	}
	LookRest += Dt;
	if (LookRest > 2.f && Dt > 0.f)
	{
		const float Ease = 1.f - FMath::Exp(-Dt / 0.6f);
		LookYaw = FMath::Lerp(LookYaw, 0.f, Ease);
		LookPitch = FMath::Lerp(LookPitch, 0.f, Ease);
	}
}

void ARidePawn::Ride(float Dt)
{
	Legs.Tick(Dt);
	Trainer->Tick(Dt);
	Reading = Trainer->Read();
	Grade = Track->Grade(Track->Wrap(static_cast<float>(Distance)));
	Trainer->SetGrade(Grade * Profile.Difficulty / 100.f);
	if (Workout.Active())
	{
		Workout.Tick(Dt);
		const TOptional<double> Target = bErg ? WorkoutTarget() : TOptional<double>();
		if (Target != Trainer->TargetPower())
		{
			Trainer->SetTargetPower(Target);
		}
	}
	const float Before = Track->Height(Track->Wrap(static_cast<float>(Distance)));
	// Fixed substeps, so the speed doesn't depend on the frame rate.
	for (float Left = Dt; Left > 0.f; Left -= PhysicsStep)
	{
		const float H = FMath::Min(Left, PhysicsStep);
		Speed = Veloverse::Step(Rider, Speed, Reading.Watts, Grade, H);
		Distance += static_cast<double>(Speed) * 100.0 * H;
	}
	Elapsed += Dt;
	if (!Segments.UsesTrack(Track))
	{
		Segments.Use(*Track);
	}
	Segments.Tick(Distance / 100.0, Elapsed, Reading.Watts);
	ClimbedMetres += FMath::Max(0.f, Track->Height(Track->Wrap(static_cast<float>(Distance))) - Before) / 100.f;
	Joules += Reading.Watts * Dt;
	ZoneSeconds[Zone(Reading.Watts)] += Dt;
	Recorder.Tick(Dt, Ridden() / 100.0, Reading.Watts, Reading.CadenceRpm, Reading.HeartRateBpm, Speed,
	              Track->Height(Track->Wrap(static_cast<float>(Distance))) / 100.0, GetActorLocation().X / 100.0,
	              GetActorLocation().Y / 100.0);
	if (!Track->bClosed && Distance >= Track->Length())
	{
		Distance = static_cast<double>(Track->Length());
		Speed = 0.f;
		EndRide();
		ARidePlayerController* Rides = GetController<ARidePlayerController>();
		if (Rides && !Automated())
		{
			Rides->ShowSummary();
		}
	}
}

void ARidePawn::Place()
{
	if (!Track || Track->Length() <= 0.f)
	{
		return; // a level without a ride track (the spikes): BeginPlay has logged it
	}
	// Before the ride the start screen looks out from the woods; riding, the camera is the rider's.
	const URideTrack* On = Track;
	double At = Distance;
	const bool bBackdrop = !bStarted && Backdrop(On, At);
	if (bBackdrop != bOnBackdrop)
	{
		bOnBackdrop = bBackdrop;
		bHeadSet = false;
	}
	const float S = On->Wrap(static_cast<float>(At));
	const FVector Eye = On->Point(S, -On->KeepLeft, EyeHeight);
	// Along the rider's own line, not at the centreline: aiming there turned the view inward by
	// atan(KeepLeft / LookAhead), 4° at 45 cm, so the road never looked straight ahead.
	const FVector Ahead = On->Point(On->Wrap(S + LookAhead), -On->KeepLeft, EyeHeight);
	const float Yaw = FMath::RadiansToDegrees(FMath::Atan2(Ahead.Y - Eye.Y, Ahead.X - Eye.X));
	const float Rise = On->Height(On->Wrap(S + GradeSpan)) - On->Height(S);
	const float Pitch = FMath::RadiansToDegrees(FMath::Atan2(Rise, GradeSpan)) * 0.6f - 3.f;
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	FVector Lift = FVector::ZeroVector;
	float Roll = 0.f;
	float Turn = 0.f;
	if (Dt > 0.f)
	{
		// Pedalling, with -RideSway: the head rises and falls twice a turn of the crank, rocks once,
		// and turns a little. Stopped (paused, or at the start) it settles to nothing.
		const bool bMoving = bSway && !bPaused && !bFinished && Speed > 0.3f;
		const float Rpm = Reading.CadenceRpm > 20.0 ? static_cast<float>(Reading.CadenceRpm) : 85.f;
		if (bMoving)
		{
			Crank = FMath::Fmod(Crank + 2.f * PI * Rpm / 60.f * Dt, 2.f * PI * 100.f);
		}
		const float Effort = bMoving ? FMath::Clamp(static_cast<float>(Reading.Watts) / 250.f, 0.4f, 1.4f) : 0.f;
		Sway = FMath::Lerp(Sway, Effort, 1.f - FMath::Exp(-Dt / 0.8f));
		Lift.Z = Sway * (0.9f * FMath::Sin(2.f * Crank) + 0.3f * FMath::Sin(0.37f * Crank + 1.f));
		Roll = Sway * (0.45f * FMath::Sin(Crank) + 0.15f * FMath::Sin(0.23f * Crank + 2.f));
		Turn = Sway * (0.2f * FMath::Sin(0.5f * Crank + 0.5f) + 0.1f * FMath::Sin(0.17f * Crank));
		// Into a bend: the bike leans to atan(v^2 k / g), the head about a third of that.
		FVector Unused2;
		float Before, After;
		On->At(On->Wrap(S - 300.f), Unused2, Before);
		On->At(On->Wrap(S + 300.f), Unused2, After);
		const float Curve = FMath::DegreesToRadians(FMath::UnwindDegrees(After - Before)) / 6.f; // per metre
		const float Want = bSway ? FMath::RadiansToDegrees(FMath::Atan(Speed * Speed * Curve / 9.81f)) * 0.35f : 0.f;
		Lean = FMath::Lerp(Lean, FMath::Clamp(Want, -4.f, 4.f), 1.f - FMath::Exp(-Dt / 0.35f));
	}
	const FRotator Road(Pitch, Yaw, 0.f);
	if (!bHeadSet || Dt <= 0.f)
	{
		Head = Road;
		bHeadSet = true;
	}
	else
	{
		// A tenth of a second's lag: enough to round the kinks off, too little to swing wide on a
		// bend. Frame-rate independent, so a slow frame doesn't jerk the view.
		const float Ease = 1.f - FMath::Exp(-Dt / 0.1f);
		Head = FMath::Lerp(Head, Head + (Road - Head).GetNormalized(), Ease);
		Head.Normalize();
	}
	// The mouse turns the head on top of all that, the road's frame first and the look in it.
	const FQuat Body = (Head + FRotator(0.f, Turn, Roll + Lean)).Quaternion();
	SetActorLocationAndRotation(Eye + Lift, (Body * FRotator(LookPitch, LookYaw, 0.f).Quaternion()).Rotator());
}

bool ARidePawn::Backdrop(const URideTrack*& On, double& At) const
{
	// Greenwood's woods, 8 km in, where the trees close over the road, on whichever route passes through them.
	float Into = 8000.f;
	FParse::Value(FCommandLine::Get(), TEXT("RideBackdrop="), Into);
	TArray<const URideTrack*> Order = {Track.Get()};
	for (const URideTrack* Each : Routes)
	{
		Order.AddUnique(Each);
	}
	for (const URideTrack* Each : Order)
	{
		for (const FRideStretch& Stretch : Each ? Each->Stretches : TArray<FRideStretch>())
		{
			if (Stretch.Name.Equals(TEXT("Greenwood"), ESearchCase::IgnoreCase))
			{
				On = Each;
				At = (Stretch.Metres + Into) * 100.0;
				return true;
			}
		}
	}
	return false;
}

void ARidePawn::FindWeather()
{
	for (TActorIterator<ADirectionalLight> It(GetWorld()); It && !Sun.IsValid(); ++It)
	{
		Sun = Cast<UDirectionalLightComponent>(It->GetLightComponent());
	}
	for (TActorIterator<ASkyLight> It(GetWorld()); It && !SkyLight.IsValid(); ++It)
	{
		SkyLight = It->GetLightComponent();
	}
	for (TActorIterator<AExponentialHeightFog> It(GetWorld()); It && !Fog.IsValid(); ++It)
	{
		Fog = It->GetComponent();
	}
	for (TActorIterator<APostProcessVolume> It(GetWorld()); It && !Post.IsValid(); ++It)
	{
		Post = *It;
	}
}

void ARidePawn::ApplyWeather()
{
	const bool bOwn = Automated() || (Profile.Weather == 0 && Profile.TimeOfDay == 0);
	if (!Track || (Track->Weather.Num() == 0 && bOwn))
	{
		return;
	}
	FRideWeather W = Track->WeatherAt(Track->Wrap(static_cast<float>(Distance)) / 100.f);
	float Azimuth = Track->SunAzimuth;
	if (!bOwn)
	{
		Overrule(W, Azimuth, Profile.Weather, Profile.TimeOfDay);
	}
	if (Track->Collection)
	{
		if (UMaterialParameterCollectionInstance* Values = GetWorld()->GetParameterCollectionInstance(Track->Collection))
		{
			Values->SetScalarParameterValue(TEXT("Rain"), W.Rain);
			Values->SetScalarParameterValue(TEXT("Wet"), W.Wet);
		}
	}
	// The shells cost a translucent pass over the whole screen: draw them only while it rains.
	Rain->SetVisibility(W.Rain > 0.001f);
	if (Sun.IsValid())
	{
		Sun->SetIntensity(W.Lux);
		Sun->GetOwner()->SetActorRotation(FRotator(-W.Elevation, Azimuth, 0.f));
	}
	if (SkyLight.IsValid())
	{
		SkyLight->SetIntensity(W.Sky);
	}
	if (Fog.IsValid())
	{
		Fog->SetFogDensity(W.Fog);
	}
	if (Post.IsValid())
	{
		// Exposure stays within a few stops of what the light calls for, as in the stills.
		const float Ev = FMath::Log2((W.Lux + 8000.f * W.Sky) / 2.5f);
		Post->Settings.bOverride_AutoExposureMinBrightness = true;
		Post->Settings.bOverride_AutoExposureMaxBrightness = true;
		Post->Settings.AutoExposureMinBrightness = Ev - 6.5f;
		Post->Settings.AutoExposureMaxBrightness = Ev + 0.5f;
	}
}
