// The start screen, the pause menu and the video settings: one UMG widget built in code, so it
// needs no Blueprint asset, with a page for each. ARidePlayerController opens and closes it.
#pragma once

#include "CoreMinimal.h"
#include "Blueprint/UserWidget.h"
#include "Components/Button.h"
#include "RideMenu.generated.h"

class ARidePawn;
class ARidePlayerController;
class UTextBlock;
class UVerticalBox;
class UWidgetSwitcher;
class URideTrack;

// A button that runs a native function when clicked, so the menu needs no UFUNCTION per button.
UCLASS()
class OPENCYCLEUE_API URideButton : public UButton
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
};

UCLASS()
class OPENCYCLEUE_API URideMenu : public UUserWidget
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
	UPROPERTY(Transient) TArray<TObjectPtr<URideButton>> FirstButton; // focused when a page opens

	ERideMenuPage Current = ERideMenuPage::Start;
	ERideMenuPage Back = ERideMenuPage::Start; // where the video page returns to
	float CourseKm = 0.f, ClimbMetres = 0.f;
	TWeakObjectPtr<const URideTrack> Surveyed;
	int32 RouteCycler = -1; // its choices are the rider's routes, filled in once they're read
	float RefreshClock = 0.f;

	ARidePlayerController* Controller() const;
	ARidePawn* Rider() const;

	void Build();
	void BuildStart();
	void BuildPause();
	void BuildVideo();
	void Refresh();
	void Survey();
	void Cycle(int32 Index, int32 Step);

	UTextBlock* Text(const FString& String, int32 Size, FLinearColor Colour = FLinearColor::White, bool bBold = false);
	URideButton* Button(const FString& Label, TFunction<void()> Action, bool bPrimary = false);
	UWidget* Cycler(const FString& Label, TArray<FString> Choices, TFunction<int32()> Get, TFunction<void(int32)> Set);
	UVerticalBox* Page(const FString& Title);
	static void Add(UVerticalBox* Box, UWidget* Widget, float Above);
};
