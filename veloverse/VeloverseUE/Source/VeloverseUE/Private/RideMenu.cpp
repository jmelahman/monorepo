// The menu's state and input, and the numbers its pages draw: each route's survey, the island map,
// the stats from the saved rides, and a ride's review. RideMenuPages.cpp paints the pages.
#include "RideMenu.h"

#include "Blueprint/WidgetTree.h"
#include "Components/BackgroundBlur.h"
#include "Dom/JsonObject.h"
#include "Engine/Texture2D.h"
#include "Framework/Application/SlateApplication.h"
#include "HAL/FileManager.h"
#include "HAL/PlatformProcess.h"
#include "ImageUtils.h"
#include "Misc/ConfigCacheIni.h"
#include "Misc/FileHelper.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "RideMenuStyle.h"
#include "RidePawn.h"
#include "RidePlayerController.h"
#include "RideTrack.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"

using namespace VeloverseMenu;

bool URideMenu::bRoutesNext = false;

// ---- Lifecycle -----------------------------------------------------------------------------------

void URideMenu::NativeOnInitialized()
{
	Super::NativeOnInitialized();
	SetIsFocusable(true);
	SetVisibility(ESlateVisibility::Visible); // the clicks land on the menu itself
	if (!GConfig->GetString(TEXT("/Script/EngineSettings.GeneralProjectSettings"), TEXT("ProjectVersion"), Version,
	                        GGameIni) ||
	    Version.IsEmpty())
	{
		Version = TEXT("dev");
	}
	ReadMap();
}

TSharedRef<SWidget> URideMenu::RebuildWidget()
{
	// The world blurred behind the pages: the only widget, so NativePaint draws everything else.
	if (WidgetTree && !WidgetTree->RootWidget)
	{
		Blur = WidgetTree->ConstructWidget<UBackgroundBlur>(UBackgroundBlur::StaticClass(), TEXT("Blur"));
		Blur->SetBlurStrength(0.f);
		WidgetTree->RootWidget = Blur;
	}
	return Super::RebuildWidget();
}

ARidePlayerController* URideMenu::Controller() const { return Cast<ARidePlayerController>(GetOwningPlayer()); }

ARidePawn* URideMenu::Rider() const
{
	const ARidePlayerController* Player = Controller();
	return Player ? Player->Rider() : nullptr;
}

void URideMenu::NativeTick(const FGeometry& Geometry, float DeltaTime)
{
	Super::NativeTick(Geometry, DeltaTime);
	RefreshClock += DeltaTime;
	if (RefreshClock < 0.25f)
	{
		return;
	}
	RefreshClock = 0.f;
	// The menu can open before the rider has read its routes and trainer, so it keeps catching up.
	if (ARidePawn* Pawn = Rider())
	{
		if (Pawn->Ftp != WorkoutsFtp)
		{
			ReadWorkouts(Pawn->Ftp);
		}
		if (Surveys.Num() != Pawn->Routes.Num() && (Current == ERideMenuPage::Start || Current == ERideMenuPage::Routes))
		{
			SurveyRoutes();
		}
	}
}

void URideMenu::Show(ERideMenuPage Page)
{
	ARidePawn* Pawn = Rider();
	// A first run asks for the rider's FTP and weight before anything else, once.
	if (Page == ERideMenuPage::Start && !bWelcome && !FRideProfile::Saved())
	{
		bWelcome = true;
		Page = ERideMenuPage::Settings;
		Tab = ERideSettingsTab::Rider;
		UE_LOG(LogTemp, Display, TEXT("RIDE first run: the rider's settings first"));
	}
	// "Ride another route" reloads the level; its start screen opens on the routes.
	if (Page == ERideMenuPage::Start && bRoutesNext)
	{
		bRoutesNext = false;
		Page = ERideMenuPage::Routes;
	}
	if ((Page == ERideMenuPage::Settings || Page == ERideMenuPage::Sensors) && Page != Current &&
	    Current != ERideMenuPage::Settings && Current != ERideMenuPage::Sensors)
	{
		Back = Current;
	}
	if (Page == ERideMenuPage::Summary && Current != ERideMenuPage::Summary && Pawn && Pawn->bStarted)
	{
		ReviewJustRidden(); // a ride's own end; reviews from the stats are loaded before they show
	}
	if (Page == ERideMenuPage::Stats || Page == ERideMenuPage::Start)
	{
		ReadHistory();
		SurveyRoutes();
		WorkOutStats();
	}
	if (Page == ERideMenuPage::Routes)
	{
		SurveyRoutes();
	}
	Current = Page;
	Hover.Reset();
	Dragging.Reset();
	Focus = DefaultFocus();
	TakeWidget(); // builds the blur on first use
	if (Blur)
	{
		const float Strength = Page == ERideMenuPage::Start    ? 0.f
		                       : Page == ERideMenuPage::Routes ? 6.f
		                       : Page == ERideMenuPage::Pause  ? 8.f
		                                                       : 18.f;
		Blur->SetBlurStrength(Strength);
	}
	Invalidate(EInvalidateWidgetReason::Paint);
}

void URideMenu::SetTab(ERideSettingsTab Now)
{
	// A first run's profile is kept as the rider leaves its tab, so the welcome shows once.
	if (ARidePawn* Pawn = Rider(); Pawn && Tab == ERideSettingsTab::Rider && Now != Tab && !FRideProfile::Saved())
	{
		Pawn->SetProfile(Pawn->Profile);
	}
	Tab = Now;
	Focus = DefaultFocus();
}

// The keys' first stop on each page.
FString URideMenu::DefaultFocus() const
{
	switch (Current)
	{
	case ERideMenuPage::Start: return TEXT("ride");
	case ERideMenuPage::Routes: return TEXT("start");
	case ERideMenuPage::Sensors: return TEXT("routes");
	case ERideMenuPage::Settings:
		return Tab == ERideSettingsTab::Graphics ? TEXT("tab0") : Tab == ERideSettingsTab::Detail ? TEXT("tab1") : TEXT("tab2");
	case ERideMenuPage::Stats: return TEXT("back");
	case ERideMenuPage::Pause:
	{
		const ARidePawn* Pawn = Rider();
		return Pawn && Pawn->bFinished ? TEXT("review") : TEXT("resume");
	}
	case ERideMenuPage::Summary: return TEXT("home");
	}
	return FString();
}

void URideMenu::GoBack()
{
	switch (Current)
	{
	case ERideMenuPage::Start: break;
	case ERideMenuPage::Routes:
	case ERideMenuPage::Stats: Show(ERideMenuPage::Start); break;
	case ERideMenuPage::Sensors: Show(Back); break;
	case ERideMenuPage::Settings:
		if (Tab == ERideSettingsTab::Rider)
		{
			LeaveRider();
		}
		else
		{
			Show(Back);
		}
		break;
	case ERideMenuPage::Pause:
		if (const ARidePawn* Pawn = Rider(); Pawn && !Pawn->bFinished)
		{
			Controller()->Resume();
		}
		break;
	case ERideMenuPage::Summary:
		if (bFromStats)
		{
			Show(ERideMenuPage::Stats);
		}
		else if (const ARidePawn* Pawn = Rider(); Pawn && Pawn->bStarted)
		{
			Controller()->Restart();
		}
		else
		{
			Show(ERideMenuPage::Start);
		}
		break;
	}
}

// Leaving the rider's settings: a first run's profile is kept as it stands, so the welcome shows once.
void URideMenu::LeaveRider()
{
	if (ARidePawn* Pawn = Rider(); Pawn && !FRideProfile::Saved())
	{
		Pawn->SetProfile(Pawn->Profile);
	}
	Show(Back);
}

void URideMenu::ReadWorkouts(float RiderFtp)
{
	WorkoutsFtp = RiderFtp;
	Workouts = ARidePawn::WorkoutLibrary(RiderFtp);
}

// Steps the start page's workout through none and the library.
void URideMenu::StepWorkout(int32 Step)
{
	ARidePawn* Pawn = Rider();
	if (!Pawn)
	{
		return;
	}
	int32 At = -1;
	if (Pawn->Workout.Active())
	{
		At = Workouts.IndexOfByPredicate([&](const Veloverse::FWorkout& W)
		                                 { return W.Name == Pawn->Workout.Workout().Name; });
	}
	const int32 Count = Workouts.Num() + 1;
	const int32 Next = ((At + 1 + Step) % Count + Count) % Count - 1;
	if (Next < 0)
	{
		Pawn->EndWorkout();
	}
	else
	{
		Pawn->StartWorkout(Workouts[Next]);
	}
}

// ---- Input ---------------------------------------------------------------------------------------

const URideMenu::FHit* URideMenu::Find(const FString& Key) const
{
	return Hits.FindByPredicate([&](const FHit& H) { return H.Key == Key; });
}

FVector2f URideMenu::ToFrame(const FGeometry& Geometry, FVector2D Screen) const
{
	return FVector2f(Geometry.AbsoluteToLocal(Screen)) / FMath::Max(Scale, 0.01f);
}

void URideMenu::Press(const FString& Key)
{
	if (const FHit* H = Find(Key); H && H->OnClick)
	{
		const TFunction<void()> Click = H->OnClick; // the hits are rebuilt if the click changes the page
		Click();
		Invalidate(EInvalidateWidgetReason::Paint);
	}
}

// The nearest hit that way: straight on counts for more than off to the side.
void URideMenu::MoveFocus(FVector2f Direction)
{
	const FHit* From = Find(Focus);
	if (!From)
	{
		Focus = DefaultFocus();
		if (!Find(Focus) && Hits.Num() > 0)
		{
			Focus = Hits[0].Key;
		}
		return;
	}
	const FVector2f Centre(From->X + From->W * 0.5f, From->Y + From->H * 0.5f);
	const FHit* Best = nullptr;
	float BestScore = TNumericLimits<float>::Max();
	for (const FHit& H : Hits)
	{
		if (H.Key == From->Key || (!H.OnClick && !H.OnDrag))
		{
			continue;
		}
		// The gap between the boxes along the way, and the offset across it.
		const float Ahead = Direction.X != 0.f
			? (Direction.X > 0.f ? H.X - (From->X + From->W) : From->X - (H.X + H.W))
			: (Direction.Y > 0.f ? H.Y - (From->Y + From->H) : From->Y - (H.Y + H.H));
		if (Ahead < -FMath::Min(From->W, From->H) * 0.5f)
		{
			continue;
		}
		const FVector2f Other(H.X + H.W * 0.5f, H.Y + H.H * 0.5f);
		const float Across = Direction.X != 0.f ? FMath::Abs(Other.Y - Centre.Y) : FMath::Abs(Other.X - Centre.X);
		const float Score = FMath::Max(Ahead, 0.f) + 2.f * Across;
		if (Score < BestScore)
		{
			BestScore = Score;
			Best = &H;
		}
	}
	if (Best)
	{
		Focus = Best->Key;
	}
}

FReply URideMenu::NativeOnKeyDown(const FGeometry& Geometry, const FKeyEvent& Event)
{
	const FKey Key = Event.GetKey();
	if (Key == EKeys::Escape || Key == EKeys::P || Key == EKeys::Gamepad_Special_Right ||
	    Key == EKeys::Gamepad_FaceButton_Right || Key == EKeys::BackSpace)
	{
		GoBack();
		return FReply::Handled();
	}
	const bool bUp = Key == EKeys::Up || Key == EKeys::Gamepad_DPad_Up || Key == EKeys::Gamepad_LeftStick_Up;
	const bool bDown = Key == EKeys::Down || Key == EKeys::Gamepad_DPad_Down || Key == EKeys::Gamepad_LeftStick_Down;
	const bool bLeft = Key == EKeys::Left || Key == EKeys::Gamepad_DPad_Left || Key == EKeys::Gamepad_LeftStick_Left;
	const bool bRight =
		Key == EKeys::Right || Key == EKeys::Gamepad_DPad_Right || Key == EKeys::Gamepad_LeftStick_Right;
	const bool bPress = Key == EKeys::Enter || Key == EKeys::SpaceBar || Key == EKeys::Gamepad_FaceButton_Bottom;
	if (!bUp && !bDown && !bLeft && !bRight && !bPress && Key != EKeys::Tab)
	{
		return Super::NativeOnKeyDown(Geometry, Event);
	}
	const bool bFirst = !bKeys;
	bKeys = true;
	if (!Find(Focus))
	{
		Focus = DefaultFocus();
		if (!Find(Focus) && Hits.Num() > 0)
		{
			Focus = Hits[0].Key;
		}
		if (!bPress)
		{
			return FReply::Handled(); // the first press shows where the focus is
		}
	}
	if (bFirst && !bPress)
	{
		return FReply::Handled();
	}
	const FHit* At = Find(Focus);
	if (bPress)
	{
		Press(Focus);
	}
	else if (Key == EKeys::Tab)
	{
		const int32 I = Hits.IndexOfByPredicate([&](const FHit& H) { return H.Key == Focus; });
		const int32 Step = Event.IsShiftDown() ? -1 : 1;
		Focus = Hits[(I + Step + Hits.Num()) % Hits.Num()].Key;
	}
	else if (bLeft && At && At->OnLeft)
	{
		const TFunction<void()> Left = At->OnLeft;
		Left();
	}
	else if (bRight && At && At->OnRight)
	{
		const TFunction<void()> Right = At->OnRight;
		Right();
	}
	else
	{
		MoveFocus(FVector2f(bLeft ? -1.f : bRight ? 1.f : 0.f, bUp ? -1.f : bDown ? 1.f : 0.f));
	}
	Invalidate(EInvalidateWidgetReason::Paint);
	return FReply::Handled();
}

FReply URideMenu::NativeOnMouseButtonDown(const FGeometry& Geometry, const FPointerEvent& Event)
{
	if (Event.GetEffectingButton() != EKeys::LeftMouseButton)
	{
		return FReply::Handled();
	}
	Mouse = ToFrame(Geometry, Event.GetScreenSpacePosition());
	bKeys = false;
	for (int32 I = Hits.Num() - 1; I >= 0; --I)
	{
		const FHit& H = Hits[I];
		if (!H.Contains(Mouse))
		{
			continue;
		}
		Focus = H.Key;
		if (H.OnDrag)
		{
			Dragging = H.Key;
			const TFunction<void(float)> Drag = H.OnDrag;
			Drag((Mouse.X - H.X) / FMath::Max(H.W, 1.f));
			return FReply::Handled().CaptureMouse(TakeWidget());
		}
		if (H.OnClick)
		{
			Press(H.Key);
			return FReply::Handled();
		}
	}
	return FReply::Handled();
}

FReply URideMenu::NativeOnMouseButtonUp(const FGeometry& Geometry, const FPointerEvent& Event)
{
	if (!Dragging.IsEmpty())
	{
		Dragging.Reset();
		return FReply::Handled().ReleaseMouseCapture();
	}
	return FReply::Handled();
}

FReply URideMenu::NativeOnMouseMove(const FGeometry& Geometry, const FPointerEvent& Event)
{
	Mouse = ToFrame(Geometry, Event.GetScreenSpacePosition());
	if (!Dragging.IsEmpty())
	{
		if (const FHit* H = Find(Dragging); H && H->OnDrag)
		{
			const TFunction<void(float)> Drag = H->OnDrag;
			Drag((Mouse.X - H->X) / FMath::Max(H->W, 1.f));
		}
	}
	if (bKeys && Event.GetCursorDelta().SizeSquared() > 4.f)
	{
		bKeys = false; // the mouse is back in charge
	}
	return FReply::Handled();
}

FReply URideMenu::NativeOnMouseWheel(const FGeometry& Geometry, const FPointerEvent& Event)
{
	if (Current == ERideMenuPage::Summary)
	{
		RidesFirst = FMath::Clamp(RidesFirst - FMath::RoundToInt32(Event.GetWheelDelta()), 0,
		                          FMath::Max(History.Num() - 1, 0));
	}
	return FReply::Handled();
}

int32 URideMenu::NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
                             FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
                             bool bParentEnabled) const
{
	Layer = Super::NativePaint(Args, Geometry, Culling, Out, Layer, Style, bParentEnabled);
	const FVector2f Local = FVector2f(Geometry.GetLocalSize());
	if (Local.Y < 1.f)
	{
		return Layer;
	}
	Scale = Local.Y / 1080.f;
	// What the mouse is over, from the last frame's hits; this frame's are made as it paints.
	Hover.Reset();
	for (int32 I = Hits.Num() - 1; I >= 0; --I)
	{
		if (Hits[I].Contains(Mouse) && (Hits[I].OnClick || Hits[I].OnDrag))
		{
			Hover = Hits[I].Key;
			break;
		}
	}
	Hits.Reset();
	const FPainter P{Geometry, Out, Layer, Scale};
	ARidePawn* Pawn = Rider();
	const FLinearColor Accent = Pawn ? Pawn->Profile.Accent() : Hex(0xFFAD50);
	FPaintContext C{P, *const_cast<URideMenu*>(this), Pawn, Local.X / Scale, Accent, TextOn(Accent),
	                FPlatformTime::Seconds()};
	switch (Current)
	{
	case ERideMenuPage::Start: PaintStart(C); break;
	case ERideMenuPage::Routes: PaintRoutes(C); break;
	case ERideMenuPage::Sensors: PaintSensors(C); break;
	case ERideMenuPage::Settings: PaintSettings(C); break;
	case ERideMenuPage::Stats: PaintStats(C); break;
	case ERideMenuPage::Pause: PaintPause(C); break;
	case ERideMenuPage::Summary: PaintReview(C); break;
	}
	return P.Layer;
}

// ---- The routes ----------------------------------------------------------------------------------

void URideMenu::ReadMap()
{
	const FString Folder = FPaths::ProjectContentDir() / TEXT("Menu");
	FString Json;
	TSharedPtr<FJsonObject> Meta;
	if (!FFileHelper::LoadFileToString(Json, *(Folder / TEXT("map.json"))) ||
	    !FJsonSerializer::Deserialize(TJsonReaderFactory<>::Create(Json), Meta) || !Meta)
	{
		UE_LOG(LogTemp, Warning, TEXT("RIDE menu: no map in %s (scripts/make_menu_map.py)"), *Folder);
		return;
	}
	MapWidth = Meta->GetNumberField(TEXT("width"));
	MapHeight = Meta->GetNumberField(TEXT("height"));
	MapScale = Meta->GetNumberField(TEXT("scale"));
	MapX0 = Meta->GetNumberField(TEXT("x0"));
	MapY0 = Meta->GetNumberField(TEXT("y0"));
	if (const TSharedPtr<FJsonObject>* Labels; Meta->TryGetObjectField(TEXT("labels"), Labels))
	{
		for (const TPair<FString, TSharedPtr<FJsonValue>>& Label : (*Labels)->Values)
		{
			const TArray<TSharedPtr<FJsonValue>>& At = Label.Value->AsArray();
			if (At.Num() == 2)
			{
				MapLabels.Add(Label.Key, FVector2f(static_cast<float>(At[0]->AsNumber()), static_cast<float>(At[1]->AsNumber())));
			}
		}
	}
	MapTexture = FImageUtils::ImportFileAsTexture2D(Folder / TEXT("map.jpg"));
	if (MapTexture)
	{
		MapBrush.SetResourceObject(MapTexture);
		MapBrush.ImageSize = FVector2D(MapWidth, MapHeight);
		MapBrush.DrawAs = ESlateBrushDrawType::RoundedBox;
	}
}

const URideMenu::FRouteSurvey* URideMenu::SurveyOf(const URideTrack* Track) const
{
	return Surveys.FindByPredicate([&](const FRouteSurvey& S) { return S.Track.Get() == Track; });
}

// Each route's length, climbing, profile, islands and line on the map, worked out once.
void URideMenu::SurveyRoutes()
{
	const ARidePawn* Pawn = Rider();
	if (!Pawn)
	{
		return;
	}
	Surveys.RemoveAll([&](const FRouteSurvey& S) { return !S.Track.IsValid(); });
	for (const URideTrack* Track : Pawn->Routes)
	{
		if (!Track || SurveyOf(Track) || Track->Centre.Num() < 2)
		{
			continue;
		}
		FRouteSurvey& S = Surveys.AddDefaulted_GetRef();
		S.Track = Track;
		S.Title = Track->Title.IsEmpty()
			? FPackageName::GetShortName(
				  FPackageName::GetLongPackagePath(Track->GetOutermost()->GetName()).LeftChop(5 /* "/Ride" */))
			: Track->Title;
		S.Blurb = Track->Blurb;
		S.bClosed = Track->bClosed;
		const float Length = Track->Length();
		S.Km = Length / 100000.f;
		for (float At = 1000.f; At < Length; At += 1000.f)
		{
			S.Climb += FMath::Max(0.f, Track->Height(At) - Track->Height(At - 1000.f)) / 100.f;
		}
		constexpr int32 Columns = 240;
		for (int32 I = 0; I < Columns; ++I)
		{
			const float H = Track->Height(Length * I / (Columns - 1.f)) / 100.f;
			S.Heights.Add(H);
			S.Highest = FMath::Max(S.Highest, H);
		}
		for (const FRideStretch& Stretch : Track->Stretches)
		{
			const FString Island = IslandColour(Stretch.Name) == Muted ? FString(TEXT("The Coast")) : Stretch.Name;
			if (S.Islands.Num() == 0 || S.Islands.Last().Value != Island)
			{
				S.Islands.Add({FMath::Clamp(Stretch.Metres * 100.f / Length, 0.f, 1.f), Island});
			}
		}
		if (S.Islands.Num() > 0)
		{
			S.Islands[0].Key = 0.f;
		}
		// The road on the map, a point every 50 m, in the map's pixels (or world metres without one).
		const int32 Every = FMath::Max(1, FMath::RoundToInt32(5000.f / Track->Step));
		for (int32 I = 0; I < Track->Centre.Num(); I += Every)
		{
			const FVector& C = Track->Centre[I];
			S.Map.Add(MapScale > 0.f ? FVector2f(C.X * MapScale + MapX0, C.Y * MapScale + MapY0)
			                         : FVector2f(C.X / 100.f, C.Y / 100.f));
		}
		if (S.bClosed && S.Map.Num() > 0)
		{
			S.Map.Add(S.Map[0]);
		}
	}
	// The routes' order, as the pawn lists them.
	Surveys.Sort([&](const FRouteSurvey& A, const FRouteSurvey& B)
	             { return Pawn->Routes.IndexOfByKey(A.Track.Get()) < Pawn->Routes.IndexOfByKey(B.Track.Get()); });
}

// ---- The stats -----------------------------------------------------------------------------------

void URideMenu::ReadHistory() { History = FRideRecorder::History(); }

void URideMenu::WorkOutStats()
{
	const FDateTime Now = FDateTime::Now();
	const FDateTime Today = Now.GetDate();
	const FDateTime Monday = Today - FTimespan::FromDays(static_cast<int32>(Today.GetDayOfWeek()));
	const FDateTime From[4] = {Monday, FDateTime(Today.GetYear(), Today.GetMonth(), 1), FDateTime(Today.GetYear(), 1, 1),
	                           FDateTime::MinValue()};
	for (FTotals& T : Totals)
	{
		T = FTotals();
	}
	WeekStarts.Reset();
	WeekKm.Init(0.0, 12);
	for (int32 I = 0; I < 12; ++I)
	{
		WeekStarts.Add(Monday - FTimespan::FromDays(7 * (11 - I)));
	}
	// The load needs months before the chart's 90 days to have built up.
	constexpr int32 LoadDays = 90, Lead = 180;
	std::vector<double> Daily(LoadDays + Lead, 0.0);
	CurveAll = {};
	Curve90 = {};
	TMap<FString, double> Islands;
	TSet<FString> Ridden;
	TMap<FString, FClimbBest> Climbs;
	for (const FRideEntry& E : History)
	{
		const double Km = E.Summary.Metres / 1000.0;
		for (int32 P = 0; P < 4; ++P)
		{
			if (E.Started >= From[P])
			{
				FTotals& T = Totals[P];
				++T.Rides;
				T.Km += Km;
				T.Seconds += E.Summary.Seconds;
				T.Climbed += E.Summary.Climbed;
				T.Kj += E.Summary.Kilojoules;
				T.Tss += E.Summary.Tss;
			}
		}
		for (int32 W = 11; W >= 0; --W)
		{
			if (E.Started >= WeekStarts[W])
			{
				if (E.Started < WeekStarts[W] + FTimespan::FromDays(7))
				{
					WeekKm[W] += Km;
				}
				break;
			}
		}
		const int32 DaysAgo = (Today - E.Started.GetDate()).GetDays();
		if (DaysAgo >= 0 && DaysAgo < static_cast<int32>(Daily.size()))
		{
			Daily[Daily.size() - 1 - DaysAgo] += E.Summary.Tss;
		}
		CurveAll = Veloverse::Best(CurveAll, E.Curve);
		if (DaysAgo < 90)
		{
			Curve90 = Veloverse::Best(Curve90, E.Curve);
		}
		// The island kilometres: the ride's distance shared out as its route's stretches are.
		if (const FRouteSurvey* S = Surveys.FindByPredicate([&](const FRouteSurvey& R) { return R.Title == E.Route; }))
		{
			Ridden.Add(S->Title);
			for (int32 I = 0; I < S->Islands.Num(); ++I)
			{
				const float End = I + 1 < S->Islands.Num() ? S->Islands[I + 1].Key : 1.f;
				Islands.FindOrAdd(S->Islands[I].Value) += Km * (End - S->Islands[I].Key);
			}
		}
		for (const FRideClimb& Climb : E.Climbs)
		{
			FClimbBest& Best = Climbs.FindOrAdd(Climb.Name);
			Best.Name = Climb.Name;
			++Best.Tries;
			if (Best.Seconds <= 0.0 || Climb.Seconds < Best.Seconds)
			{
				Best.Seconds = Climb.Seconds;
				Best.Watts = Climb.Watts;
				Best.bRecent = Climb.bPr && DaysAgo < 7;
			}
		}
	}
	Load = Veloverse::TrainingLoad(Daily);
	Load.Fitness.erase(Load.Fitness.begin(), Load.Fitness.begin() + Lead);
	Load.Fatigue.erase(Load.Fatigue.begin(), Load.Fatigue.begin() + Lead);
	DayTss.Reset();
	for (int32 I = Lead; I < static_cast<int32>(Daily.size()); ++I)
	{
		DayTss.Add(Daily[I]);
	}
	IslandKm.Reset();
	for (const TCHAR* Name : IslandNames)
	{
		IslandKm.Add({Name, Islands.FindRef(Name)});
	}
	RoutesRidden = Ridden.Num();
	Climbs.GenerateValueArray(ClimbBests);
	ClimbBests.Sort([](const FClimbBest& A, const FClimbBest& B) { return A.Seconds > B.Seconds; });
}

// ---- A ride's review -----------------------------------------------------------------------------

void URideMenu::ReviewRide(int32 Entry)
{
	if (!History.IsValidIndex(Entry))
	{
		return;
	}
	const FRideEntry& E = History[Entry];
	Review = FReview();
	Review.bValid = true;
	Review.Entry = Entry;
	Review.Route = E.Route.IsEmpty() ? FString(TEXT("Ride")) : E.Route;
	Review.Started = E.Started;
	Review.Ftp = E.Ftp > 0.f ? E.Ftp : 250.f;
	Review.MaxHeartRate = E.MaxHeartRate;
	Review.Xp = E.Xp;
	Review.Climbs = E.Climbs;
	Review.Saved = E.File;
	if (const FRouteSurvey* S = Surveys.FindByPredicate([&](const FRouteSurvey& R) { return R.Title == E.Route; }))
	{
		TArray<FString> Names = {TEXT("Harbour Isle")};
		for (const TPair<float, FString>& Island : S->Islands)
		{
			if (Island.Value != TEXT("The Coast"))
			{
				Names.AddUnique(Island.Value);
			}
		}
		Review.Islands = FString::Join(Names, TEXT(" and "));
	}
	FillReview(FRideRecorder::LoadSamples(E.File));
	Review.Summary = E.Summary.Seconds > 0.0 ? E.Summary : Review.Summary;
	// Each ride takes its place in the list, so the one shown stays in view.
	const int32 Shown = 12;
	if (Entry < RidesFirst || Entry >= RidesFirst + Shown)
	{
		RidesFirst = FMath::Max(0, Entry - Shown / 2);
	}
}

void URideMenu::ReviewJustRidden()
{
	ARidePawn* Pawn = Rider();
	if (!Pawn)
	{
		return;
	}
	bFromStats = false;
	ReadHistory();
	SurveyRoutes();
	const int32 Kept = Pawn->SavedRide.IsEmpty()
		? INDEX_NONE
		: History.IndexOfByPredicate([&](const FRideEntry& E) { return E.File == Pawn->SavedRide; });
	if (Kept != INDEX_NONE)
	{
		ReviewRide(Kept);
	}
	else
	{
		Review = FReview();
		Review.bValid = true;
		Review.Started = FDateTime::Now();
		Review.Ftp = Pawn->Ftp;
		Review.MaxHeartRate = Pawn->Profile.MaxHeartRate;
		FillReview(Pawn->Recorder.Samples());
		for (const FSegmentResult& R : Pawn->Segments.Results())
		{
			FRideClimb& Climb = Review.Climbs.AddDefaulted_GetRef();
			Climb.Name = R.Name;
			Climb.Seconds = R.Effort.Seconds;
			Climb.Watts = R.Effort.AverageWatts;
			Climb.Rank = R.Ranking.Rank;
			Climb.Of = R.Ranking.Of;
			Climb.bPr = R.Ranking.Pr(R.Effort.Seconds);
		}
	}
	Review.bJustRidden = true;
	Review.Saved = Pawn->SavedRide;
	Review.Xp = Pawn->EarnedXp.Total();
	const FRouteSurvey* S = SurveyOf(Pawn->Track);
	if (S)
	{
		Review.Route = S->Title;
	}
	// Where the ride crossed each island: its metres along the route from where it started.
	const double RouteM = Pawn->Track ? Pawn->Track->Length() / 100.0 : 0.0;
	const double RideM = Review.Summary.Metres;
	if (S && S->Islands.Num() > 0 && RouteM > 0.0 && RideM > 0.0)
	{
		TArray<FString> Names = {TEXT("Harbour Isle")};
		for (int32 I = 0; I <= 200; ++I)
		{
			double M = Pawn->StartMetres + RideM * I / 200.0;
			M = S->bClosed ? FMath::Fmod(M, RouteM) : FMath::Min(M, RouteM);
			const float Share = static_cast<float>(M / RouteM);
			int32 Island = 0;
			while (Island + 1 < S->Islands.Num() && S->Islands[Island + 1].Key <= Share)
			{
				++Island;
			}
			const FString& Name = S->Islands[Island].Value;
			if (Review.IslandSpans.Num() == 0 || Review.IslandSpans.Last().Value != Name)
			{
				Review.IslandSpans.Add({I / 200.f, Name});
			}
			if (Name != TEXT("The Coast"))
			{
				Names.AddUnique(Name);
			}
		}
		Review.Islands = FString::Join(Names, TEXT(" and "));
	}
}

// The traces, zones, histogram and best-effort notes from a ride's samples.
void URideMenu::FillReview(const std::vector<Veloverse::FSample>& Samples)
{
	Review.Summary = Veloverse::Summarize(Samples, Review.Ftp);
	Review.Traces = Veloverse::Traces(Samples, 240);
	Review.HeartSeconds = Veloverse::HeartZoneSeconds(Samples, Review.MaxHeartRate);
	Review.Cadence = Veloverse::CadenceHistogram(Samples, 60.0, 110.0, 5.0);
	// Against the rides before it: a best ever, or the best in 90 days.
	const Veloverse::FCurve Curve = Veloverse::PowerCurve(Samples);
	Veloverse::FCurve Ever = {}, Recent = {};
	for (int32 I = 0; I < History.Num(); ++I)
	{
		const FRideEntry& E = History[I];
		if (I == Review.Entry || E.Started >= Review.Started)
		{
			continue;
		}
		Ever = Veloverse::Best(Ever, E.Curve);
		if ((Review.Started - E.Started).GetTotalDays() < 90.0)
		{
			Recent = Veloverse::Best(Recent, E.Curve);
		}
	}
	const int32 Index[5] = {1, 4, 6, 8, 10}; // 5 s, 1 min, 5 min, 20 min, 1 hr in CurveDurations
	const bool bOthers = History.Num() > (Review.Entry >= 0 ? 1 : 0);
	for (int32 I = 0; I < 5; ++I)
	{
		const double W = Curve[Index[I]];
		Review.BestNote[I] = !bOthers || W <= 0.0 ? FString()
			: W > Ever[Index[I]]                  ? FString(TEXT("Best ever"))
			: W > Recent[Index[I]]                ? FString(TEXT("Best in 90 days"))
			                                      : FString();
	}
}
