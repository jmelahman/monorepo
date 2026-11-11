// Opens the menus over the ride and hands input between them and the rider. A ride starts on the
// start screen unless it is automated from the command line (ARidePawn::Automated).
//
// -RideTour=DIR shows the menus unattended: the start screen, the video settings, a few seconds of
// riding and the pause menu, each saved with its screenshot into DIR as menu_*.png, then quits.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/PlayerController.h"
#include "RidePlayerController.generated.h"

class ARidePawn;
class URideMenu;
enum class ERideMenuPage : uint8;

UCLASS()
class VELOVERSEUE_API ARidePlayerController : public APlayerController
{
	GENERATED_BODY()

public:
	void ShowStart();
	void ShowPause();
	void Ride();    // from the start screen
	void Resume();  // from the pause menu
	void Restart(); // back to the start screen, the ride reset
	void Quit();
	bool InMenu() const;
	ARidePawn* Rider() const;

protected:
	virtual void OnPossess(APawn* InPawn) override;

private:
	UPROPERTY(Transient) TObjectPtr<URideMenu> Menu;
	FString TourDir;
	int32 TourStep = 0;
	FTimerHandle TourTimer;
	void Tour();
	void Open(ERideMenuPage Page);
	void Close();
};
