#include "RideGameMode.h"

#include "Components/HierarchicalInstancedStaticMeshComponent.h"
#include "EngineUtils.h"
#include "RideHUD.h"
#include "RidePawn.h"
#include "RidePlayerController.h"

ARideGameMode::ARideGameMode()
{
	DefaultPawnClass = ARidePawn::StaticClass();
	HUDClass = ARideHUD::StaticClass();
	PlayerControllerClass = ARidePlayerController::StaticClass();
}

void ARideGameMode::StartPlay()
{
	ThinScatterWithQuality(GetWorld());
	Super::StartPlay();
}

void ARideGameMode::ThinScatterWithQuality(UWorld* World)
{
	int32 Components = 0, Instances = 0;
	for (TObjectIterator<UHierarchicalInstancedStaticMeshComponent> It; It; ++It)
	{
		UHierarchicalInstancedStaticMeshComponent* Scatter = *It;
		if (Scatter->GetWorld() != World || Scatter->InstanceEndCullDistance <= 0)
		{
			continue;
		}
#if WITH_EDITOR
		Scatter->bCanEnableDensityScaling = true;
#endif
		Scatter->bEnableDensityScaling = true;
		// foliage.DensityScale applies from here on, and again whenever the setting changes.
		Scatter->UpdateDensityScaling();
		++Components;
		Instances += Scatter->GetInstanceCount();
	}
	UE_LOG(LogTemp, Display, TEXT("RIDE scatter thins with quality: %d components, %d instances"), Components,
	       Instances);
}
