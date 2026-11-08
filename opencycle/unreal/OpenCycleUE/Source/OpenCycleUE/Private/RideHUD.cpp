#include "RideHUD.h"

#include "RideOverlay.h"

void ARideHUD::BeginPlay()
{
	Super::BeginPlay();
	if (APlayerController* Player = GetOwningPlayerController())
	{
		Overlay = CreateWidget<URideOverlay>(Player, URideOverlay::StaticClass());
		Overlay->AddToViewport(0); // under the menus
	}
}
