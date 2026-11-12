// The start screen, the pause menu, the video settings, the rider's profile and history, and the
// ride's summary: one UMG widget built in code, so it
// needs no Blueprint asset, with a page for each. ARidePlayerController opens and closes it.
#pragma once

#include "CoreMinimal.h"
#include "Blueprint/UserWidget.h"
#include "Components/Button.h"
#include "Workout.h"
#include "RideMenu.generated.h"

class ARidePawn;
class ARidePlayerController;
class UTextBlock;
class UVerticalBox;
class UWidgetSwitcher;
class URideTrack;
class URideChart;

// A button that runs a native function when clicked, so the menu needs no UFUNCTION per button.
UCLASS()
class VELOVERSEUE_API URideButton : public UButton
{
	GENERATED_BODY()

public:
	TFunction<void()> Action;
	void Bind();

private:
	UFUNCTION()
	void Fire();
};

enum class ERideMenuPage : uint8
{
	Start,
	Pause,
	Video,
	Profile, // FTP and weight, and the rides so far
	Summary, // the ride just ended
};

UCLASS()
class VELOVERSEUE_API URideMenu : public UUserWidget
{
	GENERATED_BODY()

public:
	void Show(ERideMenuPage Page);
	ERideMenuPage Page() const { return Current; }

protected:
	virtual void NativeOnInitialized() override;
	virtual TSharedRef<SWidget> RebuildWidget() override;
	virtual void NativeTick(const FGeometry& Geometry, float DeltaTime) override;
	virtual FReply NativeOnKeyDown(const FGeometry& Geometry, const FKeyEvent& Event) override;

private:
	// A setting shown as "Label   < Value >": an index into its choices, read and written through
	// Get and Set. Get returns -1 for a value none of the choices match ("Custom").
	struct FCycler
	{
		TArray<FString> Choices;
		TFunction<int32()> Get;
		TFunction<void(int32)> Set;
		UTextBlock* Value = nullptr;
	};
	TArray<FCycler> Cyclers;

	UPROPERTY(Transient) TObjectPtr<UWidgetSwitcher> Pages;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> CourseTitle;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> CourseText;
	UPROPERTY(Transient) TObjectPtr<UWidget> RouteRow;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> TrainerNote;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> PauseTitle;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> PauseStats;
	UPROPERTY(Transient) TObjectPtr<URideButton> ResumeButton;
	UPROPERTY(Transient) TObjectPtr<URideButton> EndButton;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummaryTitle;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummaryRide;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummaryPower;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummaryBest;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummarySaved;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> SummaryXp;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> ProfileLevel;
	UPROPERTY(Transient) TObjectPtr<URideChart> SummaryChart;
	UPROPERTY(Transient) TObjectPtr<UTextBlock> HistoryText;
	UPROPERTY(Transient) TArray<TObjectPtr<URideButton>> FirstButton; // focused when a page opens

	ERideMenuPage Current = ERideMenuPage::Start;
	ERideMenuPage Back = ERideMenuPage::Start; // where the video page returns to
	float CourseKm = 0.f, ClimbMetres = 0.f;
	TWeakObjectPtr<const URideTrack> Surveyed;
	int32 RouteCycler = -1; // its choices are the rider's routes, filled in once they're read
	TArray<Veloverse::FWorkout> Workouts; // the library, for the start page's workout choice
	int32 StartCycler = -1; // and these the route's stretches, refilled when the route changes
	TWeakObjectPtr<const URideTrack> StartsOf;
	float RefreshClock = 0.f;

	ARidePlayerController* Controller() const;
	ARidePawn* Rider() const;

	void Build();
	void BuildStart();
	void BuildPause();
	void BuildVideo();
	void BuildProfile();
	void BuildSummary();
	void ShowHistory();
	void ShowSummary();
	void Refresh();
	void Survey();
	void Cycle(int32 Index, int32 Step);

	UTextBlock* Text(const FString& String, int32 Size, FLinearColor Colour = FLinearColor::White, bool bBold = false);
	URideButton* Button(const FString& Label, TFunction<void()> Action, bool bPrimary = false);
	UWidget* Cycler(const FString& Label, TArray<FString> Choices, TFunction<int32()> Get, TFunction<void(int32)> Set);
	UVerticalBox* Page(const FString& Title);
	static void Add(UVerticalBox* Box, UWidget* Widget, float Above);
};
