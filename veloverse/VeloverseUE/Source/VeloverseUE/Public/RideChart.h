// The ride summary's chart, painted in code: power over the ride (smoothed over 30 s, coloured by
// zone) on the road's height, and under it a strip of the time spent in each power zone.
#pragma once

#include "CoreMinimal.h"
#include "Blueprint/UserWidget.h"
#include "RideStats.h"
#include "RideChart.generated.h"

UCLASS()
class VELOVERSEUE_API URideChart : public UUserWidget
{
	GENERATED_BODY()

public:
	void SetRide(const std::vector<Veloverse::FSample>& Samples, float Ftp);

protected:
	virtual TSharedRef<SWidget> RebuildWidget() override;
	virtual int32 NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
	                          FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
	                          bool bParentEnabled) const override;

private:
	static constexpr int32 Columns = 240;
	TArray<float> Power;     // W, a value a column
	TArray<float> Elevation; // m
	float Zones[7] = {};     // seconds
	float Ftp = 250.f;
};
