#include "RidePlayerController.h"

#include "Engine/Engine.h"
#include "Engine/GameViewportClient.h"
#include "GameFramework/GameUserSettings.h"
#include "Kismet/GameplayStatics.h"
#include "Kismet/KismetSystemLibrary.h"
#include "Misc/CommandLine.h"
#include "Misc/ConfigCacheIni.h"
#include "DynamicRHI.h"
#include "RHIGlobals.h"
#include "RHIStats.h"
#include "TimerManager.h"
#include "UnrealClient.h"
#include "RideMenu.h"
#include "RidePawn.h"
#include "RideTrack.h"
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
	DetectQualityOnFirstLaunch();
	UseCursor();
	ShowStart();
	if (FParse::Value(FCommandLine::Get(), TEXT("RideTour="), TourDir))
	{
		FParse::Value(FCommandLine::Get(), TEXT("RideTourRide="), TourRide);
		FParse::Value(FCommandLine::Get(), TEXT("RideTourRoute="), TourRoute);
		GetWorldTimerManager().SetTimer(TourTimer, this, &ARidePlayerController::Tour, 2.f, true);
	}
}

void ARidePlayerController::UseCursor()
{
	// Our arrow, in the HUD's off-white with a dark edge, for the system's: a hardware cursor, so it
	// moves at the mouse's rate however low the frame rate. Content/Menu/Cursors/Arrow.png and
	// Arrow@2x.png (VeloverseUE/Cursors, staged by scripts/package_snapshot.sh); the engine picks the
	// size nearest the screen's scale. Without them the system's arrow stays.
	UGameViewportClient* Viewport = GEngine ? GEngine->GameViewport : nullptr;
	const bool bSet = Viewport && Viewport->SetHardwareCursor(EMouseCursor::Default, TEXT("Menu/Cursors/Arrow"), FVector2D(1.5 / 24.0, 1.5 / 24.0));
	UE_LOG(LogTemp, Display, TEXT("RIDE cursor: %s"), bSet ? TEXT("ours") : TEXT("the system's"));
}

bool ARidePlayerController::TourChangeRoute()
{
	ARidePawn* Pawn = Rider();
	if (TourRoute < 0 || TourChange > 2 || !Pawn || Pawn->Elapsed < TourRide * 0.5f)
	{
		return false;
	}
	if (TourChange == 0)
	{
		ShowRoutes(); // as the dock's Change route
	}
	else if (TourChange == 1)
	{
		const FString File = TourDir / TEXT("menu_8_change.png");
		FScreenshotRequest::RequestScreenshot(File, true, false);
		UE_LOG(LogTemp, Display, TEXT("RIDE shot %s"), *File);
	}
	else
	{
		const FString Was = Pawn->Track ? Pawn->Track->Title : FString();
		Pawn->UseRoute(TourRoute);
		Resume();
		UE_LOG(LogTemp, Display, TEXT("RIDE tour changed route at %.0f s: %s to %s (%s)"), Pawn->Elapsed, *Was,
		       Pawn->Track ? *Pawn->Track->Title : TEXT("none"), *Pawn->RideRoutes);
	}
	++TourChange;
	return true;
}

void ARidePlayerController::DetectQualityOnFirstLaunch()
{
	// The first launch picks the graphics quality from the GPU, so a laptop doesn't start on Epic: an
	// integrated GPU starts on Low, a dedicated one by its memory. Not the engine's hardware benchmark:
	// its GPU test lost the device (VK_ERROR_DEVICE_LOST) on an AMD Phoenix iGPU under RADV and took the
	// game down before the menu. A rider who has saved their own settings keeps them, and the choice is
	// saved, so it runs once.
	UGameUserSettings* Settings = UGameUserSettings::GetGameUserSettings();
	int32 Saved = 0;
	if (!Settings || GConfig->GetInt(TEXT("ScalabilityGroups"), TEXT("sg.ShadowQuality"), Saved, GGameUserSettingsIni))
	{
		return;
	}
	FTextureMemoryStats Memory;
	RHIGetTextureMemoryStats(Memory);
	const double Gb = double(Memory.DedicatedVideoMemory) / (1024.0 * 1024.0 * 1024.0);
	const int32 Level = GRHIDeviceIsIntegrated ? 0 : Gb < 5.0 ? 1 : Gb < 9.0 ? 2 : 3;
	Settings->SetOverallScalabilityLevel(Level);
	Settings->ApplySettings(false);
	UE_LOG(LogTemp, Display, TEXT("RIDE quality detected: %s, %s, %.1f GB, overall %d"), *GRHIAdapterName,
	       GRHIDeviceIsIntegrated ? TEXT("integrated") : TEXT("dedicated"), Gb, Level);
}

void ARidePlayerController::Tour()
{
	// Every other step screenshots the page, which lands a frame or two later, and the next moves on.
	const TCHAR* Shots[] = {TEXT("start"), TEXT("routes"), TEXT("sensors"), TEXT("graphics"), TEXT("world"),
		TEXT("riding"), TEXT("rider"), TEXT("stats"), TEXT("ride"), TEXT("pause"), TEXT("summary")};
	constexpr int32 Riding = 8;
	const int32 Page = TourStep / 2;
	if (TourStep % 2 == 0)
	{
		// The ride is shot near its end, once there's some of it to summarize.
		const ARidePawn* Pawn = Rider();
		if (Page == Riding && Pawn && Pawn->Elapsed < TourRide)
		{
			TourChangeRoute();
			return;
		}
		// The start page waits for the world's light and textures to settle, as a rider would see it.
		float Settle = 0.f;
		FParse::Value(FCommandLine::Get(), TEXT("RideTourSettle="), Settle);
		if (Page == 0 && GetWorld()->GetRealTimeSeconds() < Settle)
		{
			return;
		}
		++TourStep;
		if (Page < UE_ARRAY_COUNT(Shots))
		{
			const FString File = TourDir / FString::Printf(TEXT("menu_%d_%s.png"), Page, Shots[Page]);
			FScreenshotRequest::RequestScreenshot(File, true, false);
			UE_LOG(LogTemp, Display, TEXT("RIDE shot %s"), *File);
		}
		return;
	}
	++TourStep;
	switch (Page)
	{
	case 0: Open(ERideMenuPage::Routes); break;
	case 1: Open(ERideMenuPage::Sensors); break;
	case 2: Open(ERideMenuPage::Settings); Menu->SetTab(ERideSettingsTab::Graphics); break;
	case 3: Menu->SetTab(ERideSettingsTab::World); break;
	case 4: Menu->SetTab(ERideSettingsTab::Ride); break;
	case 5: Menu->SetTab(ERideSettingsTab::Rider); break;
	case 6: Open(ERideMenuPage::Stats); break;
	case 7: Ride(); break;
	case 8: ShowPause(); break;
	case 9: EndRide(); break;
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
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->bPaused = false;
	}
	RideInput();
}

void ARidePlayerController::RideInput()
{
	const ARidePawn* Pawn = Rider();
	if (!Pawn || !Pawn->bStarted || Pawn->bFreeLook || ARidePawn::Automated())
	{
		SetShowMouseCursor(false);
		SetInputMode(FInputModeGameOnly());
		return;
	}
	// The cursor stays for the dock, and the game still reads the keys and a held button's drag.
	SetShowMouseCursor(true);
	FInputModeGameAndUI Mode;
	Mode.SetLockMouseToViewportBehavior(EMouseLockMode::DoNotLock);
	Mode.SetHideCursorDuringCapture(false);
	SetInputMode(Mode);
}

void ARidePlayerController::ShowStart() { Open(ERideMenuPage::Start); }

void ARidePlayerController::ShowPause() { Open(ERideMenuPage::Pause); }

void ARidePlayerController::ShowRoutes() { Open(ERideMenuPage::Routes); }

void ARidePlayerController::Ride()
{
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->bStarted = true;
	}
	Close();
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->Dock.Moved(); // shown at the off, so the rider learns it's there
	}
}

void ARidePlayerController::Resume() { Close(); }

void ARidePlayerController::EndRide()
{
	if (ARidePawn* Pawn = Rider())
	{
		Pawn->EndRide();
	}
	ShowSummary();
}

void ARidePlayerController::ShowSummary() { Open(ERideMenuPage::Summary); }

void ARidePlayerController::Restart()
{
	UGameplayStatics::OpenLevel(this, FName(*UGameplayStatics::GetCurrentLevelName(this)));
}

void ARidePlayerController::Quit() { UKismetSystemLibrary::QuitGame(this, this, EQuitPreference::Quit, false); }
