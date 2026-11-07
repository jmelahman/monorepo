// A plain readout over the ride until the real HUD is built in UMG.
#pragma once

#include "CoreMinimal.h"
#include "GameFramework/HUD.h"
#include "RideHUD.generated.h"

UCLASS()
class OPENCYCLEUE_API ARideHUD : public AHUD
{
	GENERATED_BODY()

public:
	virtual void DrawHUD() override;

private:
	// The course drawn once, in 0..1 across each panel: its plan seen from above, and its height
	// against distance.
	TArray<FVector2D> Plan;
	TArray<FVector2D> Profile;
	float LowestMetres = 0.f, HighestMetres = 0.f;
	void Survey(const class URideTrack& Track);
	void DrawMap(const class ARidePawn& Rider, float Scale);
	void DrawProfile(const class ARidePawn& Rider, float Scale);
	void Polyline(const TArray<FVector2D>& Points, FVector2D Origin, FVector2D Size, FLinearColor Colour, float Thickness);
};
