#include "RideGameMode.h"

#include "RideHUD.h"
#include "RidePawn.h"
#include "RidePlayerController.h"

ARideGameMode::ARideGameMode()
{
	DefaultPawnClass = ARidePawn::StaticClass();
	HUDClass = ARideHUD::StaticClass();
	PlayerControllerClass = ARidePlayerController::StaticClass();
}
