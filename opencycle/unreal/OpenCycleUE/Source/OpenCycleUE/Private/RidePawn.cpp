#include "RidePawn.h"

#include "BleTrainer.h"

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
#include "Misc/CommandLine.h"
#include "Misc/Paths.h"
#include "RidePlayerController.h"
#include "RHI.h"
#include "RideTrack.h"
#include "UnrealClient.h"

namespace
{
constexpr float LookAhead = 600.f; // cm: the camera looks this far up the road
constexpr float GradeSpan = 800.f; // cm over which the camera's pitch follows the grade
constexpr float SensorWidth = 23.76f;
constexpr float FocalLength = 16.f; // mm, as the stills
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

const TArray<FString> ARidePawn::TrainerSources = {TEXT("keyboard"), TEXT("sim")};

bool ARidePawn::Automated()
{
	const TCHAR* Line = FCommandLine::Get();
	float Unused;
	FString Text;
	return FParse::Value(Line, TEXT("RideStart="), Unused) || FParse::Value(Line, TEXT("RideWatts="), Unused) ||
	       FParse::Value(Line, TEXT("RideShots="), Text) || FParse::Param(Line, TEXT("RideNoMenu"));
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

	TrackAsset = TSoftObjectPtr<URideTrack>(FSoftObjectPath(TEXT("/Game/Coast/Ride/DA_RideTrack.DA_RideTrack")));
}

void ARidePawn::BeginPlay()
{
	Super::BeginPlay();
	Track = TrackAsset.LoadSynchronous();
	if (!Track || Track->Length() <= 0.f)
	{
		UE_LOG(LogTemp, Error, TEXT("RIDE no track at %s: run scripts/ride_track.py"), *TrackAsset.ToString());
	}
	else if (Track->RainShell)
	{
		Rain->SetStaticMesh(Track->RainShell);
	}
	ReadCommandLine();
	bStarted = bStarted || Automated();
	Distance = StartMetres * 100.0;
	FindWeather();
	ApplyWeather();
	Place();
}

FString ARidePawn::TrainerName() const { return Trainer ? Trainer->Name() : TEXT("None"); }

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
	Place();
	Automate(Dt);
	WeatherClock += Dt;
	if (WeatherClock > 0.2f)
	{
		WeatherClock = 0.f;
		ApplyWeather();
	}
}

void ARidePawn::ReadCommandLine()
{
	const TCHAR* Line = FCommandLine::Get();
	FParse::Value(Line, TEXT("RideStart="), StartMetres);
	FParse::Value(Line, TEXT("RideProfileAt="), ProfileAt);
	FString Source = TrainerSources[0];
	FParse::Value(Line, TEXT("RideTrainer="), Source);
	UseTrainer(TrainerSources.Contains(Source) ? Source : TrainerSources[0]);
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

void ARidePawn::UseTrainer(const FString& Source)
{
	TrainerSource = Source;
	if (Source == TEXT("sim"))
	{
		Trainer = MakeUnique<FBleTrainer>(MakeUnique<FSimulatedLink>(&Legs));
		return;
	}
	// The keyboard trainer reads the legs; this proxy keeps one owner for both cases.
	struct FLegs : ITrainer
	{
		const FKeyboardTrainer* Legs;
		explicit FLegs(const FKeyboardTrainer* In) : Legs(In) {}
		virtual FString Name() const override { return Legs->Name(); }
		virtual FTrainerReading Read() const override { return Legs->Read(); }
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

void ARidePawn::Ride(float Dt)
{
	Legs.Tick(Dt);
	Trainer->Tick(Dt);
	Reading = Trainer->Read();
	Grade = Track->Grade(Track->Wrap(static_cast<float>(Distance)));
	Trainer->SetGrade(Grade);
	// Fixed substeps, so the speed doesn't depend on the frame rate.
	for (float Left = Dt; Left > 0.f; Left -= PhysicsStep)
	{
		const float H = FMath::Min(Left, PhysicsStep);
		Speed = OpenCycle::Step(Rider, Speed, Reading.Watts, Grade, H);
		Distance += static_cast<double>(Speed) * 100.0 * H;
	}
	Elapsed += Dt;
	if (!Track->bClosed && Distance >= Track->Length())
	{
		Distance = static_cast<double>(Track->Length());
		Speed = 0.f;
		bFinished = true;
		ARidePlayerController* Rides = GetController<ARidePlayerController>();
		if (Rides && !Automated())
		{
			Rides->ShowPause();
		}
	}
}

void ARidePawn::Place()
{
	const float S = Track->Wrap(static_cast<float>(Distance));
	const FVector Eye = Track->Point(S, -Track->KeepLeft, EyeHeight);
	FVector Ahead;
	float Unused;
	Track->At(Track->Wrap(S + LookAhead), Ahead, Unused);
	const float Yaw = FMath::RadiansToDegrees(FMath::Atan2(Ahead.Y - Eye.Y, Ahead.X - Eye.X));
	const float Rise = Track->Height(Track->Wrap(S + GradeSpan)) - Track->Height(S);
	const float Pitch = FMath::RadiansToDegrees(FMath::Atan2(Rise, GradeSpan)) * 0.6f - 3.f;
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	FVector Lift = FVector::ZeroVector;
	float Roll = 0.f;
	float Turn = 0.f;
	if (Dt > 0.f)
	{
		// Pedalling: the head rises and falls twice a turn of the crank, rocks once, and turns a
		// little. Stills get it too, but stopped (paused, or at the start) it settles to nothing.
		const bool bMoving = !bPaused && !bFinished && Speed > 0.3f;
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
		Track->At(Track->Wrap(S - 300.f), Unused2, Before);
		Track->At(Track->Wrap(S + 300.f), Unused2, After);
		const float Curve = FMath::DegreesToRadians(FMath::UnwindDegrees(After - Before)) / 6.f; // per metre
		const float Want = FMath::RadiansToDegrees(FMath::Atan(Speed * Speed * Curve / 9.81f)) * 0.35f;
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
	SetActorLocationAndRotation(Eye + Lift, Head + FRotator(0.f, Turn, Roll + Lean));
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
	if (!Track || Track->Weather.Num() == 0)
	{
		return;
	}
	const FRideWeather W = Track->WeatherAt(Track->Wrap(static_cast<float>(Distance)) / 100.f);
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
		Sun->GetOwner()->SetActorRotation(FRotator(-W.Elevation, Track->SunAzimuth, 0.f));
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
