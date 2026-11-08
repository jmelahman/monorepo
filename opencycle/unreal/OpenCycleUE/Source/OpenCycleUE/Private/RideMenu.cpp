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
#include "RidePawn.h"
#include "RidePlayerController.h"
#include "RideTrack.h"
#include "Styling/CoreStyle.h"

namespace
{
// The HUD's orange.
const FLinearColor Accent(1.f, 0.42f, 0.08f);
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

URideButton* URideMenu::Button(const FString& Label, TFunction<void()> Action, bool bPrimary)
{
	URideButton* Button = WidgetTree->ConstructWidget<URideButton>();
	const FLinearColor Base = bPrimary ? Accent : FLinearColor(1.f, 1.f, 1.f, 0.08f);
	const FLinearColor Hover = bPrimary ? Accent * 1.15f : FLinearColor(1.f, 1.f, 1.f, 0.18f);
	FButtonStyle Style;
	Style.SetNormal(FSlateRoundedBoxBrush(Base, 6.f));
	Style.SetHovered(FSlateRoundedBoxBrush(Hover, 6.f));
	Style.SetPressed(FSlateRoundedBoxBrush(Base * 0.8f, 6.f));
	Style.SetDisabled(FSlateRoundedBoxBrush(FLinearColor(1.f, 1.f, 1.f, 0.03f), 6.f));
	Style.SetNormalPadding(FMargin(18.f, 10.f));
	Style.SetPressedPadding(FMargin(18.f, 11.f, 18.f, 9.f));
	Button->SetStyle(Style);
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
	FirstButton.SetNum(3);
	BuildStart();
	BuildPause();
	BuildVideo();
}

void URideMenu::BuildStart()
{
	UVerticalBox* P = Page(TEXT(""));
	Add(P, Text(TEXT("OpenCycle"), 44, FLinearColor::White, true), 0.f);
	Add(P, Text(TEXT("Coast"), 22, Accent), 0.f);
	CourseText = Text(TEXT(""), 15, Faint);
	Add(P, CourseText, 14.f);
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
	    28.f);
	TrainerNote = Text(TEXT(""), 13, Faint);
	Add(P, TrainerNote, 6.f);
	URideButton* Go = Button(TEXT("Ride"), [this] { Controller()->Ride(); }, true);
	FirstButton[static_cast<int32>(ERideMenuPage::Start)] = Go;
	Add(P, Go, 28.f);
	Add(P, Button(TEXT("Video settings"), [this] { Show(ERideMenuPage::Video); }), 10.f);
	Add(P, Button(TEXT("Quit"), [this] { Controller()->Quit(); }), 10.f);
	Add(P, Text(TEXT("W/S or Up/Down change power. Esc pauses."), 13, Faint), 24.f);
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

// ---- Showing ---------------------------------------------------------------------------------

void URideMenu::Survey()
{
	const ARidePawn* Pawn = Rider();
	if (!Pawn || !Pawn->Track || CourseKm > 0.f)
	{
		return;
	}
	const URideTrack& Track = *Pawn->Track;
	const float Length = Track.Length();
	CourseKm = Length / 100000.f;
	for (float S = 1000.f; S < Length; S += 1000.f)
	{
		ClimbMetres += FMath::Max(0.f, Track.Height(S) - Track.Height(S - 1000.f)) / 100.f;
	}
}

void URideMenu::Refresh()
{
	Survey();
	const ARidePawn* Pawn = Rider();
	if (CourseKm > 0.f)
	{
		CourseText->SetText(FText::FromString(FString::Printf(
			TEXT("%.1f km and %.0f m of climbing, keeping left: a lane through a small town and its rice "
		         "paddies, a jungle with a shrine, and a sea wall under the mountain. The weather changes "
		         "as you go."),
			CourseKm, ClimbMetres)));
	}
	if (Pawn)
	{
		const bool bSim = Pawn->TrainerSource == TEXT("sim");
		TrainerNote->SetText(FText::FromString(
			bSim ? TEXT("A trainer played from the keyboard's watts, through the same Bluetooth code a real "
		                "one will use. It takes the grade and sends back power, cadence and heart rate.")
				 : TEXT("Hold W or Up to push harder, S or Down to ease off. Bluetooth trainers are next.")));
		PauseTitle->SetText(FText::FromString(Pawn->bFinished ? TEXT("Finished") : TEXT("Paused")));
		const float Km = static_cast<float>(Pawn->Distance / 100000.0);
		const float Average = Pawn->Elapsed > 0.f ? Km / (Pawn->Elapsed / 3600.f) : 0.f;
		PauseStats->SetText(FText::FromString(FString::Printf(TEXT("%.2f of %.1f km in %s, averaging %.1f km/h"), Km,
		                                                      CourseKm, *Clock(Pawn->Elapsed), Average)));
		ResumeButton->SetVisibility(Pawn->bFinished ? ESlateVisibility::Collapsed : ESlateVisibility::Visible);
		FirstButton[static_cast<int32>(ERideMenuPage::Pause)] = Pawn->bFinished ? nullptr : ResumeButton.Get();
	}
	for (FCycler& Item : Cyclers)
	{
		const int32 I = Item.Get();
		Item.Value->SetText(FText::FromString(Item.Choices.IsValidIndex(I) ? Item.Choices[I] : TEXT("Custom")));
	}
}

void URideMenu::Cycle(int32 Index, int32 Step)
{
	FCycler& Item = Cyclers[Index];
	const int32 Count = Item.Choices.Num();
	const int32 I = Item.Get();
	Item.Set(Item.Choices.IsValidIndex(I) ? (I + Step + Count) % Count : 0);
	Refresh();
}
