#pragma once

#include "CoreMinimal.h"
#include "GameFramework/GameModeBase.h"
#include "RideGameMode.generated.h"

UCLASS()
class OPENCYCLEUE_API ARideGameMode : public AGameModeBase
{
	GENERATED_BODY()

public:
	ARideGameMode();
	virtual void StartPlay() override;

	// The scatter that thins with Foliage quality: instanced meshes given a draw distance (grass, ferns,
	// litter, small props). Trees, buildings and walls draw to the horizon and never thin, so a lower
	// setting keeps the shape of the land and loses only what is underfoot.
	static void ThinScatterWithQuality(UWorld* World);
};
