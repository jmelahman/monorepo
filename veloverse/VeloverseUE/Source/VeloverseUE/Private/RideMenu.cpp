#include "RideMenu.h"

#include "Blueprint/WidgetTree.h"
#include "Brushes/SlateRoundedBoxBrush.h"
#include "Components/Border.h"
#include "Components/HorizontalBox.h"
#include "Components/HorizontalBoxSlot.h"
#include "Components/SizeBox.h"
#include "Components/TextBlock.h"
#include "Components/VerticalBox.h"
#include "Components/VerticalBoxSlot.h"
#include "Components/WidgetSwitcher.h"
#include "GameFramework/GameUserSettings.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "HAL/FileManager.h"
#include "HAL/PlatformProcess.h"
#include "RideChart.h"
#include "RidePawn.h"
#include "RidePlayerController.h"
#include "RideTrack.h"
#include "Styling/CoreStyle.h"

namespace
{
// The HUD's orange.
FLinearColor Accent(1.f, 0.42f, 0.08f); // the profile's HUD colour: set as the menu builds, and by Recolour
const FLinearColor Faint(1.f, 1.f, 1.f, 0.6f);
const TArray<FString> Levels = {TEXT("Low"), TEXT("Medium"), TEXT("High"), TEXT("Epic"), TEXT("Cinematic")};
const TArray<float> Scales = {50.f, 67.f, 75.f, 85.f, 100.f};
const TArray<float> FrameCaps = {30.f, 60.f, 90.f, 120.f, 0.f};
const TArray<EWindowMode::Type> WindowModes = {EWindowMode::Fullscreen, EWindowMode::WindowedFullscreen,
                                               EWindowMode::Windowed};

UGameUserSettings& Settings() { return *UGameUserSettings::GetGameUserSettings(); }

// Applies and saves a changed video setting at once, so its effect shows behind the menu.
void Apply() { Settings().ApplySettings(false); }

int32 Nearest(const TArray<float>& Values, float Value)
{
	int32 Best = 0;
	for (int32 I = 1; I < Values.Num(); ++I)
	{
		if (FMath::Abs(Values[I] - Value) < FMath::Abs(Values[Best] - Value))
		{
			Best = I;
		}
	}
	return Best;
}

FString Clock(float Seconds)
{
	const int32 S = FMath::FloorToInt32(Seconds);
	return FString::Printf(TEXT("%d:%02d:%02d"), S / 3600, S / 60 % 60, S % 60);
}

// The choices for a number setting, From to To in Step, and the index nearest a value.
TArray<FString> Range(int32 From, int32 To, int32 Step, const TCHAR* Unit)
{
	TArray<FString> Out;
	for (int32 V = From; V <= To; V += Step)
	{
		Out.Add(FString::Printf(TEXT("%d %s"), V, Unit));
	}
	return Out;
}
int32 InRange(float Value, int32 From, int32 To, int32 Step)
{
	return FMath::Clamp(FMath::RoundToInt32((Value - From) / Step), 0, (To - From) / Step);
}

// "212 W", or a dash for a ride too short to have it.
FString Watts(double W) { return W > 0.0 ? FString::Printf(TEXT("%.0f W"), W) : FString(TEXT("—")); }
} // namespace

void URideButton::Bind() { OnClicked.AddUniqueDynamic(this, &URideButton::Fire); }

void URideButton::Fire()
{
	if (Action)
	{
		Action();
	}
}

void URideMenu::NativeOnInitialized()
{
	Super::NativeOnInitialized();
	SetIsFocusable(true);
}

TSharedRef<SWidget> URideMenu::RebuildWidget()
{
	if (WidgetTree && !WidgetTree->RootWidget)
	{
		Build();
	}
	return Super::RebuildWidget();
}

FReply URideMenu::NativeOnKeyDown(const FGeometry& Geometry, const FKeyEvent& Event)
{
	const FKey Key = Event.GetKey();
	if (Key == EKeys::Escape || Key == EKeys::P || Key == EKeys::Gamepad_Special_Right)
	{
		if (Current == ERideMenuPage::Video)
		{
			Show(Back);
			return FReply::Handled();
		}
		if (Current == ERideMenuPage::Profile)
		{
			Show(ERideMenuPage::Start);
			return FReply::Handled();
		}
		const ARidePawn* Pawn = Rider();
		if (Current == ERideMenuPage::Pause && Pawn && !Pawn->bFinished)
		{
			Controller()->Resume();
			return FReply::Handled();
		}
	}
	return Super::NativeOnKeyDown(Geometry, Event);
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
	// The menu can open before the rider has read its track and trainer, so it keeps catching up.
	RefreshClock += DeltaTime;
	if (RefreshClock > 0.25f)
	{
		RefreshClock = 0.f;
		Refresh();
	}
}

void URideMenu::Show(ERideMenuPage Page)
{
	if (Page == ERideMenuPage::Video && Current != ERideMenuPage::Video)
	{
		Back = Current;
	}
	Current = Page;
	TakeWidget(); // builds the pages on first use
	Pages->SetActiveWidgetIndex(static_cast<int32>(Page));
	if (Page == ERideMenuPage::Profile)
	{
		ShowHistory();
	}
	if (Page == ERideMenuPage::Summary)
	{
		ShowSummary();
	}
	Refresh();
	if (URideButton* First = FirstButton[static_cast<int32>(Page)])
	{
		First->SetKeyboardFocus();
	}
}

// ---- Building --------------------------------------------------------------------------------

UTextBlock* URideMenu::Text(const FString& String, int32 Size, FLinearColor Colour, bool bBold)
{
	UTextBlock* Block = WidgetTree->ConstructWidget<UTextBlock>();
	Block->SetText(FText::FromString(String));
	Block->SetFont(FCoreStyle::GetDefaultFontStyle(bBold ? "Bold" : "Regular", Size));
	Block->SetColorAndOpacity(FSlateColor(Colour));
	Block->SetAutoWrapText(true);
	return Block;
}

// A button's look: its colour, and its colour hovered (brighter, when not given).
static FButtonStyle ButtonStyle(FLinearColor Base, FLinearColor Hover)
{
	if (Hover == FLinearColor())
	{
		Hover = Base * 1.15f;
		Hover.A = Base.A;
	}
	FButtonStyle Style;
	Style.SetNormal(FSlateRoundedBoxBrush(Base, 6.f));
	Style.SetHovered(FSlateRoundedBoxBrush(Hover, 6.f));
	Style.SetPressed(FSlateRoundedBoxBrush(Base * 0.8f, 6.f));
	Style.SetDisabled(FSlateRoundedBoxBrush(FLinearColor(1.f, 1.f, 1.f, 0.03f), 6.f));
	Style.SetNormalPadding(FMargin(18.f, 10.f));
	Style.SetPressedPadding(FMargin(18.f, 11.f, 18.f, 9.f));
	return Style;
}

URideButton* URideMenu::Button(const FString& Label, TFunction<void()> Action, bool bPrimary)
{
	URideButton* Button = WidgetTree->ConstructWidget<URideButton>();
	Button->SetStyle(ButtonStyle(bPrimary ? Accent : FLinearColor(1.f, 1.f, 1.f, 0.08f),
	                             bPrimary ? FLinearColor() : FLinearColor(1.f, 1.f, 1.f, 0.18f)));
	if (bPrimary)
	{
		PrimaryButtons.Add(Button);
	}
	UTextBlock* Caption = Text(Label, 18, FLinearColor::White, bPrimary);
	Caption->SetAutoWrapText(false);
	Button->SetContent(Caption);
	Button->Action = MoveTemp(Action);
	Button->Bind();
	return Button;
}

UWidget* URideMenu::Cycler(const FString& Label, TArray<FString> Choices, TFunction<int32()> Get,
                           TFunction<void(int32)> Set)
{
	const int32 Index = Cyclers.Num();
	UHorizontalBox* Row = WidgetTree->ConstructWidget<UHorizontalBox>();
	UHorizontalBoxSlot* Name = Row->AddChildToHorizontalBox(Text(Label, 16));
	Name->SetSize(FSlateChildSize(ESlateSizeRule::Fill));
	Name->SetVerticalAlignment(VAlign_Center);
	Row->AddChildToHorizontalBox(Button(TEXT("<"), [this, Index] { Cycle(Index, -1); }));
	USizeBox* Box = WidgetTree->ConstructWidget<USizeBox>();
	Box->SetWidthOverride(170.f);
	UTextBlock* Value = Text(TEXT(""), 16, Accent);
	AccentTexts.Add(Value);
	Value->SetJustification(ETextJustify::Center);
	Box->SetContent(Value);
	UHorizontalBoxSlot* Middle = Row->AddChildToHorizontalBox(Box);
	Middle->SetVerticalAlignment(VAlign_Center);
	Row->AddChildToHorizontalBox(Button(TEXT(">"), [this, Index] { Cycle(Index, 1); }));
	Cyclers.Add({MoveTemp(Choices), MoveTemp(Get), MoveTemp(Set), Value});
	return Row;
}

void URideMenu::Add(UVerticalBox* Box, UWidget* Widget, float Above)
{
	UVerticalBoxSlot* Slot = Box->AddChildToVerticalBox(Widget);
	Slot->SetPadding(FMargin(0.f, Above, 0.f, 0.f));
	Slot->SetHorizontalAlignment(HAlign_Fill);
}

UVerticalBox* URideMenu::Page(const FString& Title)
{
	UVerticalBox* Box = WidgetTree->ConstructWidget<UVerticalBox>();
	Pages->AddChild(Box);
	if (!Title.IsEmpty())
	{
		Add(Box, Text(Title, 34, FLinearColor::White, true), 0.f);
	}
	return Box;
}

void URideMenu::Build()
{
	// The world stays in view: a light shade over it, and the panel on the left.
	UBorder* Shade = WidgetTree->ConstructWidget<UBorder>();
	Shade->SetBrushColor(FLinearColor(0.f, 0.f, 0.f, 0.3f));
	Shade->SetPadding(FMargin(80.f, 40.f));
	Shade->SetHorizontalAlignment(HAlign_Left);
	Shade->SetVerticalAlignment(VAlign_Center);
	WidgetTree->RootWidget = Shade;

	USizeBox* Width = WidgetTree->ConstructWidget<USizeBox>();
	Width->SetWidthOverride(560.f);
	Shade->SetContent(Width);
	UBorder* Panel = WidgetTree->ConstructWidget<UBorder>();
	Panel->SetBrush(FSlateRoundedBoxBrush(FLinearColor(0.015f, 0.015f, 0.02f, 0.85f), 12.f));
	Panel->SetPadding(FMargin(40.f, 36.f));
	Width->SetContent(Panel);

	Pages = WidgetTree->ConstructWidget<UWidgetSwitcher>();
	Panel->SetContent(Pages);
	// In ERideMenuPage's order: the switcher's index is the page.
	FirstButton.SetNum(5);
	Accent = (Rider() ? Rider()->Profile : FRideProfile::Load()).Accent();
	BuildStart();
	BuildPause();
	BuildVideo();
	BuildProfile();
	BuildSummary();
}

void URideMenu::BuildStart()
{
	UVerticalBox* P = Page(TEXT(""));
	Add(P, Text(TEXT("Veloverse"), 44, FLinearColor::White, true), 0.f);
	StartLevel = Text(TEXT(""), 13, Faint);
	Add(P, StartLevel, 0.f);
	CourseTitle = Text(TEXT(""), 22, Accent);
	AccentTexts.Add(CourseTitle);
	Add(P, CourseTitle, 0.f);
	CourseText = Text(TEXT(""), 15, Faint);
	Add(P, CourseText, 10.f);
	// The route's profile, its climbs (the timed segments) in the accent, and the start.
	RoutePreview = CreateWidget<URideChart>(this, URideChart::StaticClass());
	USizeBox* PreviewBox = WidgetTree->ConstructWidget<USizeBox>();
	PreviewBox->SetHeightOverride(64.f);
	PreviewBox->SetContent(RoutePreview);
	Add(P, PreviewBox, 6.f);
	CourseClimbs = Text(TEXT(""), 12, Faint);
	Add(P, CourseClimbs, 4.f);
	RouteCycler = Cyclers.Num();
	RouteRow = Cycler(
		TEXT("Route"), {},
		[this]
		{
			const ARidePawn* Pawn = Rider();
			return Pawn ? Pawn->RouteIndex() : -1;
		},
		[this](int32 I)
		{
			if (ARidePawn* Pawn = Rider())
			{
				Pawn->UseRoute(I);
			}
		});
	RouteRow->SetVisibility(ESlateVisibility::Collapsed);
	Add(P, RouteRow, 20.f);
	StartCycler = Cyclers.Num();
	Add(P,
	    Cycler(
		    TEXT("Start in"), {},
		    [this]
		    {
			    const ARidePawn* Pawn = Rider();
			    return Pawn ? Pawn->StartIndex() : -1;
		    },
		    [this](int32 I)
		    {
			    if (ARidePawn* Pawn = Rider())
			    {
				    Pawn->UseStart(I);
			    }
		    }),
	    12.f);
	Add(P,
	    Cycler(
		    TEXT("Power from"), {TEXT("Keyboard"), TEXT("Simulated trainer")},
		    [this]
		    {
			    const ARidePawn* Pawn = Rider();
			    return Pawn ? ARidePawn::TrainerSources.IndexOfByKey(Pawn->TrainerSource) : -1;
		    },
		    [this](int32 I)
		    {
			    if (ARidePawn* Pawn = Rider())
			    {
				    Pawn->UseTrainer(ARidePawn::TrainerSources[I]);
			    }
		    }),
	    12.f);
	TrainerNote = Text(TEXT(""), 13, Faint);
	Add(P, TrainerNote, 6.f);
	// Ride the road, or a workout from the library with its length and load.
	// Its choices are filled in by ReadWorkouts, again whenever the FTP changes.
	WorkoutCycler = Cyclers.Num();
	Add(P,
	    Cycler(
		    TEXT("Workout"), {},
		    [this]
		    {
			    const ARidePawn* Pawn = Rider();
			    if (!Pawn || !Pawn->Workout.Active())
			    {
				    return 0;
			    }
			    const int32 I = Workouts.IndexOfByPredicate([Pawn](const Veloverse::FWorkout& W)
			                                                { return W.Name == Pawn->Workout.Workout().Name; });
			    return I + 1;
		    },
		    [this](int32 I)
		    {
			    if (ARidePawn* Pawn = Rider())
			    {
				    if (Workouts.IsValidIndex(I - 1))
				    {
					    Pawn->StartWorkout(Workouts[I - 1]);
				    }
				    else
				    {
					    Pawn->EndWorkout();
				    }
			    }
		    }),
	    12.f);
	ReadWorkouts(Rider() ? Rider()->Ftp : FRideProfile::Load().Ftp);
	URideButton* Go = Button(TEXT("Ride"), [this] { Controller()->Ride(); }, true);
	FirstButton[static_cast<int32>(ERideMenuPage::Start)] = Go;
	Add(P, Go, 28.f);
	Add(P, Button(TEXT("Profile and rides"), [this] { Show(ERideMenuPage::Profile); }), 10.f);
	Add(P, Button(TEXT("Video settings"), [this] { Show(ERideMenuPage::Video); }), 10.f);
	Add(P, Button(TEXT("Quit"), [this] { Controller()->Quit(); }), 10.f);
	Add(P, Text(TEXT("W/S or Up/Down change power. Esc pauses. In a workout: [ and ] easier or harder, N skips a step, E toggles ERG."), 13, Faint), 24.f);
}

void URideMenu::BuildPause()
{
	UVerticalBox* P = Page(TEXT(""));
	PauseTitle = Text(TEXT("Paused"), 34, FLinearColor::White, true);
	Add(P, PauseTitle, 0.f);
	PauseStats = Text(TEXT(""), 15, Faint);
	Add(P, PauseStats, 10.f);
	ResumeButton = Button(TEXT("Resume"), [this] { Controller()->Resume(); }, true);
	Add(P, ResumeButton, 28.f);
	EndButton = Button(TEXT("End ride"), [this] { Controller()->EndRide(); });
	Add(P, EndButton, 10.f);
	Add(P, Button(TEXT("Video settings"), [this] { Show(ERideMenuPage::Video); }), 10.f);
	Add(P, Button(TEXT("Back to the start"), [this] { Controller()->Restart(); }), 10.f);
	Add(P, Button(TEXT("Quit"), [this] { Controller()->Quit(); }), 10.f);
}

void URideMenu::BuildVideo()
{
	UVerticalBox* P = Page(TEXT("Video"));
	Add(P, Text(TEXT("Changes apply at once and are kept for next time."), 13, Faint), 6.f);
	auto Group = [](int32 (UGameUserSettings::*Get)() const, void (UGameUserSettings::*Set)(int32))
	{
		return TPair<TFunction<int32()>, TFunction<void(int32)>>(
			[Get] { return (Settings().*Get)(); },
			[Set](int32 I)
			{
				(Settings().*Set)(I);
				Apply();
			});
	};
	Add(P,
	    Cycler(
		    TEXT("Quality"), Levels, [] { return Settings().GetOverallScalabilityLevel(); },
		    [](int32 I)
		    {
			    Settings().SetOverallScalabilityLevel(I);
			    Apply();
		    }),
	    20.f);
	const TPair<FString, TPair<TFunction<int32()>, TFunction<void(int32)>>> Groups[] = {
		{TEXT("View distance"),
		 Group(&UGameUserSettings::GetViewDistanceQuality, &UGameUserSettings::SetViewDistanceQuality)},
		{TEXT("Shadows"), Group(&UGameUserSettings::GetShadowQuality, &UGameUserSettings::SetShadowQuality)},
		{TEXT("Lighting"),
		 Group(&UGameUserSettings::GetGlobalIlluminationQuality, &UGameUserSettings::SetGlobalIlluminationQuality)},
		{TEXT("Reflections"),
		 Group(&UGameUserSettings::GetReflectionQuality, &UGameUserSettings::SetReflectionQuality)},
		{TEXT("Foliage"), Group(&UGameUserSettings::GetFoliageQuality, &UGameUserSettings::SetFoliageQuality)},
		{TEXT("Effects"),
		 Group(&UGameUserSettings::GetVisualEffectQuality, &UGameUserSettings::SetVisualEffectQuality)},
	};
	for (const auto& [Label, Access] : Groups)
	{
		Add(P, Cycler(Label, Levels, Access.Key, Access.Value), 6.f);
	}
	Add(P,
	    Cycler(
		    TEXT("Render scale"), {TEXT("50%"), TEXT("67%"), TEXT("75%"), TEXT("85%"), TEXT("100%")},
		    []
		    {
			    float Normalized, Value, Min, Max;
			    Settings().GetResolutionScaleInformationEx(Normalized, Value, Min, Max);
			    return Nearest(Scales, Value);
		    },
		    [](int32 I)
		    {
			    Settings().SetResolutionScaleValueEx(Scales[I]);
			    Apply();
		    }),
	    18.f);
	Add(P,
	    Cycler(
		    TEXT("Frame rate cap"), {TEXT("30"), TEXT("60"), TEXT("90"), TEXT("120"), TEXT("None")},
		    [] { return FrameCaps.IndexOfByKey(Settings().GetFrameRateLimit()); },
		    [](int32 I)
		    {
			    Settings().SetFrameRateLimit(FrameCaps[I]);
			    Apply();
		    }),
	    6.f);
	Add(P,
	    Cycler(
		    TEXT("VSync"), {TEXT("Off"), TEXT("On")}, [] { return Settings().IsVSyncEnabled() ? 1 : 0; },
		    [](int32 I)
		    {
			    Settings().SetVSyncEnabled(I == 1);
			    Apply();
		    }),
	    6.f);
	Add(P,
	    Cycler(
		    TEXT("Window"), {TEXT("Fullscreen"), TEXT("Borderless"), TEXT("Windowed")},
		    [] { return WindowModes.IndexOfByKey(Settings().GetFullscreenMode()); },
		    [](int32 I)
		    {
			    Settings().SetFullscreenMode(WindowModes[I]);
			    Apply();
		    }),
	    6.f);
	URideButton* Done = Button(TEXT("Back"), [this] { Show(Back); }, true);
	FirstButton[static_cast<int32>(ERideMenuPage::Video)] = Done;
	Add(P, Done, 24.f);
}

void URideMenu::BuildProfile()
{
	UVerticalBox* P = Page(TEXT("Profile"));
	Add(P, Text(TEXT("Your weight sets how hard the climbs are; FTP sets the power zones and the training load."), 13,
	            Faint),
	    6.f);
	auto Setting = [this](float FRideProfile::*Field, int32 From, int32 To, int32 Step)
	{
		return TPair<TFunction<int32()>, TFunction<void(int32)>>(
			[this, Field, From, To, Step]
			{
				const ARidePawn* Pawn = Rider();
				return Pawn ? InRange(Pawn->Profile.*Field, From, To, Step) : -1;
			},
			[this, Field, From, Step](int32 I)
			{
				if (ARidePawn* Pawn = Rider())
				{
					FRideProfile Changed = Pawn->Profile;
					Changed.*Field = static_cast<float>(From + I * Step);
					Pawn->SetProfile(Changed);
				}
			});
	};
	const auto Ftp = Setting(&FRideProfile::Ftp, 100, 450, 5);
	Add(P, Cycler(TEXT("FTP"), Range(100, 450, 5, TEXT("W")), Ftp.Key, Ftp.Value), 20.f);
	const auto Weight = Setting(&FRideProfile::WeightKg, 40, 150, 1);
	Add(P, Cycler(TEXT("Your weight"), Range(40, 150, 1, TEXT("kg")), Weight.Key, Weight.Value), 6.f);
	const auto Bike = Setting(&FRideProfile::BikeKg, 6, 20, 1);
	Add(P, Cycler(TEXT("Bike"), Range(6, 20, 1, TEXT("kg")), Bike.Key, Bike.Value), 6.f);
	// The level, and the HUD colours it has unlocked; the HUD and menus take a new one at once.
	ProfileLevel = Text(TEXT(""), 14, Faint);
	Add(P, ProfileLevel, 20.f);
	ThemeCycler = Cyclers.Num();
	Add(P,
	    Cycler(
			TEXT("HUD colour"), UnlockedThemes((Rider() ? Rider()->Profile : FRideProfile::Load()).Xp),
			[this]
			{
				const ARidePawn* Pawn = Rider();
				return Pawn ? FMath::Max(Cyclers[ThemeCycler].Choices.IndexOfByKey(Pawn->Profile.Theme), 0) : 0;
			},
			[this](int32 I)
			{
				const TArray<FString>& Themes = Cyclers[ThemeCycler].Choices;
				if (ARidePawn* Pawn = Rider(); Pawn && Themes.IsValidIndex(I))
				{
					FRideProfile Changed = Pawn->Profile;
					Changed.Theme = Themes[I];
					Pawn->SetProfile(Changed);
				}
			}),
	    6.f);
	Add(P, Text(TEXT("Recent rides"), 20, FLinearColor::White, true), 24.f);
	HistoryText = Text(TEXT(""), 14, Faint);
	Add(P, HistoryText, 8.f);
	URideButton* Done = Button(TEXT("Back"), [this] { Show(ERideMenuPage::Start); }, true);
	FirstButton[static_cast<int32>(ERideMenuPage::Profile)] = Done;
	Add(P, Done, 24.f);
}

void URideMenu::BuildSummary()
{
	UVerticalBox* P = Page(TEXT(""));
	SummaryTitle = Text(TEXT("Ride summary"), 34, FLinearColor::White, true);
	Add(P, SummaryTitle, 0.f);
	SummaryRide = Text(TEXT(""), 16, Accent);
	AccentTexts.Add(SummaryRide);
	Add(P, SummaryRide, 6.f);
	SummaryChart = CreateWidget<URideChart>(this, URideChart::StaticClass());
	USizeBox* ChartBox = WidgetTree->ConstructWidget<USizeBox>();
	ChartBox->SetHeightOverride(150.f);
	ChartBox->SetContent(SummaryChart);
	Add(P, ChartBox, 16.f);
	SummaryPower = Text(TEXT(""), 15, FLinearColor::White);
	Add(P, SummaryPower, 14.f);
	SummaryBest = Text(TEXT(""), 14, Faint);
	Add(P, SummaryBest, 8.f);
	SummaryXp = Text(TEXT(""), 14, FLinearColor::White);
	Add(P, SummaryXp, 10.f);
	SummarySaved = Text(TEXT(""), 12, Faint);
	Add(P, SummarySaved, 10.f);
	URideButton* Again = Button(TEXT("Back to the start"), [this] { Controller()->Restart(); }, true);
	FirstButton[static_cast<int32>(ERideMenuPage::Summary)] = Again;
	Add(P, Again, 22.f);
	Add(P, Button(TEXT("Open the rides folder"),
	              []
	              {
		              const FString Folder = FPaths::ConvertRelativePathToFull(FRideRecorder::RidesFolder());
		              IFileManager::Get().MakeDirectory(*Folder, true);
		              FPlatformProcess::ExploreFolder(*Folder);
	              }),
	    10.f);
	Add(P, Button(TEXT("Quit"), [this] { Controller()->Quit(); }), 10.f);
}

void URideMenu::ShowHistory()
{
	const TArray<FRideEntry> Rides = FRideRecorder::History();
	FString Lines;
	double Week = 0.0, WeekKm = 0.0;
	const FDateTime WeekAgo = FDateTime::Now() - FTimespan::FromDays(7.0);
	for (int32 I = 0; I < Rides.Num(); ++I)
	{
		const FRideEntry& Ride = Rides[I];
		if (Ride.Started >= WeekAgo)
		{
			Week += Ride.Summary.Tss;
			WeekKm += Ride.Summary.Metres / 1000.0;
		}
		if (I < 6)
		{
			Lines += FString::Printf(TEXT("%s   %.1f km in %s   %s   TSS %.0f\n"), *Ride.Started.ToString(TEXT("%d %b %H:%M")),
			                         Ride.Summary.Metres / 1000.0, *Clock(Ride.Summary.Seconds),
			                         *Watts(Ride.Summary.AverageWatts), Ride.Summary.Tss);
		}
	}
	if (Rides.IsEmpty())
	{
		Lines = TEXT("None yet. Rides over 30 seconds are kept here.");
	}
	else
	{
		Lines += FString::Printf(TEXT("\nLast 7 days: %.0f km, training load %.0f TSS. %d rides in all."), WeekKm, Week,
		                         Rides.Num());
	}
	HistoryText->SetText(FText::FromString(Lines));
	if (const ARidePawn* Pawn = Rider())
	{
		const Veloverse::FLevel L = Veloverse::LevelFor(Pawn->Profile.Xp);
		ProfileLevel->SetText(FText::FromString(
			L.Needed > 0 ? FString::Printf(TEXT("Level %d · %d of %d XP to level %d · %lld XP in all"), L.Level, L.Into,
			                               L.Needed, L.Level + 1, Pawn->Profile.Xp)
						 : FString::Printf(TEXT("Level %d, the top · %lld XP in all"), L.Level, Pawn->Profile.Xp)));
	}
}

void URideMenu::ShowSummary()
{
	const ARidePawn* Pawn = Rider();
	if (!Pawn)
	{
		return;
	}
	const Veloverse::FSummary S = Pawn->Recorder.Summary(Pawn->Ftp);
	const bool bEnd = Pawn->Track && !Pawn->Track->bClosed && Pawn->Distance >= Pawn->Track->Length();
	SummaryTitle->SetText(FText::FromString(bEnd ? TEXT("Finished") : TEXT("Ride summary")));
	SummaryRide->SetText(FText::FromString(FString::Printf(TEXT("%.1f km in %s · %.1f km/h · %.0f m climbed"),
	                                                       S.Metres / 1000.0, *Clock(S.Seconds), S.AverageSpeed * 3.6,
	                                                       S.Climbed)));
	SummaryChart->SetRide(Pawn->Recorder.Samples(), Pawn->Ftp);
	FString Power = FString::Printf(TEXT("Average %s · Normalized %s · Max %s\nIntensity %.2f · Training load %.0f TSS · %.0f kJ"),
	                                *Watts(S.AverageWatts), *Watts(S.NormalizedWatts), *Watts(S.MaxWatts), S.Intensity,
	                                S.Tss, S.Kilojoules);
	Power += S.AverageCadence > 0.0 ? FString::Printf(TEXT("\nCadence %.0f rpm"), S.AverageCadence) : FString();
	Power += S.AverageHeartRate > 0.0
		? FString::Printf(TEXT(" · Heart rate %.0f bpm, max %.0f"), S.AverageHeartRate, S.MaxHeartRate)
		: FString();
	SummaryPower->SetText(FText::FromString(Power));
	const TCHAR* Names[] = {TEXT("5 s"), TEXT("1 min"), TEXT("5 min"), TEXT("20 min"), TEXT("1 hr")};
	FString Best = TEXT("Best efforts:");
	for (int32 I = 0; I < UE_ARRAY_COUNT(Names); ++I)
	{
		Best += FString::Printf(TEXT("%s %s %s"), I ? TEXT(" ·") : TEXT(""), Names[I], *Watts(S.Best[I]));
	}
	// This ride's segments: each one's time, and a PR or its place among past rides.
	for (const FSegmentResult& R : Pawn->Segments.Results())
	{
		const int32 Sec = FMath::RoundToInt32(R.Effort.Seconds);
		FString Place = R.Ranking.Pr(R.Effort.Seconds)
			? FString::Printf(TEXT("PR, %.1f s faster"), *R.Ranking.PreviousBest - R.Effort.Seconds)
			: R.Ranking.Of == 1 ? FString(TEXT("first time"))
								: FString::Printf(TEXT("%d of %d"), R.Ranking.Rank, R.Ranking.Of);
		Best += FString::Printf(TEXT("\n%s  %d:%02d at %s, %s"), *R.Name, Sec / 60, Sec % 60,
		                        *Watts(R.Effort.AverageWatts), *Place);
	}
	SummaryBest->SetText(FText::FromString(Best));
	// What the ride earned, the level it leaves the rider at, and anything that unlocked.
	FString Xp;
	if (!Pawn->SavedRide.IsEmpty())
	{
		const Veloverse::FRideXp& E = Pawn->EarnedXp;
		Xp = FString::Printf(TEXT("+%d XP: %d distance · %d climbing"), E.Total(), E.Distance, E.Climbing);
		Xp += E.Segments ? FString::Printf(TEXT(" · %d segments"), E.Segments) : FString();
		Xp += E.Workout ? FString::Printf(TEXT(" · %d workout"), E.Workout) : FString();
		const Veloverse::FLevel Before = Veloverse::LevelFor(Pawn->XpBefore), After = Veloverse::LevelFor(Pawn->Profile.Xp);
		Xp += After.Level > Before.Level ? FString::Printf(TEXT("\nLevel %d! "), After.Level)
										 : FString::Printf(TEXT("\nLevel %d · "), After.Level);
		Xp += After.Needed > 0 ? FString::Printf(TEXT("%d XP to level %d"), After.Needed - After.Into, After.Level + 1)
							   : FString(TEXT("the top level"));
		for (const Veloverse::FTheme& T : Veloverse::Unlocked(Pawn->XpBefore, Pawn->Profile.Xp))
		{
			Xp += FString::Printf(TEXT("\nUnlocked the %s HUD colour: choose it in Profile."), UTF8_TO_TCHAR(T.Name));
		}
	}
	SummaryXp->SetText(FText::FromString(Xp));
	SummaryXp->SetVisibility(Xp.IsEmpty() ? ESlateVisibility::Collapsed : ESlateVisibility::Visible);
	SummarySaved->SetText(FText::FromString(
		!Pawn->SavedRide.IsEmpty()
			? FString::Printf(TEXT("Saved to your rides. %s.fit, in %s, is ready to upload to Strava (strava.com/upload) or Garmin Connect."),
			                  *FPaths::GetBaseFilename(Pawn->SavedRide), *FPaths::GetPath(Pawn->SavedRide))
		: S.Seconds < FRideRecorder::MinSeconds ? FString(TEXT("Under 30 seconds, so not kept."))
												 : FString(TEXT("Not saved."))));
}

// ---- Showing ---------------------------------------------------------------------------------

void URideMenu::Survey()
{
	const ARidePawn* Pawn = Rider();
	if (!Pawn || !Pawn->Track || Surveyed.Get() == Pawn->Track)
	{
		return;
	}
	const URideTrack& Track = *Pawn->Track;
	Surveyed = &Track;
	const float Length = Track.Length();
	CourseKm = Length / 100000.f;
	ClimbMetres = 0.f;
	for (float S = 1000.f; S < Length; S += 1000.f)
	{
		ClimbMetres += FMath::Max(0.f, Track.Height(S) - Track.Height(S - 1000.f)) / 100.f;
	}
	// The preview: 240 heights, and the climbs the ride will time.
	ARidePawn* Mutable = Rider();
	if (!Mutable->Segments.UsesTrack(&Track))
	{
		Mutable->Segments.Use(Track);
	}
	TArray<float> Heights;
	for (int32 I = 0; I < 240; ++I)
	{
		Heights.Add(Track.Height(Length * (I + 0.5f) / 240.f) / 100.f);
	}
	TArray<FVector2f> Climbs;
	FString Names; // the longest and the steepest
	const double Metres = Length / 100.0;
	int32 Longest = 0, Steepest = 0;
	for (int32 I = 0; I < Pawn->Segments.Num(); ++I)
	{
		const Veloverse::FSegment& S = Pawn->Segments.Segment(I);
		Climbs.Add(FVector2f(static_cast<float>(S.Start / Metres), static_cast<float>(S.End / Metres)));
		Longest = S.Length(Metres) > Pawn->Segments.Segment(Longest).Length(Metres) ? I : Longest;
		Steepest = S.Grade(Metres) > Pawn->Segments.Segment(Steepest).Grade(Metres) ? I : Steepest;
	}
	auto Describe = [&](int32 I)
	{
		const Veloverse::FSegment& S = Pawn->Segments.Segment(I);
		return FString::Printf(TEXT("%s, %.1f km at %.0f%%"), *Pawn->Segments.Name(I), S.Length(Metres) / 1000.0,
		                       S.Grade(Metres) * 100.0);
	};
	if (Pawn->Segments.Num() > 0)
	{
		Names = TEXT("longest ") + Describe(Longest);
		Names += Steepest != Longest ? TEXT("; steepest ") + Describe(Steepest) : FString();
	}
	RoutePreview->SetRoute(MoveTemp(Heights), MoveTemp(Climbs), Pawn->StartMetres * 100.f / Length, Accent);
	CourseClimbs->SetText(FText::FromString(
		Pawn->Segments.Num() ? FString::Printf(TEXT("%d timed climbs, in orange: %s"), Pawn->Segments.Num(), *Names)
							 : FString(TEXT("No timed climbs on this route."))));
}

void URideMenu::Refresh()
{
	Survey();
	const ARidePawn* Pawn = Rider();
	if (Pawn)
	{
		if (const FLinearColor Now = Pawn->Profile.Accent(); !Now.Equals(Accent))
		{
			Recolour(Now);
		}
		if (Pawn->Ftp != WorkoutsFtp)
		{
			ReadWorkouts(Pawn->Ftp);
		}
		if (Cyclers.IsValidIndex(ThemeCycler))
		{
			Cyclers[ThemeCycler].Choices = UnlockedThemes(Pawn->Profile.Xp); // a ride may have unlocked more
		}
		const Veloverse::FLevel L = Veloverse::LevelFor(Pawn->Profile.Xp);
		StartLevel->SetText(FText::FromString(
			L.Needed > 0 ? FString::Printf(TEXT("Level %d · %d / %d XP"), L.Level, L.Into, L.Needed)
						 : FString::Printf(TEXT("Level %d"), L.Level)));
	}
	if (CourseKm > 0.f && Pawn && Pawn->Track)
	{
		const URideTrack& Track = *Pawn->Track;
		const FString Level = FPackageName::GetShortName(
			FPackageName::GetLongPackagePath(Track.GetOutermost()->GetName()).LeftChop(FString(TEXT("/Ride")).Len()));
		CourseTitle->SetText(FText::FromString(Track.Title.IsEmpty() ? Level : Track.Title));
		const FString Kind = Track.bClosed ? TEXT("a loop") : TEXT("there and back");
		CourseText->SetText(FText::FromString(FString::Printf(
			TEXT("%.1f km, %s, with %.0f m of climbing. %s%sThe weather changes as you go."), CourseKm, *Kind,
			ClimbMetres, *Track.Blurb, Track.Blurb.IsEmpty() ? TEXT("") : TEXT(" "))));
		RoutePreview->SetStart(Pawn->StartMetres * 100.f / FMath::Max(Track.Length(), 1.f));
	}
	if (Pawn && Cyclers.IsValidIndex(RouteCycler))
	{
		FCycler& Route = Cyclers[RouteCycler];
		if (Route.Choices.Num() != Pawn->Routes.Num())
		{
			Route.Choices.Reset();
			for (const URideTrack* Each : Pawn->Routes)
			{
				Route.Choices.Add(Each->Title.IsEmpty() ? TEXT("Default") : Each->Title);
			}
			RouteRow->SetVisibility(Route.Choices.Num() > 1 ? ESlateVisibility::Visible : ESlateVisibility::Collapsed);
		}
	}
	if (Pawn && Cyclers.IsValidIndex(StartCycler) && StartsOf.Get() != Pawn->Track)
	{
		// "Coast · 5 km": a loop passes some places more than once.
		StartsOf = Pawn->Track;
		FCycler& Start = Cyclers[StartCycler];
		Start.Choices.Reset();
		for (const ARidePawn::FStart& Each : Pawn->Starts())
		{
			Start.Choices.Add(FString::Printf(TEXT("%s · %.0f km"), *Each.Name, Each.Metres / 1000.f));
		}
	}
	if (Pawn)
	{
		const bool bSim = Pawn->TrainerSource == TEXT("sim");
		TrainerNote->SetText(FText::FromString(
			bSim ? TEXT("A trainer played from the keyboard's watts, through the same Bluetooth code a real "
		                "one will use. It takes the grade and sends back power, cadence and heart rate.")
				 : TEXT("Hold W or Up to push harder, S or Down to ease off. Bluetooth trainers are next.")));
		PauseTitle->SetText(FText::FromString(Pawn->bFinished ? TEXT("Finished") : TEXT("Paused")));
		const float Km = static_cast<float>(Pawn->Ridden() / 100000.0);
		const float Average = Pawn->Elapsed > 0.f ? Km / (Pawn->Elapsed / 3600.f) : 0.f;
		PauseStats->SetText(FText::FromString(FString::Printf(TEXT("%.2f of %.1f km in %s, averaging %.1f km/h"), Km,
		                                                      CourseKm, *Clock(Pawn->Elapsed), Average)));
		ResumeButton->SetVisibility(Pawn->bFinished ? ESlateVisibility::Collapsed : ESlateVisibility::Visible);
		EndButton->SetVisibility(Pawn->bFinished ? ESlateVisibility::Collapsed : ESlateVisibility::Visible);
		FirstButton[static_cast<int32>(ERideMenuPage::Pause)] = Pawn->bFinished ? nullptr : ResumeButton.Get();
	}
	for (FCycler& Item : Cyclers)
	{
		const int32 I = Item.Get();
		Item.Value->SetText(FText::FromString(Item.Choices.IsValidIndex(I) ? Item.Choices[I] : TEXT("Custom")));
	}
}

void URideMenu::Recolour(FLinearColor Now)
{
	Accent = Now;
	for (UTextBlock* Each : AccentTexts)
	{
		Each->SetColorAndOpacity(FSlateColor(Accent));
	}
	for (URideButton* Each : PrimaryButtons)
	{
		Each->SetStyle(ButtonStyle(Accent, FLinearColor()));
	}
	RoutePreview->SetAccent(Accent);
}

void URideMenu::ReadWorkouts(float RiderFtp)
{
	WorkoutsFtp = RiderFtp;
	Workouts = ARidePawn::WorkoutLibrary(RiderFtp);
	TArray<FString>& Names = Cyclers[WorkoutCycler].Choices;
	Names = {TEXT("None, just ride")};
	for (const Veloverse::FWorkout& W : Workouts)
	{
		Names.Add(FString::Printf(TEXT("%hs, %.0f min, TSS %.0f"), W.Name.c_str(), W.Seconds() / 60.0, W.Tss()));
	}
}

TArray<FString> URideMenu::UnlockedThemes(int64 Xp)
{
	TArray<FString> Out;
	for (const Veloverse::FTheme& T : Veloverse::Themes())
	{
		if (Veloverse::IsUnlocked(T, Xp))
		{
			Out.Add(UTF8_TO_TCHAR(T.Name));
		}
	}
	return Out;
}

void URideMenu::Cycle(int32 Index, int32 Step)
{
	FCycler& Item = Cyclers[Index];
	const int32 Count = Item.Choices.Num();
	if (Count == 0)
	{
		return;
	}
	const int32 I = Item.Get();
	Item.Set(Item.Choices.IsValidIndex(I) ? (I + Step + Count) % Count : 0);
	Refresh();
}
