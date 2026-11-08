#include "RidePlayerController.h"

#include "Kismet/GameplayStatics.h"
#include "Kismet/KismetSystemLibrary.h"
#include "Misc/CommandLine.h"
#include "TimerManager.h"
#include "UnrealClient.h"
#include "RideMenu.h"
#include "RidePawn.h"
#include "Scalability.h"

ARidePawn* ARidePlayerController::Rider() const { return Cast<ARidePawn>(GetPawn()); }

void ARidePlayerController::OnPossess(APawn* InPawn)
{
	Super::OnPossess(InPawn);
	if (!IsLocalController())
	{
		return;
	}
	if (ARidePawn::Automated())
	{
		// Tests and stills look the same whatever a player chose: Epic, without saving it, unless
		// -RideQuality=0..3 asks for another preset (scripts/pod_bench.sh rides each).
		int32 Quality = 3;
		FParse::Value(FCommandLine::Get(), TEXT("RideQuality="), Quality);
		Scalability::FQualityLevels Levels;
		Levels.SetFromSingleQualityLevel(FMath::Clamp(Quality, 0, 3));
		Scalability::SetQualityLevels(Levels);
		return;
	}
	ShowStart();
	if (FParse::Value(FCommandLine::Get(), TEXT("RideTour="), TourDir))
	{
		GetWorldTimerManager().SetTimer(TourTimer, this, &ARidePlayerController::Tour, 2.f, true);
	}
}

void ARidePlayerController::Tour()
{
	// Every other step screenshots the page, which lands a frame or two later, and the next moves on.
	const TCHAR* Shots[] = {TEXT("start"), TEXT("video"), TEXT("ride"), TEXT("pause")};
	const int32 Page = TourStep / 2;
	if (TourStep++ % 2 == 0)
	{
		if (Page < UE_ARRAY_COUNT(Shots))
		{
			const FString File = TourDir / FString::Printf(TEXT("menu_%d_%s.png"), Page, Shots[Page]);
			FScreenshotRequest::RequestScreenshot(File, true, false);
			UE_LOG(LogTemp, Display, TEXT("RIDE shot %s"), *File);
		}
		return;
	}
	switch (Page)
	{
	case 0: Open(ERideMenuPage::Video); break;
	case 1: Ride(); break;
	case 2: ShowPause(); break;
	default: Quit(); break;
	}
}

bool ARidePlayerController::InMenu() const { return Menu && Menu->IsInViewport(); }

void ARidePlayerController::Open(ERideMenuPage Page)
{
	if (!Menu)
	{
		Menu = CreateWidget<URideMenu>(this, URideMenu::StaticClass());
	}
	if (!Menu->IsInViewport())
	{
		Menu->AddToViewport(10);
	}
	Menu->Show(Page);
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->bPaused = true;
	}
	SetShowMouseCursor(true);
	FInputModeUIOnly Mode;
	Mode.SetWidgetToFocus(Menu->TakeWidget());
	Mode.SetLockMouseToViewportBehavior(EMouseLockMode::DoNotLock);
	SetInputMode(Mode);
}

void ARidePlayerController::Close()
{
	if (Menu)
	{
		Menu->RemoveFromParent();
	}
	SetShowMouseCursor(false);
	SetInputMode(FInputModeGameOnly());
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->bPaused = false;
	}
}

void ARidePlayerController::ShowStart() { Open(ERideMenuPage::Start); }

void ARidePlayerController::ShowPause() { Open(ERideMenuPage::Pause); }

void ARidePlayerController::Ride()
{
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->bStarted = true;
	}
	Close();
}

void ARidePlayerController::Resume() { Close(); }

void ARidePlayerController::Restart()
{
	UGameplayStatics::OpenLevel(this, FName(*UGameplayStatics::GetCurrentLevelName(this)));
}

void ARidePlayerController::Quit() { UKismetSystemLibrary::QuitGame(this, this, EQuitPreference::Quit, false); }
