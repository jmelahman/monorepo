#include "RidePawn.h"

#include "BleTrainer.h"
#include "BlueZLink.h"

#include "Camera/CameraComponent.h"
#include "Components/DirectionalLightComponent.h"
#include "Components/ExponentialHeightFogComponent.h"
#include "Components/SkyAtmosphereComponent.h"
#include "Components/SkyLightComponent.h"
#include "Animation/AnimSequence.h"
#include "Components/PointLightComponent.h"
#include "Components/SceneCaptureComponent2D.h"
#include "Components/SkeletalMeshComponent.h"
#include "Components/SpotLightComponent.h"
#include "Engine/TextureRenderTarget2D.h"
#include "ImageUtils.h"
#include "Components/StaticMeshComponent.h"
#include "Engine/DirectionalLight.h"
#include "Engine/Engine.h"
#include "Engine/GameViewportClient.h"
#include "Engine/ExponentialHeightFog.h"
#include "Engine/PostProcessVolume.h"
#include "Engine/SkyLight.h"
#include "Engine/SkeletalMesh.h"
#include "Engine/StaticMesh.h"
#include "EngineUtils.h"
#include "GameFramework/PlayerController.h"
#include "Kismet/KismetSystemLibrary.h"
#include "Materials/MaterialInstanceDynamic.h"
#include "Materials/MaterialParameterCollection.h"
#include "Materials/MaterialParameterCollectionInstance.h"
#include "HAL/FileManager.h"
#include "HAL/IConsoleManager.h"
#include "Misc/CommandLine.h"
#include "Misc/FileHelper.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "RidePlayerController.h"
#include "RiderAnim.h"
#include "RHI.h"
#include "RideTrack.h"
#include "UnrealClient.h"
#include "InputCoreTypes.h"
#include "Units.h"

namespace
{
constexpr float LookAhead = 600.f; // cm: the camera looks this far up the road
constexpr float GradeSpan = 800.f; // cm over which the camera's pitch follows the grade
// Behind the bike: the camera this far back along the look from a point ChaseAim over the road,
// tipped down. Measured from a Zwift screenshot the user sent (14 Nov): the horizon sits 45% down
// the frame, the rider's helmet 53% and the wheels at the bottom edge, so the camera is just over
// the rider's head (1.8 m), barely tipped (2 degrees), and at the game's 61 degree lens 5.1 m back.
// Before: 4.2 m back aimed at the bike's middle, 9 degrees down; 3 m back at 1.8 m read as over
// the bars, and 6.2 m back at 3 m, 12 degrees down, was a guess at Zwift's from memory.
// The eyes moved 22.8 cm up the bike to the rider's (40.8 cm ahead of the bottom bracket, from 18),
// so the camera stays put over the bike with the aim 22.8 cm further along the same line.
constexpr float ChaseBack = 532.8f;
constexpr float ChaseAim = 162.f;
constexpr float ChaseTilt = -2.f;
constexpr float ChaseGrade = 0.35f; // the share of the road's pitch the camera follows (was 0.5)
constexpr float ChaseGrassReach = 1.3f; // the grass reach from behind, times the rider's setting
const TCHAR* const GrassCollection = TEXT("/Game/Climb/Grass/MPC_Grass.MPC_Grass");
// Over the bars: the camera this far behind and below the eyes, in the bike's frame (cm), tipped
// down so the bars, hoods and computer sit in the bottom third with the horizon in the top third
// at the default 61 degree field of view. Nearer the bars, the narrow view can't hold both: the bars
// fall out of the frame's foot or the horizon out of its top. make_bike.py's COCKPIT_CAMERA holds
// the same numbers for its preview. That is for make_bike.py's front end, made for eyes 150 cm up
// and 18 cm ahead of the bottom bracket; over the Blender bike the camera is CockpitCamera.
const FVector CockpitEye(-45.f, 0.f, -38.f);
// Over the Blender bike: the camera in the bike's frame (cm, from the ground under the bottom
// bracket), where rider/README.md's Cockpit action is made to be seen from.
const FVector CockpitCamera(-27.f, 0.f, 112.f);
// The Blender bike's origin is the road under the rider's eyes, with its bottom bracket 40.8 cm
// behind them: the Pedal effort's eye point, (0.408, 0, 1.375) m from the bottom bracket's ground
// (rider/README.md). Before the rider, the eyes were a guess at 18 cm ahead and 150 cm up.
const FVector PartsOrigin(-40.8f, 0.f, 0.f);
constexpr float CockpitTilt = -8.f;

// A mesh by path, or none when it isn't in the project: no warning for a bike not yet made.
UStaticMesh* MeshAt(const TCHAR* Path)
{
	const FSoftObjectPath Object(Path);
	if (!FPackageName::DoesPackageExist(Object.GetLongPackageName()))
	{
		return nullptr;
	}
	return Cast<UStaticMesh>(Object.TryLoad());
}

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
		W.Fog = 0.012f; // a light haze that layers the far hills; 0.006 left them flat and sharp
		W.Rain = W.Wet = 0.f;
	}
	else if (Weather == 2)
	{
		// An overcast: soft shadows from a sun about twice as strong, against the sky, as the rain's.
		// 25000 lux against a 0.8 sky drew clear-day shadows (out/qa/cloudy1), and 2500 against 2.5
		// still drew them dark and crisp (cloudy2).
		W.Lux = 1200.f;
		W.Sky = 3.f;
		W.Fog = 0.025f;
		W.Rain = W.Wet = 0.f;
	}
	else if (Weather == 3)
	{
		// Under rain cloud the light is nearly all sky. The sky light is captured from the atmosphere,
		// which the sun lights, so dimming the sun dims the sky with it and the shadows keep their
		// contrast (out/qa/rain8, rain9): the sky's own multiplier has to carry the shift.
		W.Lux = 800.f;
		W.Sky = 3.f;
		W.Fog = 0.08f;
		W.Rain = W.Wet = 1.f;
	}
}
constexpr float SensorWidth = 23.76f;
constexpr float FocalLength = 20.f; // mm: a 61° view; the stills' 16 mm (73°) made the ride look fast
constexpr float PhysicsStep = 1.f / 120.f;
} // namespace

// The design board's Vael still: where its camera stood, in cm (the world's "ride_02_vael" view
// in v43, from make_world_level.py's info.json). The start screen rides the nearest road to it.
static const FVector BoardEye(-6022.73 * 100.0, -6151.28 * 100.0, 445.19 * 100.0);

// A camera's look rather than a renderer's: exposed for the highlights, which roll off and lose
// colour as film and sensors do, a touch less saturated than life so the world reads painted
// rather than photographed, shadows lifted a touch and cooled, light warmed, a soft vignette and
// a trace of lens fringing. All of it is in the tonemapper's one pass, so it costs
// nothing a frame. scripts/make_coast_ride.py gives the stills' camera the same.
void ARidePawn::CameraLook(FPostProcessSettings& Look)
{
	Look.bOverride_AutoExposureBias = true;
	// 0.55 until 0.3.0 washed pale surfaces out to white (13 Nov); the level's volume has 0.3.
	Look.AutoExposureBias = -0.1f;
	Look.bOverride_BloomIntensity = true;
	Look.BloomIntensity = 0.4f;
	Look.bOverride_FilmSlope = true;
	Look.FilmSlope = 0.84f;
	Look.bOverride_FilmToe = true;
	Look.FilmToe = 0.6f;
	Look.bOverride_FilmShoulder = true;
	Look.FilmShoulder = 0.34f;
	Look.bOverride_ColorSaturationHighlights = true;
	Look.ColorSaturationHighlights = FVector4(0.85f, 0.85f, 0.85f, 1.f);
	Look.bOverride_ColorGainHighlights = true;
	Look.ColorGainHighlights = FVector4(1.05f, 1.01f, 0.93f, 1.f);
	Look.bOverride_ColorOffsetShadows = true;
	Look.ColorOffsetShadows = FVector4(0.002f, 0.004f, 0.008f, 0.f);
	Look.bOverride_ColorSaturation = true;
	Look.ColorSaturation = FVector4(0.94f, 0.94f, 0.94f, 1.f);
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
	Rain->SetUsingAbsoluteLocation(true); // PlaceRain sets it, round the camera
	Rain->SetUsingAbsoluteScale(true);
	Rain->SetCastShadow(false);
	Rain->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	Rain->SetMobility(EComponentMobility::Movable);

	// The bike stands on the road, wherever the camera has swung to, so it keeps its own transform.
	Bike = CreateDefaultSubobject<UStaticMeshComponent>(TEXT("Bike"));
	Bike->SetupAttachment(Camera);
	Bike->SetUsingAbsoluteLocation(true);
	Bike->SetUsingAbsoluteRotation(true);
	Bike->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	Bike->SetMobility(EComponentMobility::Movable);
	Bike->SetVisibility(false);
	// The bars sit under and ahead of the eyes, turning with the head.
	Cockpit = CreateDefaultSubobject<UStaticMeshComponent>(TEXT("Cockpit"));
	Cockpit->SetupAttachment(Camera);
	// Place() sets its transform each frame: the mesh's origin is the rider's eyes.
	Cockpit->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	Cockpit->SetCastShadow(false);
	Cockpit->SetMobility(EComponentMobility::Movable);
	Cockpit->SetVisibility(false);

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
	Bike->SetStaticMesh(MeshAt(BikeMesh));
	Cockpit->SetStaticMesh(MeshAt(CockpitMesh));
	MakeBikeParts();
	MakeRider();
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
	PlaceRain(0.f);
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
	if (!All.IsValidIndex(Index))
	{
		return;
	}
	if (bStarted)
	{
		RiddenBefore = Ridden(); // the ride goes on from there
	}
	StartMetres = All[Index].Metres;
	ChosenStart = StartMetres;
	Distance = StartMetres * 100.0;
	Segments.Reset(bStarted);
	bHeadSet = false;
	ApplyWeather();
	Place();
	PlaceRain(0.f);
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
	if (!Routes.IsValidIndex(Index) || Routes[Index] == Track)
	{
		return;
	}
	if (bStarted)
	{
		// Mid-ride: the ride goes on along the new route from its start, its segments and all.
		RiddenBefore = Ridden();
		RideRoutes = (RideRoutes.IsEmpty() ? Track->Title : RideRoutes) + TEXT(", then ") + Routes[Index]->Title;
		bChangedRoute = true;
	}
	Track = Routes[Index];
	ChosenRoute = Track->Title;
	if (bStarted || !bStartGiven)
	{
		StartMetres = DefaultStart();
		ChosenStart = -1.f;
	}
	Distance = StartMetres * 100.0;
	if (bStarted)
	{
		Segments.Use(*Track, true);
	}
	else
	{
		Segments.Reset();
	}
	if (Track->RainShell)
	{
		Rain->SetStaticMesh(Track->RainShell);
	}
	bHeadSet = false;
	ApplyWeather();
	Place();
	PlaceRain(0.f);
}

float ARidePawn::TrackLength() const { return Track ? Track->Length() : 0.f; }

void ARidePawn::Tick(float Dt)
{
	Super::Tick(Dt);
	TurnKitPreview(Dt);
	ReadKeys();
	static IConsoleVariable* ViewScale = IConsoleManager::Get().FindConsoleVariable(TEXT("r.ViewDistanceScale"));
	if (!Automated() && ViewScale && GrassViewScale != FMath::Max(ViewScale->GetFloat(), 0.1f))
	{
		UseGrassReach(); // the view distance changed on the graphics page
	}
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
	PlaceRain(Dt);
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
		const FString Route = bChangedRoute ? RideRoutes : Track ? Track->Title : FString();
		SavedRide = Recorder.Save(Route, StartMetres, Used, Climbs, Earned.Total());
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
	ApplyKit();
}

void ARidePawn::ApplyKit()
{
	using namespace Veloverse::Kit;
	// import_rider.py's slots: the skin and the kit share the body's masks (C1..C6: jersey, accent,
	// shorts, socks, hair, skin), the helmet's and shoe's own first channel is the shell and the
	// upper, and the brows (M_Hair) are flat. A part left at -1 keeps the instance's own colour.
	struct FSlot
	{
		const TCHAR* Instance;
		const TCHAR* Parameter;
		EPart Part;
	};
	static const FSlot Slots[] = {
		{TEXT("MI_Rider_Skin"), TEXT("C1"), Jersey}, {TEXT("MI_Rider_Skin"), TEXT("C2"), Accent},
		{TEXT("MI_Rider_Skin"), TEXT("C3"), Shorts}, {TEXT("MI_Rider_Skin"), TEXT("C4"), Socks},
		{TEXT("MI_Rider_Skin"), TEXT("C5"), Hair},   {TEXT("MI_Rider_Skin"), TEXT("C6"), Skin},
		{TEXT("MI_Rider_Kit"), TEXT("C1"), Jersey},  {TEXT("MI_Rider_Kit"), TEXT("C2"), Accent},
		{TEXT("MI_Rider_Kit"), TEXT("C3"), Shorts},  {TEXT("MI_Rider_Kit"), TEXT("C4"), Socks},
		{TEXT("MI_Rider_Kit"), TEXT("C5"), Hair},    {TEXT("MI_Rider_Kit"), TEXT("C6"), Skin},
		{TEXT("MI_Rider_Helmet"), TEXT("C1"), Helmet}, {TEXT("MI_Rider_Shoe"), TEXT("C1"), Shoes},
		{TEXT("MI_Rider_Hair"), TEXT("Colour"), Hair},
	};
	int32 Set = 0;
	for (USkeletalMeshComponent* C : {RiderBody.Get(), RiderArms.Get(), StudioRider.Get()})
	{
		const USkeletalMesh* Mesh = C ? C->GetSkeletalMeshAsset() : nullptr;
		if (!Mesh)
		{
			continue;
		}
		const TArray<FSkeletalMaterial>& Own = Mesh->GetMaterials();
		for (int32 I = 0; I < Own.Num(); ++I)
		{
			const FString Name = Own[I].MaterialInterface ? Own[I].MaterialInterface->GetName() : FString();
			UMaterialInstanceDynamic* Mid = nullptr;
			for (const FSlot& S : Slots)
			{
				if (Name != S.Instance)
				{
					continue;
				}
				// Back to the instance's own colour: a fresh dynamic instance starts from it.
				if (!Mid)
				{
					C->SetMaterial(I, Own[I].MaterialInterface);
					Mid = C->CreateAndSetMaterialInstanceDynamic(I);
				}
				if (Mid && Profile.Kit[S.Part] >= 0)
				{
					Mid->SetVectorParameterValue(S.Parameter, Pick(S.Part, Profile.Kit[S.Part]).Colour);
					++Set;
				}
			}
		}
	}
	UE_LOG(LogTemp, Display, TEXT("RIDE kit: %d colours set"), Set);
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
	if (RiderFov > 0.f)
	{
		RiderFov = Profile.Fov; // on the start screen's view: the rider's lens comes back with the ride
	}
	else
	{
		Camera->SetFieldOfView(Profile.Fov);
	}
	UseGrassReach();
}

void ARidePawn::UseGrassReach()
{
	// The lower view-distance qualities (r.ViewDistanceScale 0.4 on Low) pull every instance's cull
	// in, grass too, but the grass material's fade still shrinks it towards its full reach: so on Low
	// it vanished at full size, a hard edge close to the bike. Undo that scale for the grass, so its
	// reach is the rider's Grass setting alone and it fades out before it's culled; Low thins it by
	// density instead (DefaultScalability.ini).
	static IConsoleVariable* ViewScale = IConsoleManager::Get().FindConsoleVariable(TEXT("r.ViewDistanceScale"));
	static IConsoleVariable* Reach = IConsoleManager::Get().FindConsoleVariable(TEXT("grass.CullDistanceScale"));
	if (!ViewScale || !Reach)
	{
		return;
	}
	// From behind the camera stands higher and sees more of the ground ahead, so the carpet
	// reaches further there.
	const float Share = Profile.Grass / 100.f * (View() == ERideView::Chase ? ChaseGrassReach : 1.f);
	GrassViewScale = FMath::Max(ViewScale->GetFloat(), 0.1f);
	Reach->Set(Share / GrassViewScale, ECVF_SetByCode);
	// The blades shrink away over the last stretch before their cull, and the ground takes their
	// colour from there (MPC_Grass "Reach", oc_climb.py): both move with the cull. Before 14 Nov
	// the fade was fixed, so a reach over 100% only culled blades that had already shrunk away.
	if (UMaterialParameterCollection* Collection = LoadObject<UMaterialParameterCollection>(nullptr, GrassCollection))
	{
		if (UMaterialParameterCollectionInstance* Values = GetWorld()->GetParameterCollectionInstance(Collection))
		{
			Values->SetScalarParameterValue(TEXT("Reach"), Share);
		}
	}
}

void ARidePawn::ReadCommandLine()
{
	const TCHAR* Line = FCommandLine::Get();
	bStartGiven = FParse::Value(Line, TEXT("RideStart="), StartMetres);
	// For stills of the bike from aside: the look swung this far (degrees, + right). Past 45 it
	// stays put, as a free look held away does.
	if (float Yaw = 0.f; FParse::Value(Line, TEXT("RideLookYaw="), Yaw))
	{
		LookYaw = FMath::Clamp(Yaw, -110.f, 110.f);
	}
	// For stills of the arms over the bars: how far RiderArms tips back (degrees); below 0, no arms.
	if (FParse::Value(Line, TEXT("RideEffort="), PinnedEffort))
	{
		PinnedEffort = FMath::Clamp(PinnedEffort, 0.f, 2.f);
		RiderEffort = PinnedEffort;
	}
	if (FParse::Value(Line, TEXT("RideArmsTilt="), ArmsTilt))
	{
		bArmsTiltGiven = true;
		UE_LOG(LogTemp, Display, TEXT("RIDE arms tilt %.0f"), ArmsTilt);
	}
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
	ApplyKit(); // the rider was dressed before the profile was read, in the export's own colours
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
	FParse::Value(Line, TEXT("RideView="), ShotView);
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
	if (bStarted && !bPaused && !Automated())
	{
		Docking(*Player);
	}
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
	// after a couple of seconds' rest the eyes drift back to the road, unless the rider has turned
	// well away from it (more than 45 degrees aside or 25 up or down), to look at something: then
	// the view stays put until the mouse brings it back. Not in a menu, where the mouse is the menu's,
	// and with the dock about only while a button is held (and wasn't pressed on the dock).
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	float Dx = 0.f, Dy = 0.f;
	const bool bHeld = Player.IsInputKeyDown(EKeys::LeftMouseButton) || Player.IsInputKeyDown(EKeys::RightMouseButton);
	const bool bLooks = bFreeLook || Automated() || !bStarted || (bHeld && !bPressOnDock);
	if (!bPaused && bLooks)
	{
		Player.GetInputMouseDelta(Dx, Dy);
	}
	if (!FMath::IsNearlyZero(Dx) || !FMath::IsNearlyZero(Dy))
	{
		const float Turn = LookSpeed * Profile.LookSpeed / 100.f;
		LookYaw = FMath::Clamp(LookYaw + Dx * Turn, -110.f, 110.f);
		LookPitch = FMath::Clamp(LookPitch + Dy * Turn, -45.f, 35.f);
		LookRest = 0.f;
		return;
	}
	LookRest += Dt;
	const bool bAway = FMath::Abs(LookYaw) > 45.f || FMath::Abs(LookPitch) > 25.f;
	if (LookRest > 2.f && Dt > 0.f && !bAway)
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
	// Where the rider is, not the camera, which rides behind in the chase view.
	const FVector At = Track->Point(Track->Wrap(static_cast<float>(Distance)), -Track->KeepLeft, 0.f);
	Recorder.Tick(Dt, Ridden() / 100.0, Reading.Watts, Reading.CadenceRpm, Reading.HeartRateBpm, Speed,
	              Track->Height(Track->Wrap(static_cast<float>(Distance))) / 100.0, At.X / 100.0, At.Y / 100.0);
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
	FString Named;
	const bool bBoardView = !FParse::Value(FCommandLine::Get(), TEXT("RideBackdropIn="), Named);
	if (bBackdrop != bOnBackdrop)
	{
		bOnBackdrop = bBackdrop;
		bHeadSet = false;
		// The start screen's view keeps the game's exposure; -RideBackdropEv= stops it up or down.
		float Ev = 0.f;
		FParse::Value(FCommandLine::Get(), TEXT("RideBackdropEv="), Ev);
		FPostProcessSettings Plain;
		CameraLook(Plain);
		Camera->PostProcessSettings.AutoExposureBias = Plain.AutoExposureBias + (bBackdrop ? Ev : 0.f);
		// The board's stills were taken with the editor's 90-degree lens; riding, the rider's own.
		if (bBackdrop && bBoardView)
		{
			RiderFov = Camera->FieldOfView;
			Camera->SetFieldOfView(90.f);
		}
		else if (RiderFov > 0.f)
		{
			Camera->SetFieldOfView(RiderFov);
			RiderFov = 0.f;
		}
	}
	const float S = On->Wrap(static_cast<float>(At));
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	// The road's bend here (radians a metre, right positive), from 3 m either side.
	FVector Unused2;
	float Before, After;
	On->At(On->Wrap(S - 300.f), Unused2, Before);
	On->At(On->Wrap(S + 300.f), Unused2, After);
	const float Curve = FMath::DegreesToRadians(FMath::UnwindDegrees(After - Before)) / 6.f;
	const bool bMoves = Moves() && !bBackdrop;
	if (Dt > 0.f)
	{
		// Into a bend the rider takes its inside, up to 40 cm, more the tighter it is (30 cm on a
		// 40 m radius), and drifts back out of it over a couple of seconds.
		const float Want = bMoves ? FMath::Clamp(Curve * 1200.f, -40.f, 40.f) : 0.f;
		Apex = FMath::Lerp(Apex, Want, 1.f - FMath::Exp(-Dt / 1.5f));
		Motion = FMath::Lerp(Motion, bMoves ? 1.f : 0.f, 1.f - FMath::Exp(-Dt / 0.6f));
	}
	const FVector Eye = On->Point(S, RideLine(*On, S), EyeHeight);
	// Along the rider's own line, not at the centreline: aiming there turned the view inward by
	// atan(KeepLeft / LookAhead), 4° at 45 cm, so the road never looked straight ahead. The line
	// ahead wanders too, so the view turns gently as it does.
	const FVector Ahead = On->Point(On->Wrap(S + LookAhead), RideLine(*On, On->Wrap(S + LookAhead)), EyeHeight);
	float Yaw = FMath::RadiansToDegrees(FMath::Atan2(Ahead.Y - Eye.Y, Ahead.X - Eye.X));
	const float Rise = On->Height(On->Wrap(S + GradeSpan)) - On->Height(S);
	float Pitch = FMath::RadiansToDegrees(FMath::Atan2(Rise, GradeSpan)) * 0.6f - 3.f;
	if (bBackdrop && bBoardView)
	{
		// The board's still: straight up the road, as its camera was (30 m ahead, 40 cm lower). The
		// road's own heading, not the board's fixed bearing: the v45 roads bend round the land, and
		// the old bearing pointed the start screen off the tarmac.
		const double Run = FVector::Dist2D(Ahead, Eye);
		Pitch = FMath::RadiansToDegrees(FMath::Atan2(Ahead.Z - Eye.Z - 40.0 * Run / 3000.0, Run));
	}
	FVector Lift = FVector::ZeroVector;
	float Judder = 0.f; // degrees of nod
	float Roll = 0.f;
	float Turn = 0.f;
	float Rock = 0.f; // the pedalling's roll before the head's setting: the bike's, by its own
	if (Dt > 0.f)
	{
		// Pedalling: the head rises and falls twice a turn of the crank, rocks once, and turns a
		// little. Stopped (paused, or at the start) it settles to nothing.
		const bool bMoving = bMoves && !bPaused && !bFinished && Speed > 0.3f;
		const float Rpm = Reading.CadenceRpm > 20.0 ? static_cast<float>(Reading.CadenceRpm) : 85.f;
		if (bMoving)
		{
			Crank = FMath::Fmod(Crank + 2.f * PI * Rpm / 60.f * Dt, 2.f * PI * 100.f);
		}
		const float Effort = bMoving ? FMath::Clamp(static_cast<float>(Reading.Watts) / 250.f, 0.4f, 1.4f) : 0.f;
		// The pedalling and the road's judder at a base of 60% of what they were before 14 Nov, at the
		// user's request; the rider's settings (Rider: Head sway, Bike sway) scale them below.
		const float Strength = 0.6f;
		Sway = FMath::Lerp(Sway, Effort * Strength, 1.f - FMath::Exp(-Dt / 0.8f));
		// The rider's effort by W/kg, rider/README.md's blend: Easy to 1.5, Pedal at 3, Hard from 5;
		// eased over a couple of seconds so a surge doesn't snap the back down.
		const float Wkg = bMoving ? static_cast<float>(Reading.Watts) / FMath::Max(Profile.WeightKg, 30.f) : 0.f;
		const float EffortWant = PinnedEffort >= 0.f ? PinnedEffort
		                   : Wkg < 3.f        ? FMath::GetMappedRangeValueClamped(FVector2f(1.5f, 3.f), FVector2f(0.f, 1.f), Wkg)
		                                      : FMath::GetMappedRangeValueClamped(FVector2f(3.f, 5.f), FVector2f(1.f, 2.f), Wkg);
		RiderEffort = PinnedEffort >= 0.f ? EffortWant : FMath::Lerp(RiderEffort, EffortWant, 1.f - FMath::Exp(-Dt / 1.5f));
		Lift.Z = Sway * (0.9f * FMath::Sin(2.f * Crank) + 0.3f * FMath::Sin(0.37f * Crank + 1.f));
		// The rock with each stroke is small and its depth wanders, so it doesn't read as a metronome;
		// the head's turn is a slow drift, not a sine (both too strong and regular, the user, 14 Nov).
		const float Vary = 0.7f + 0.5f * FMath::PerlinNoise1D(S / 900.f + 5.f);
		Roll = Sway * (0.2f * Vary * FMath::Sin(Crank) + 0.12f * FMath::PerlinNoise1D(S / 300.f + 2.f));
		Turn = Sway * 0.12f * FMath::PerlinNoise1D(S / 600.f + 9.f);
		// The road through the bike: a judder tied to the distance, so it quickens with the speed and
		// stops with the bike. Noise over 0.6 m and 2.3 m (13 and 3.5 Hz at 30 km/h), about half a
		// centimetre up and down and a sixth of a degree of nod at that speed, more going faster.
		const float Rough = Motion * FMath::Clamp(Speed / 8.f, 0.f, 1.4f) * Strength;
		const float Fine = FMath::PerlinNoise1D(S / 60.f), Coarse = FMath::PerlinNoise1D(S / 230.f + 17.f);
		Lift.Z += Rough * (0.35f * Fine + 0.45f * Coarse);
		Judder = Rough * (0.12f * FMath::PerlinNoise1D(S / 60.f + 41.f) + 0.06f * Coarse);
		Roll += Rough * 0.1f * FMath::PerlinNoise1D(S / 90.f + 83.f);
		Rock = Roll;
		const float HeadScale = Profile.HeadSway / 100.f;
		Lift.Z *= HeadScale;
		Judder *= HeadScale;
		Roll *= HeadScale;
		Turn *= HeadScale;
		// Into a bend: the bike leans to atan(v^2 k / g), the head about a third of that.
		const float Want = bMoves ? FMath::RadiansToDegrees(FMath::Atan(Speed * Speed * Curve / 9.81f)) * 0.35f : 0.f;
		Lean = FMath::Lerp(Lean, FMath::Clamp(Want, -4.f, 4.f), 1.f - FMath::Exp(-Dt / 0.35f));
		// The fork turns to the bend's own angle, atan(wheelbase k), at most 3 degrees: the hands
		// stay on the hoods, which move a centimetre at that.
		const float Turning = bMoves ? FMath::RadiansToDegrees(FMath::Atan(0.988f * Curve)) : 0.f;
		Steer = FMath::Lerp(Steer, FMath::Clamp(Turning, -3.f, 3.f), 1.f - FMath::Exp(-Dt / 0.35f));
		// A surge tips the head back a little and easing off lets it drop, about half a degree
		// for each m/s gained a second, up to a degree and a half.
		const float Gain = LastSpeed >= 0.f && !bPaused ? (Speed - LastSpeed) / Dt : 0.f;
		LastSpeed = Speed;
		Surge = FMath::Lerp(Surge, bMoves ? FMath::Clamp(Gain * 0.5f, -1.5f, 1.5f) : 0.f, 1.f - FMath::Exp(-Dt / 0.5f));
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
	const FQuat Body = (Head + FRotator(Surge + Judder, Turn, Roll + Lean)).Quaternion();
	const ERideView Seen = bStarted && !bBackdrop ? View() : ERideView::Eyes;
	// Over the bars rides the Blender bike when it's imported, else make_bike.py's front end.
	const bool bParts = BikeParts.Num() > 0;
	Cockpit->SetVisibility(Seen == ERideView::Cockpit && !bParts);
	Bike->SetVisibility(Seen == ERideView::Chase || (Seen == ERideView::Cockpit && bParts), true);
	if (RiderBody)
	{
		// Over the bars the camera is in the rider's back: the body only throws its shadow there.
		RiderBody->SetVisibility(Seen == ERideView::Chase);
		RiderBody->SetCastHiddenShadow(Seen == ERideView::Cockpit && bParts);
	}
	if (RiderArms)
	{
		RiderArms->SetVisibility(Seen == ERideView::Cockpit && bParts && ArmsTilt >= 0.f);
	}
	if (Seen == ERideView::Cockpit && bParts)
	{
		// The bike's origin is the road under the eyes, and it moves with the body as the old front
		// end did, so the bars hold still in the frame while the look turns.
		const FQuat Look = FRotator(LookPitch + CockpitTilt, LookYaw, 0.f).Quaternion();
		Bike->SetWorldLocationAndRotation(Eye + Lift - Body.RotateVector(FVector(0.f, 0.f, EyeHeight)), Body);
		PoseBike();
		const FVector CameraAt = PartsOrigin + CockpitCamera - FVector(0.f, 0.f, EyeHeight);
		SetActorLocationAndRotation(Eye + Lift + Body.RotateVector(CameraAt), (Body * Look).Rotator());
		return;
	}
	if (Seen == ERideView::Cockpit)
	{
		// The camera back and down from the eyes, moving with the body; the front end stays put on
		// the body while the look turns, so it is placed relative to the camera by the look's inverse.
		const FQuat Look = FRotator(LookPitch + CockpitTilt, LookYaw, 0.f).Quaternion();
		SetActorLocationAndRotation(Eye + Lift + Body.RotateVector(CockpitEye), (Body * Look).Rotator());
		Cockpit->SetRelativeLocationAndRotation(Look.Inverse().RotateVector(-CockpitEye), Look.Inverse());
		return;
	}
	if (Seen != ERideView::Chase)
	{
		// The eyes look down the road as over the bars do, CockpitTilt below level, with no bike in
		// the frame; Dolly (ERideView::Steady) stays level (the user, 15 Nov).
		const float Tilt = Seen == ERideView::Eyes ? CockpitTilt : 0.f;
		SetActorLocationAndRotation(Eye + Lift, (Body * FRotator(LookPitch + Tilt, LookYaw, 0.f).Quaternion()).Rotator());
		return;
	}
	// Behind: the bike on the road under the rider, leaning into the bend and tipped with the grade,
	// and the camera back along the look, level, so the sway is the bike's and not the picture's.
	// Its roll, the lean and the rock together, is 80% of what it was before 15 Nov, at the user's request,
	// scaled by the rider's own setting (Rider: Bike sway) and apart from the head's.
	const float Tip = FMath::RadiansToDegrees(FMath::Atan2(On->Height(On->Wrap(S + 100.f)) - On->Height(On->Wrap(S - 100.f)), 200.f));
	Bike->SetWorldLocationAndRotation(On->Point(S, RideLine(*On, S), 0.f), FRotator(Tip, Head.Yaw, 0.8f * Profile.BikeSway / 100.f * (Lean * 3.f + Rock)));
	PoseBike();
	const FRotator Look(Head.Pitch * ChaseGrade + LookPitch + ChaseTilt, Head.Yaw + LookYaw, 0.f);
	const FVector Aim = Eye + Lift * 0.3f - FVector(0.f, 0.f, EyeHeight - ChaseAim);
	const FVector Back = Aim - Look.Vector() * ChaseBack;
	SetActorLocationAndRotation(Back, Look);
}

namespace
{
// The Blender bike's parts, as bike/README.md names them, and the pivots its moving ones turn
// about, in cm in the bike's frame: the ground under the bottom bracket, +X ahead, the drive side +Y.
enum EBikePart { BikeFrame, BikeFork, BikeWheelFront, BikeWheelRear, BikeCranks, BikePedalL, BikePedalR, BikeChain, PartCount };
const TCHAR* const BikePartNames[PartCount] = {
	TEXT("Frame"), TEXT("Fork"), TEXT("WheelFront"), TEXT("WheelRear"),
	TEXT("Cranks"), TEXT("PedalL"), TEXT("PedalR"), TEXT("Chain")};
const FVector FrontHub(58.63f, 0.f, 33.9f);
const FVector RearHub(-40.195f, 0.f, 33.9f);
const FVector BottomBracket(0.f, 0.f, 26.9f);
// The fork, bars and front wheel steer about the head tube's axis, through its centre
// (bike/README.md's Fork pivot and axis: 73 degrees from the ground, up and back).
const FVector HeadTube(41.193f, 0.f, 76.228f);
const FVector SteerAxis(-0.2924f, 0.f, 0.9563f);
constexpr float WheelRadius = 33.9f; // to the tyre's tread
constexpr float CrankLength = 17.25f; // to the pedal's axle
constexpr float RiderLoop = 2.f; // s: rider/README.md's Pedal, 60 frames at 30 fps a revolution
} // namespace

void ARidePawn::MakeBikeParts()
{
	TArray<UStaticMesh*> Meshes;
	for (const TCHAR* Name : BikePartNames)
	{
		UStaticMesh* Mesh = MeshAt(*FString::Printf(TEXT("/Game/Rider/Bike/SM_Bike_%s.SM_Bike_%s"), Name, Name));
		if (!Mesh)
		{
			return; // not imported yet: SM_Bike, if it's there
		}
		Meshes.Add(Mesh);
	}
	for (int32 Part = 0; Part < PartCount; ++Part)
	{
		UStaticMeshComponent* C = NewObject<UStaticMeshComponent>(this, *FString::Printf(TEXT("Bike%s"), BikePartNames[Part]));
		C->SetMobility(EComponentMobility::Movable);
		C->SetCollisionEnabled(ECollisionEnabled::NoCollision);
		C->SetStaticMesh(Meshes[Part]);
		C->SetupAttachment(Bike);
		C->SetRelativeLocation(PartsOrigin);
		C->SetVisibility(false);
		C->RegisterComponent();
		BikeParts.Add(C);
	}
	Bike->SetStaticMesh(nullptr);
	UE_LOG(LogTemp, Display, TEXT("RIDE bike: %d parts"), BikeParts.Num());
}

bool ARidePawn::HasBike() const
{
	return BikeParts.Num() == PartCount || (Bike && Bike->GetStaticMesh());
}

void ARidePawn::PoseBike()
{
	if (BikeParts.Num() != PartCount)
	{
		return;
	}
	// Each part's mesh is in the bike's frame, so turning it by R about a pivot P puts it at
	// P - R(P). A positive turn about +Y carries the top forward, as the wheels roll and the cranks
	// go round.
	const auto Turn = [this](int32 Part, const FVector& Pivot, float Angle)
	{
		const FQuat R(FVector::YAxisVector, Angle);
		BikeParts[Part]->SetRelativeLocationAndRotation(PartsOrigin + Pivot - R.RotateVector(Pivot), R);
	};
	const double Round = 2.0 * PI * WheelRadius;
	const float Spin = float(FMath::Fmod(Distance, Round) / WheelRadius);
	// Steered: the wheel spins about its hub, then turns with the fork about the head tube.
	const FQuat Steered(SteerAxis.GetSafeNormal(), FMath::DegreesToRadians(Steer));
	BikeParts[BikeFork]->SetRelativeLocationAndRotation(PartsOrigin + HeadTube - Steered.RotateVector(HeadTube), Steered);
	const FQuat Rolled(FVector::YAxisVector, Spin);
	BikeParts[BikeWheelFront]->SetRelativeLocationAndRotation(
		PartsOrigin + HeadTube + Steered.RotateVector(FrontHub - Rolled.RotateVector(FrontHub) - HeadTube), Steered * Rolled);
	Turn(BikeWheelRear, RearHub, Spin);
	Turn(BikeCranks, BottomBracket, Crank);
	// The pedals ride round on the cranks' ends but stay level, as a foot keeps them.
	const FQuat Pedal(FVector::YAxisVector, Crank);
	const FVector Arm(CrankLength, 0.f, 0.f);
	BikeParts[BikePedalR]->SetRelativeLocation(PartsOrigin + Pedal.RotateVector(Arm) - Arm);
	BikeParts[BikePedalL]->SetRelativeLocation(PartsOrigin + Pedal.RotateVector(-Arm) + Arm);
	if (RiderBody && RiderPedal)
	{
		// One revolution is frames 0-59 at 30 fps, frame 0 with the drive-side crank ahead as at
		// Crank 0, and turning the same way (rider/README.md). Past frame 59 it holds that frame.
		const float Turned = FMath::Fmod(Crank, 2.f * PI) / (2.f * PI);
		const float At = FMath::Min(Turned * RiderLoop, RiderPedal->GetPlayLength());
		for (USkeletalMeshComponent* C : {RiderBody.Get(), RiderArms.Get()})
		{
			if (URiderAnim* Anim = C ? Cast<URiderAnim>(C->GetAnimInstance()) : nullptr)
			{
				// Over the bars the arms hold one pose: the take's elbows swing with the stroke while the
				// bars, carried with the camera, stay put, and the hands read as shaking on them (15 Nov).
				const bool bHeld = C == RiderArms && bArmsCockpit;
				Anim->Time = bHeld ? 0.f : At;
				Anim->Effort = bHeld ? 0.f : RiderEffort; // Cockpit isn't blended
			}
		}
		if (RiderArms && bArmsCockpit && ArmsTilt == 0.f)
		{
			// The hands grip the drops, so they turn with the fork about the head tube, as the bars do.
			RiderArms->SetRelativeLocationAndRotation(PartsOrigin + HeadTube - Steered.RotateVector(HeadTube), Steered);
		}
		else if (RiderArms && RiderArms->IsVisible() && ArmsTilt != 0.f)
		{
			// Tipped back about the knuckles, which hold the hoods all round the loop; the turn's
			// top goes back, so it is negative in this frame.
			const FVector Grip = 0.5f * (RiderBody->GetBoneLocation(TEXT("middle_01_l"), EBoneSpaces::ComponentSpace) +
			                             RiderBody->GetBoneLocation(TEXT("middle_01_r"), EBoneSpaces::ComponentSpace));
			const FQuat R(FVector::YAxisVector, -FMath::DegreesToRadians(ArmsTilt));
			RiderArms->SetRelativeLocationAndRotation(PartsOrigin + Grip - R.RotateVector(Grip), R);
		}
	}
}

void ARidePawn::MakeRider()
{
	USkeletalMesh* Mesh = Cast<USkeletalMesh>(FSoftObjectPath(RiderMesh).TryLoad());
	TArray<UAnimSequence*> Takes;
	for (const TCHAR* Path : RiderTakes)
	{
		if (UAnimSequence* Take = Cast<UAnimSequence>(FSoftObjectPath(Path).TryLoad()))
		{
			Takes.Add(Take);
		}
	}
	UAnimSequence* Loop = Cast<UAnimSequence>(FSoftObjectPath(RiderTakes[1]).TryLoad());
	if (BikeParts.Num() != PartCount || !Mesh || !Loop)
	{
		return; // not imported yet, or no bike to sit on
	}
	// Both copies play the efforts through URiderAnim; PoseBike sets its time and effort.
	auto Animate = [](USkeletalMeshComponent* C, const TArray<UAnimSequence*>& Plays)
	{
		C->SetAnimationMode(EAnimationMode::AnimationBlueprint);
		C->SetAnimInstanceClass(URiderAnim::StaticClass());
		C->RegisterComponent();
		if (URiderAnim* Anim = Cast<URiderAnim>(C->GetAnimInstance()))
		{
			Anim->Takes = Plays;
		}
	};
	RiderBody = NewObject<USkeletalMeshComponent>(this, TEXT("RiderBody"));
	RiderBody->SetMobility(EComponentMobility::Movable);
	RiderBody->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	RiderBody->SetSkeletalMesh(Mesh);
	RiderBody->SetupAttachment(Bike);
	RiderBody->SetRelativeLocation(PartsOrigin); // its origin is the bike's: the ground under the BB
	RiderBody->SetVisibility(false);
	Animate(RiderBody, Takes);
	// Hidden over the bars, it still poses for its shadow and for RiderArms' grip.
	RiderBody->VisibilityBasedAnimTickOption = EVisibilityBasedAnimTickOption::AlwaysTickPoseAndRefreshBones;
	RiderPedal = Loop;
	RiderArms = NewObject<USkeletalMeshComponent>(this, TEXT("RiderArms"));
	RiderArms->SetMobility(EComponentMobility::Movable);
	RiderArms->SetCollisionEnabled(ECollisionEnabled::NoCollision);
	RiderArms->SetSkeletalMesh(Mesh);
	RiderArms->SetupAttachment(Bike);
	RiderArms->SetRelativeLocation(PartsOrigin);
	RiderArms->SetVisibility(false);
	RiderArms->SetCastShadow(false); // folded or tipped back, its shadow would be wrong: RiderBody casts
	UAnimSequence* Arms = Cast<UAnimSequence>(FSoftObjectPath(RiderCockpit).TryLoad());
	bArmsCockpit = Arms != nullptr;
	if (bArmsCockpit && !bArmsTiltGiven)
	{
		ArmsTilt = 0.f; // Cockpit's shoulders are already out of the frame
	}
	Animate(RiderArms, bArmsCockpit ? TArray<UAnimSequence*>{Arms} : Takes);
	for (const TCHAR* Bone : {TEXT("neck_01"), TEXT("thigh_l"), TEXT("thigh_r")})
	{
		RiderArms->HideBoneByName(Bone, EPhysBodyOp::PBO_None); // and all below it
	}
	UE_LOG(LogTemp, Display, TEXT("RIDE rider: %d bones, %d takes, Pedal %.3f s, arms %s, tilt %.0f"),
	       Mesh->GetRefSkeleton().GetNum(), Takes.Num(), Loop->GetPlayLength(),
	       bArmsCockpit ? TEXT("Cockpit") : TEXT("Pedal"), ArmsTilt);
	ApplyKit();
}

namespace
{
// The Kit page's studio, in cm: 3 km above the world's origin, clear of the land, the weather and
// every light in the level. Its meshes and lights share lighting channel 1 alone, so the sun (on 0)
// doesn't reach them, and the capture leaves out the sky, the fog and the sky's light.
const FVector StudioAt(0.f, 0.f, 300000.f);
// The turntable turns about the middle of the wheelbase, 9 cm ahead of the bottom bracket.
const FVector StudioOrigin(-9.f, 0.f, 0.f);
// The camera, on a long lens from the non-drive side at yaw 0, a little above the saddle and looking
// down at the bike's middle: side on, the bike and its margins fill the card's width.
const FVector StudioEye(0.f, -440.f, 175.f);
const FVector StudioAim(0.f, 0.f, 62.f);
constexpr float StudioFov = 28.f;
constexpr int32 StudioW = 780, StudioH = 1060; // 1.5x the card's 520 x 706
constexpr float StudioSpin = 24.f; // degrees a second: a turn in 15 s
} // namespace

const FSlateBrush* ARidePawn::KitPreview(float Radius) const
{
	StudioAsked = FPlatformTime::Seconds();
	if (!StudioTarget)
	{
		return nullptr;
	}
	StudioBrush.OutlineSettings.CornerRadii = FVector4(Radius, Radius, Radius, Radius);
	return &StudioBrush;
}

void ARidePawn::MakeKitPreview()
{
	const URiderAnim* Riding = RiderBody ? Cast<URiderAnim>(RiderBody->GetAnimInstance()) : nullptr;
	if (BikeParts.Num() != PartCount || !Riding)
	{
		return;
	}
	auto InStudio = [](UPrimitiveComponent* C)
	{
		C->SetMobility(EComponentMobility::Movable);
		C->SetCollisionEnabled(ECollisionEnabled::NoCollision);
		C->SetVisibleInSceneCaptureOnly(true);
		C->SetLightingChannels(false, true, false);
	};
	Studio = NewObject<USceneComponent>(this, TEXT("Studio"));
	Studio->SetMobility(EComponentMobility::Movable);
	Studio->SetupAttachment(GetRootComponent());
	Studio->SetUsingAbsoluteLocation(true);
	Studio->SetUsingAbsoluteRotation(true);
	Studio->SetUsingAbsoluteScale(true);
	Studio->RegisterComponent();
	Studio->SetWorldLocationAndRotation(StudioAt, FRotator::ZeroRotator);
	Turntable = NewObject<USceneComponent>(this, TEXT("Turntable"));
	Turntable->SetMobility(EComponentMobility::Movable);
	Turntable->SetupAttachment(Studio);
	Turntable->RegisterComponent();
	StudioCapture = NewObject<USceneCaptureComponent2D>(this, TEXT("StudioCapture"));
	for (int32 Part = 0; Part < PartCount; ++Part)
	{
		UStaticMeshComponent* C = NewObject<UStaticMeshComponent>(this, *FString::Printf(TEXT("Studio%s"), BikePartNames[Part]));
		InStudio(C);
		C->SetStaticMesh(BikeParts[Part]->GetStaticMesh());
		C->SetupAttachment(Turntable);
		C->SetRelativeLocation(StudioOrigin);
		C->RegisterComponent();
		StudioParts.Add(C);
		StudioCapture->ShowOnlyComponent(C);
	}
	StudioRider = NewObject<USkeletalMeshComponent>(this, TEXT("StudioRider"));
	InStudio(StudioRider);
	StudioRider->SetSkeletalMesh(RiderBody->GetSkeletalMeshAsset());
	StudioRider->SetupAttachment(Turntable);
	StudioRider->SetRelativeLocation(StudioOrigin);
	StudioRider->VisibilityBasedAnimTickOption = EVisibilityBasedAnimTickOption::AlwaysTickPoseAndRefreshBones;
	StudioRider->SetAnimationMode(EAnimationMode::AnimationBlueprint);
	StudioRider->SetAnimInstanceClass(URiderAnim::StaticClass());
	StudioRider->RegisterComponent();
	if (URiderAnim* Anim = Cast<URiderAnim>(StudioRider->GetAnimInstance()))
	{
		Anim->Takes = Riding->Takes;
		Anim->Effort = 1.f; // Pedal, the default
	}
	StudioCapture->ShowOnlyComponent(StudioRider);
	// The floor: a dark disc of the engine's plane under a pool of light, for the shadow to land on.
	if (UStaticMesh* Plane = MeshAt(TEXT("/Engine/BasicShapes/Plane.Plane")))
	{
		UStaticMeshComponent* Floor = NewObject<UStaticMeshComponent>(this, TEXT("StudioFloor"));
		InStudio(Floor);
		Floor->SetStaticMesh(Plane);
		Floor->SetupAttachment(Studio);
		Floor->SetRelativeScale3D(FVector(14.f, 14.f, 1.f));
		Floor->RegisterComponent();
		if (UMaterialInstanceDynamic* Mid = Floor->CreateAndSetMaterialInstanceDynamic(0))
		{
			Mid->SetVectorParameterValue(TEXT("Color"), FLinearColor(0.09f, 0.09f, 0.095f));
		}
		StudioCapture->ShowOnlyComponent(Floor);
	}
	// Three-point light, on channel 1 only: a warm key high on the camera's left, a cool rim from
	// behind for the outline, a soft fill from the right; and a pool from overhead for the floor.
	// Candelas, for the capture's fixed exposure (StudioCapture's look, below).
	auto Light = [this](ULocalLightComponent* L, const FVector& At, float Candelas, const FLinearColor& Colour, bool bShadow)
	{
		L->SetMobility(EComponentMobility::Movable);
		L->SetupAttachment(Studio);
		L->SetRelativeLocationAndRotation(At, (StudioAim - At).Rotation());
		L->SetIntensityUnits(ELightUnits::Candelas);
		L->SetIntensity(Candelas);
		L->SetLightColor(Colour);
		L->SetAttenuationRadius(1200.f);
		L->SetCastShadows(bShadow);
		L->SetLightingChannels(false, true, false);
		L->SetVisibility(false);
		L->RegisterComponent();
		StudioLights.Add(L);
	};
	auto Spot = [this](const TCHAR* Name, float Inner, float Outer)
	{
		USpotLightComponent* L = NewObject<USpotLightComponent>(this, Name);
		L->SetInnerConeAngle(Inner);
		L->SetOuterConeAngle(Outer);
		L->SetSourceRadius(30.f);
		L->SetSoftSourceRadius(60.f);
		return L;
	};
	Light(Spot(TEXT("StudioKey"), 20.f, 40.f), FVector(-260.f, -300.f, 330.f), 45.f, FLinearColor(1.f, 0.95f, 0.88f), true);
	Light(Spot(TEXT("StudioRim"), 20.f, 40.f), FVector(240.f, 300.f, 270.f), 70.f, FLinearColor(0.82f, 0.9f, 1.f), false);
	Light(Spot(TEXT("StudioPool"), 18.f, 46.f), FVector(0.f, 0.f, 430.f), 40.f, FLinearColor(1.f, 0.97f, 0.93f), true);
	UPointLightComponent* Fill = NewObject<UPointLightComponent>(this, TEXT("StudioFill"));
	Fill->SetSourceRadius(60.f);
	Light(Fill, FVector(300.f, -360.f, 110.f), 8.f, FLinearColor(0.92f, 0.95f, 1.f), false);

	StudioTarget = NewObject<UTextureRenderTarget2D>(this, TEXT("StudioTarget"));
	StudioTarget->RenderTargetFormat = RTF_RGBA8; // sRGB, for the capture's tone-mapped colour
	StudioTarget->ClearColor = FLinearColor::Black;
	StudioTarget->InitAutoFormat(StudioW, StudioH);
	StudioTarget->UpdateResourceImmediate(true);
	StudioCapture->SetupAttachment(Studio);
	StudioCapture->SetRelativeLocationAndRotation(StudioEye, (StudioAim - StudioEye).Rotation());
	StudioCapture->FOVAngle = StudioFov;
	StudioCapture->TextureTarget = StudioTarget;
	StudioCapture->CaptureSource = ESceneCaptureSource::SCS_FinalColorLDR;
	StudioCapture->PrimitiveRenderMode = ESceneCapturePrimitiveRenderMode::PRM_UseShowOnlyList;
	StudioCapture->bCaptureEveryFrame = false;
	StudioCapture->bCaptureOnMovement = false;
	StudioCapture->bAlwaysPersistRenderingState = true; // keeps the temporal AA's history between frames
	StudioLook(0);
	StudioCapture->PostProcessBlendWeight = 1.f;
	StudioCapture->RegisterComponent();

	StudioBrush.SetResourceObject(StudioTarget);
	StudioBrush.ImageSize = FVector2D(StudioW, StudioH);
	StudioBrush.DrawAs = ESlateBrushDrawType::RoundedBox;
	StudioBrush.OutlineSettings.RoundingType = ESlateBrushRoundingType::FixedRadius;
	StudioBrush.OutlineSettings.Width = 0.f;
	StudioBrush.TintColor = FSlateColor(FLinearColor::White);
	ApplyKit();
	UE_LOG(LogTemp, Display, TEXT("RIDE kit preview: %d parts, %d lights, %dx%d"), StudioParts.Num(), StudioLights.Num(), StudioW, StudioH);
}

// The capture's show flags and grade. 0 is the Kit page's; -RideKitDump steps through the others
// to compare: 1 the world's own (its volume's auto exposure), 2 the game's grade on a histogram
// between EV100 -2 and 6 with no bloom, 3 that grade at a fixed exposure with eye adaptation on.
void ARidePawn::StudioLook(int32 Variant)
{
	FEngineShowFlags& Show = StudioCapture->ShowFlags;
	Show = FEngineShowFlags(ESFIM_Game);
	Show.SetFog(false);
	Show.SetVolumetricFog(false);
	Show.SetAtmosphere(false);
	Show.SetCloud(false);
	Show.SetMotionBlur(false);
	// The engine's own exposure, lifted: the ride's grade (CameraLook) blew the card out to white
	// at every exposure tried, and the plain look came out a stop or three dark (v65's dumps,
	// 15 Nov). Variants 1..3 are the brighter and darker lifts -RideKitDump compares.
	static const float Lifts[] = {1.5f, 0.5f, 2.5f, 3.5f};
	FPostProcessSettings& Look = StudioCapture->PostProcessSettings;
	Look = FPostProcessSettings();
	Look.bOverride_VignetteIntensity = true;
	Look.VignetteIntensity = 0.f;
	Look.bOverride_AutoExposureBias = true;
	Look.AutoExposureBias = Lifts[FMath::Clamp(Variant, 0, 3)];
}

void ARidePawn::TurnKitPreview(float Dt)
{
	// -RideKitDump runs the studio from the start, menu or not, to dump its picture.
	static const bool bDumping = FString(FCommandLine::Get()).Contains(TEXT("RideKitDump="));
	const bool bWant = bDumping || (StudioAsked >= 0.0 && FPlatformTime::Seconds() - StudioAsked < 0.25);
	if (bWant && !Studio)
	{
		MakeKitPreview();
	}
	if (!Studio || !StudioCapture)
	{
		return;
	}
	if (bWant != bStudioOn)
	{
		bStudioOn = bWant;
		StudioCapture->bCaptureEveryFrame = bWant;
		for (ULocalLightComponent* L : StudioLights)
		{
			L->SetVisibility(bWant);
		}
		StudioRider->VisibilityBasedAnimTickOption = bWant ? EVisibilityBasedAnimTickOption::AlwaysTickPoseAndRefreshBones
		                                                   : EVisibilityBasedAnimTickOption::OnlyTickPoseWhenRendered;
	}
	if (!bWant)
	{
		return;
	}
	// -RideKitDump=<file.png>: the capture's own picture in each StudioLook, two seconds apart, for checking its
	// light and exposure apart from the menu's drawing of it.
	StudioRunFor += Dt;
	FString Dump;
	if (StudioDumps < 4 && StudioRunFor > 2.f * (StudioDumps + 1) && FParse::Value(FCommandLine::Get(), TEXT("RideKitDump="), Dump))
	{
		Dump = FPaths::GetBaseFilename(Dump, false) + FString::Printf(TEXT("_%d.png"), StudioDumps);
		FImage Picture;
		if (FImageUtils::GetRenderTargetImage(StudioTarget, Picture))
		{
			FLinearColor Sum(0.f, 0.f, 0.f, 0.f);
			const TArrayView64<FColor> Pixels = Picture.AsBGRA8();
			for (const FColor& Px : Pixels)
			{
				Sum += FLinearColor(Px.R, Px.G, Px.B, Px.A) / 255.f;
			}
			Sum /= FMath::Max<int64>(Pixels.Num(), 1);
			const bool bSaved = FImageUtils::SaveImageByExtension(*Dump, Picture);
			UE_LOG(LogTemp, Display, TEXT("RIDE kit dump %d: mean %.3f %.3f %.3f alpha %.3f, %s %s"), StudioDumps, Sum.R, Sum.G, Sum.B, Sum.A,
			       bSaved ? TEXT("saved") : TEXT("not saved"), *Dump);
		}
		else
		{
			UE_LOG(LogTemp, Display, TEXT("RIDE kit dump: no picture (resource %d)"), StudioTarget->GetResource() != nullptr);
		}
		if (++StudioDumps < 4)
		{
			StudioLook(StudioDumps);
		}
	}
	// A slow turn, and an easy 60 rpm: the rider's loop and the cranks in step, as PoseBike keeps them.
	StudioTurn = FMath::Fmod(StudioTurn + StudioSpin * Dt, 360.f);
	Turntable->SetRelativeRotation(FRotator(0.f, StudioTurn, 0.f));
	StudioCrank = FMath::Fmod(StudioCrank + 2.f * PI * Dt, 2.f * PI);
	if (URiderAnim* Anim = Cast<URiderAnim>(StudioRider->GetAnimInstance()))
	{
		Anim->Time = FMath::Min(StudioCrank / (2.f * PI) * RiderLoop, RiderPedal ? RiderPedal->GetPlayLength() : RiderLoop);
		Anim->Effort = 1.f;
	}
	const auto Turn = [this](int32 Part, const FVector& Pivot, float Angle)
	{
		const FQuat R(FVector::YAxisVector, Angle);
		StudioParts[Part]->SetRelativeLocationAndRotation(StudioOrigin + Pivot - R.RotateVector(Pivot), R);
	};
	Turn(BikeCranks, BottomBracket, StudioCrank);
	Turn(BikeWheelRear, RearHub, StudioCrank * 2.6f); // 50 x 19: the wheel turns with the cranks
	const FQuat Pedal(FVector::YAxisVector, StudioCrank);
	const FVector Arm(CrankLength, 0.f, 0.f);
	StudioParts[BikePedalR]->SetRelativeLocation(StudioOrigin + Pedal.RotateVector(Arm) - Arm);
	StudioParts[BikePedalL]->SetRelativeLocation(StudioOrigin + Pedal.RotateVector(-Arm) + Arm);
}

void ARidePawn::PlaceRain(float Dt)
{
	// Riding into rain, the drops come at the rider: they fall at about 7 m/s, so from the bike they
	// slant in from ahead by atan(speed / 7), 40 degrees at 21 km/h. Eased so a surge doesn't twitch
	// them; held at 55 degrees (35 km/h), past which the shells no longer cover the view.
	const float Want = bStarted && !bOnBackdrop ? FMath::RadiansToDegrees(FMath::Atan(Speed / 7.f)) : 0.f;
	RainTilt = Dt > 0.f ? FMath::FInterpTo(RainTilt, FMath::Min(Want, 55.f), Dt, 2.f) : 0.f;
	if (!Rain->IsVisible())
	{
		return;
	}
	// The streaks fall down the shells, so tipping their tops forward along the road slants the rain
	// at the rider (and a little crosswind from the left). The shells stretch along their length by
	// 1 / cos(tilt), the drops' speed past the rider, so the streaks lengthen as it rises; and they
	// slide forward up their axis 10 sin(tilt) m, or the 14 m shell's top rim (15 m up) would come
	// into view ahead (a sweep of the view's rays, 0-55 degrees).
	const float T = FMath::DegreesToRadians(RainTilt);
	const FRotator Slant(-RainTilt, Head.Yaw, 4.f);
	Rain->SetWorldLocationAndRotation(Camera->GetComponentLocation() + Slant.RotateVector(FVector(0.f, 0.f, 1000.f * FMath::Sin(T))), Slant);
	Rain->SetWorldScale3D(FVector(1.f, 1.f, 1.f / FMath::Cos(T)));
}

bool ARidePawn::Moves() const
{
	// Tests and stills ride steady, so they compare frame to frame, unless they ask for the motion.
	return Automated() ? bSway : View() != ERideView::Steady;
}

float ARidePawn::RideLine(const URideTrack& On, float S) const
{
	// A slow, uneven wander across the lane over 20-50 m, mostly within 5-8 cm and about 15 cm at
	// most; tied to the distance, so it stops when the bike does. Two sines up to 34 cm read as a
	// weave (the user, 14 Nov). Plus the inside of the bend (Apex). Steady eases back to KeepLeft.
	const float Drift = Motion * (12.f * FMath::PerlinNoise1D(S / 5200.f + 3.f) + 6.f * FMath::PerlinNoise1D(S / 1900.f + 11.f));
	return -On.KeepLeft + Drift + Apex;
}

TArray<ERideView> ARidePawn::Views() const
{
	TArray<ERideView> All;
	if (BikeParts.Num() > 0 || (Cockpit && Cockpit->GetStaticMesh()))
	{
		All.Add(ERideView::Cockpit);
	}
	All.Add(ERideView::Eyes);
	All.Add(ERideView::Steady);
	if (HasBike())
	{
		All.Add(ERideView::Chase);
	}
	return All;
}

ERideView ARidePawn::View() const
{
	const ERideView Wanted = static_cast<ERideView>(FMath::Clamp(Profile.View, 0, 3));
	if (Automated())
	{
		const ERideView Shot = static_cast<ERideView>(FMath::Clamp(ShotView, 0, 3));
		return Views().Contains(Shot) ? Shot : ERideView::Eyes;
	}
	return Views().Contains(Wanted) ? Wanted : ERideView::Eyes;
}

void ARidePawn::CycleView()
{
	const TArray<ERideView> All = Views();
	const int32 Next = (All.IndexOfByKey(View()) + 1) % All.Num();
	Profile.View = static_cast<int32>(All[Next]);
	Profile.Save();
	UseGrassReach(); // the chase view reaches further
}

void ARidePawn::SetFreeLook(bool bOn)
{
	bFreeLook = bOn;
	bPressOnDock = false;
	DockHover = -1;
	if (bOn)
	{
		Dock.Hide();
	}
	else
	{
		Dock.Moved();
	}
	if (ARidePlayerController* Rides = GetController<ARidePlayerController>())
	{
		Rides->RideInput();
	}
}

void ARidePawn::PressDock(Veloverse::EDock Button)
{
	using Veloverse::EDock;
	ARidePlayerController* Rides = GetController<ARidePlayerController>();
	switch (Button)
	{
	case EDock::View: CycleView(); break;
	case EDock::Graphs:
		Profile.bGraphs = !Profile.bGraphs;
		Profile.Save();
		break;
	case EDock::Hud:
		Profile.bHud = !Profile.bHud;
		Profile.Save();
		break;
	case EDock::Route:
		if (Rides)
		{
			Rides->ShowRoutes();
		}
		break;
	case EDock::Look: SetFreeLook(true); break;
	case EDock::Pause:
		if (Rides)
		{
			Rides->ShowPause();
		}
		break;
	default: break;
	}
}

void ARidePawn::NextWeather()
{
	// The HUD's weather pill steps through Settings > World's choices (the user, 15 Nov): the
	// route's own, then clear, overcast and rain, and round to the route's own again.
	Profile.Weather = (Profile.Weather + 1) % 4;
	Profile.Save();
	ApplyWeather();
}

void ARidePawn::Docking(APlayerController& Player)
{
	using Veloverse::EDock;
	// The keys work with the dock up or away, and in free look.
	if (Player.WasInputKeyJustPressed(EKeys::Tab))
	{
		SetFreeLook(!bFreeLook);
		return;
	}
	const TPair<FKey, EDock> Keys[] = {{EKeys::V, EDock::View}, {EKeys::G, EDock::Graphs}, {EKeys::H, EDock::Hud},
		{EKeys::R, EDock::Route}};
	for (const TPair<FKey, EDock>& Key : Keys)
	{
		if (Player.WasInputKeyJustPressed(Key.Key))
		{
			PressDock(Key.Value);
			return;
		}
	}
	const float Dt = GetWorld() ? GetWorld()->GetDeltaSeconds() : 0.f;
	if (bFreeLook)
	{
		DockHover = -1;
		Dock.Tick(Dt, false);
		return;
	}
	// The pointer in the HUD's frame: 720 tall, as wide as the screen's shape makes it.
	FVector2D Size(0.0, 0.0);
	if (GEngine && GEngine->GameViewport)
	{
		GEngine->GameViewport->GetViewportSize(Size);
	}
	float Mx = 0.f, My = 0.f;
	const bool bOnScreen = Size.Y > 0.0 && Player.GetMousePosition(Mx, My);
	const float K = bOnScreen ? 720.f / static_cast<float>(Size.Y) : 0.f;
	const FVector2f Now(Mx * K, My * K);
	if (bOnScreen && MouseAt.X >= 0.f && FVector2f::Distance(Now, MouseAt) > 0.5f)
	{
		Dock.Moved();
	}
	MouseAt = bOnScreen ? Now : FVector2f(-1.f, -1.f);
	const Veloverse::FDockLayout L =
		Veloverse::LayoutDock(static_cast<float>(Size.X) * K, 720.f, Dock.Shown);
	DockHover = bOnScreen ? L.Hit(Now.X, Now.Y) : -1;
	bWeatherHover = bOnScreen && DockHover < 0 && Profile.bHud && WeatherPill.Contains(Now.X, Now.Y);
	const bool bHeld = Player.IsInputKeyDown(EKeys::LeftMouseButton) || Player.IsInputKeyDown(EKeys::RightMouseButton);
	if (Player.WasInputKeyJustPressed(EKeys::LeftMouseButton) || Player.WasInputKeyJustPressed(EKeys::RightMouseButton))
	{
		bPressOnDock = DockHover >= 0 || bWeatherHover;
		if (bPressOnDock && Player.WasInputKeyJustPressed(EKeys::LeftMouseButton))
		{
			if (DockHover >= 0)
			{
				PressDock(static_cast<EDock>(DockHover));
			}
			else
			{
				NextWeather();
			}
		}
	}
	else if (!bHeld)
	{
		bPressOnDock = false;
	}
	Dock.Tick(Dt, DockHover >= 0 || bPressOnDock);
	// The cursor goes with the dock, and while a drag turns the head.
	const bool bDragging = bHeld && !bPressOnDock;
	Player.CurrentMouseCursor = Dock.Visible() && !bDragging ? EMouseCursor::Default : EMouseCursor::None;
}

bool ARidePawn::Backdrop(const URideTrack*& On, double& At) const
{
	// The design board's backdrop: the pine valley near the top of Vael, where its camera stood
	// (the world's "ride_02_vael" view, in metres), on whichever route passes within 50 m of it.
	// -RideBackdropIn=NAME and -RideBackdrop=METRES look from a way into a named stretch instead.
	TArray<const URideTrack*> Order = {Track.Get()};
	for (const URideTrack* Each : Routes)
	{
		Order.AddUnique(Each);
	}
	FString In;
	float Into = 0.f;
	FParse::Value(FCommandLine::Get(), TEXT("RideBackdrop="), Into);
	if (!FParse::Value(FCommandLine::Get(), TEXT("RideBackdropIn="), In))
	{
		const FVector Where(BoardEye.X, BoardEye.Y, 0.0);
		for (const URideTrack* Each : Order)
		{
			if (!Each || Each->Length() <= 0.f)
			{
				continue;
			}
			const float S = Each->Nearest(Where);
			if (FVector::Dist2D(Each->Point(S, 0.f), Where) < 5000.0)
			{
				On = Each;
				At = S + Into * 100.0;
				return true;
			}
		}
		return false;
	}
	for (const URideTrack* Each : Order)
	{
		for (const FRideStretch& Stretch : Each ? Each->Stretches : TArray<FRideStretch>())
		{
			if (Stretch.Name.Equals(In, ESearchCase::IgnoreCase))
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
	for (TActorIterator<ASkyAtmosphere> It(GetWorld()); It && !Atmosphere.IsValid(); ++It)
	{
		Atmosphere = It->GetComponent();
		if (Atmosphere.IsValid())
		{
			Rayleigh = Atmosphere->RayleighScatteringScale;
			Mie = Atmosphere->MieScatteringScale;
			Absorption = Atmosphere->OtherAbsorptionScale;
			UE_LOG(LogTemp, Display, TEXT("WEATHER atmosphere rayleigh %.4f mie %.4f"), Rayleigh, Mie);
		}
	}
	for (TActorIterator<APostProcessVolume> It(GetWorld()); It && !Post.IsValid(); ++It)
	{
		Post = *It;
	}
}

void ARidePawn::ApplyWeather()
{
	// The automated stills keep the level's light, which is what they review, unless their profile
	// names a time of day: then they ride the rider's sky, so QA can see the evening a rider picks.
	const bool bQaSky = Automated() && Profile.TimeOfDay == 0;
	const bool bOwn = bQaSky || (Profile.Weather == 0 && Profile.TimeOfDay == 0);
	// A route with no weather rows rides its default row, a clear day, as the pill says: the level's
	// own light is flat and pale in the ride (out/qa/own1 against clear4).
	if (!Track || (Track->Weather.Num() == 0 && bQaSky))
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
		// Cloud hides the sun's disc and spreads its light: widen the source as the light falls
		// below a clear day's, so a rain sky's shadows go soft and faint instead of sharp.
		if (SunAngle < 0.f)
		{
			SunAngle = Sun->LightSourceAngle;
			UE_LOG(LogTemp, Display, TEXT("WEATHER first apply: sun %s lux %.0f angle %.2f sky %.2f fog %.3f rain %.1f"),
				*Sun->GetName(), W.Lux, SunAngle, W.Sky, W.Fog, W.Rain);
		}
		const float Cloud = FMath::Clamp(1.f - W.Lux / 70000.f, 0.f, 1.f);
		Sun->SetLightSourceAngle(FMath::Lerp(SunAngle, 12.f, FMath::Pow(Cloud, 1.5f)));
		// Turned only on a real change: a sun that turns at all is a moving light to the virtual
		// shadow maps, which then redraw their pages each frame at the moving light's coarser
		// resolution. The route's rows ease the sun's height along it; steps of a quarter degree
		// shift a bike's shadow by millimetres.
		const FRotator Aim(-W.Elevation, Azimuth, 0.f);
		if (!Aim.Equals(SunAim, 0.25f))
		{
			Sun->GetOwner()->SetActorRotation(Aim);
			SunAim = Aim;
			UE_LOG(LogTemp, Display, TEXT("RIDE sun turned to %.1f high, %.0f round"), W.Elevation, Azimuth);
		}
		// An overcast sky is grey-white to the zenith, not blue: the ride frames only the sky near
		// the horizon, where the cloud layer never showed (out/qa/cloudy5, package 0454), so thin
		// the atmosphere's blue scattering and thicken its haze instead.
		if (Atmosphere.IsValid())
		{
			const float Thick = FMath::Pow(Cloud, 1.5f);
			// The ozone's absorption stays with the blue, or the thinned sky goes lilac (out/qa/cloudy6).
			// Only on a change: this runs five times a second and each set re-sends the atmosphere.
			const float R = Rayleigh * FMath::Lerp(1.f, 0.05f, Thick); // 0.15 left a violet cast (out/qa/cloudy7)
			if (!FMath::IsNearlyEqual(R, Atmosphere->RayleighScatteringScale, 1e-5f))
			{
				Atmosphere->SetRayleighScatteringScale(R);
				Atmosphere->SetMieScatteringScale(Mie * FMath::Lerp(1.f, 2.5f, Thick));
				Atmosphere->SetOtherAbsorptionScale(Absorption * FMath::Lerp(1.f, 0.05f, Thick));
			}
		}
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
