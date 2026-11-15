// Opens the menus over the ride and hands input between them and the rider. A ride starts on the
// start screen unless it is automated from the command line (ARidePawn::Automated).
//
// -RideTour=DIR shows the menus unattended: the start screen, the routes, the sensors, the graphics
// and rider settings, the stats, some riding (-RideTourRide=SECONDS, 40 by default), the pause menu
// and the ride's review, each saved with its screenshot into DIR as menu_*.png, then quits.
// -RideTourRoute=N also changes route halfway through the ride, as the dock's button does: the
// routes page over the ride (menu_8_change.png), then route N, the ride going on along it.
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
	void EndRide(); // from the pause menu: saves the ride and shows its summary
	void ShowSummary();
	void ShowRoutes(); // mid-ride, from the dock: pick another route and ride on along it
	// Riding: the cursor and dock with the mouse, or free look's mouse as a head alone.
	void RideInput();
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
	float TourRide = 40.f;
	int32 TourRoute = -1;
	int32 TourChange = 0; // the route change's steps: open, shot, pick
	FTimerHandle TourTimer;
	bool TourChangeRoute();
	void Tour();
	void DetectQualityOnFirstLaunch();
	void UseCursor();
	void Open(ERideMenuPage Page);
	void Close();
};
