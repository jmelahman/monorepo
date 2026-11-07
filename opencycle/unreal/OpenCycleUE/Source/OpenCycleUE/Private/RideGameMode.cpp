#include "RideGameMode.h"

#include "RideHUD.h"
#include "RidePawn.h"

ARideGameMode::ARideGameMode()
{
	DefaultPawnClass = ARidePawn::StaticClass();
	HUDClass = ARideHUD::StaticClass();
}
