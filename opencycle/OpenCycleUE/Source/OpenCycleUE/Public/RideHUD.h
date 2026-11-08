// Puts the ride's overlay (URideOverlay) on the screen. The overlay paints itself, so there is
// nothing to draw here.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/HUD.h"
#include "RideHUD.generated.h"

class URideOverlay;

UCLASS()
class OPENCYCLEUE_API ARideHUD : public AHUD
{
	GENERATED_BODY()

protected:
	virtual void BeginPlay() override;

private:
	UPROPERTY(Transient) TObjectPtr<URideOverlay> Overlay;
};
