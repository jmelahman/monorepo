#include "RideGameMode.h"

#include "Components/HierarchicalInstancedStaticMeshComponent.h"
#include "Engine/StaticMesh.h"
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

// Plants and stones thin; the kit's props share their batches (stone lanterns, paper lanterns, fences,
// vending machines) and must not: a lane on Low lost every lantern. Everything from Poly Haven is
// ground cover, and of our own kit only the tufts and hydrangeas are.
static bool IsPlant(const UHierarchicalInstancedStaticMeshComponent* Scatter)
{
	const UStaticMesh* Mesh = Scatter->GetStaticMesh();
	if (!Mesh)
	{
		return false;
	}
	const FString Path = Mesh->GetPathName();
	if (!Path.Contains(TEXT("/Gen/")))
	{
		return true;
	}
	const FString Name = Mesh->GetName();
	return Name.StartsWith(TEXT("V_tuft")) || Name.StartsWith(TEXT("V_ajisai"));
}

void ARideGameMode::ThinScatterWithQuality(UWorld* World)
{
	int32 Components = 0, Instances = 0;
	for (TObjectIterator<UHierarchicalInstancedStaticMeshComponent> It; It; ++It)
	{
		UHierarchicalInstancedStaticMeshComponent* Scatter = *It;
		if (Scatter->GetWorld() != World || Scatter->InstanceEndCullDistance <= 0 || !IsPlant(Scatter))
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
