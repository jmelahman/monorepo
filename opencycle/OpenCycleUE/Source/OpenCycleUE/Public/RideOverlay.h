// The HUD over the ride, painted in code to the layout in the design canvas (a 1280x720 frame,
// scaled to the screen's height). Top left: power, its zone and the zone bar. Top right: one
// panel with the road ahead, or the climb you're on, its grade coloured yellow to red, and at its
// foot the stretch you're in and the loop; the weather under it. Bottom middle: the ride so far.
#pragma once

#include "CoreMinimal.h"
#include "Blueprint/UserWidget.h"
#include "RideOverlay.generated.h"

class ARidePawn;
class URideTrack;

UCLASS()
class OPENCYCLEUE_API URideOverlay : public UUserWidget
{
	GENERATED_BODY()

protected:
	virtual TSharedRef<SWidget> RebuildWidget() override;
	virtual void NativeTick(const FGeometry& Geometry, float DeltaTime) override;
	virtual int32 NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
	                          FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
	                          bool bParentEnabled) const override;

private:
	// The road every SampleM metres: height in metres, and the grade over the 50 m round it.
	static constexpr float SampleM = 10.f;
	TArray<float> Heights;
	TArray<float> Grades;
	struct FClimb
	{
		float Start = 0.f, Top = 0.f; // metres along the track
		float Rise = 0.f;             // metres
		float Steepest = 0.f;         // rise over run
	};
	TArray<FClimb> Climbs;
	float LengthM = 0.f;
	bool bClosed = false;
	TWeakObjectPtr<const URideTrack> Surveyed;

	// Power as shown, eased over about a second so the number and the zone don't flicker.
	float Watts = 0.f;

	const ARidePawn* Rider() const;
	void Survey(const URideTrack& Track);
	float HeightAt(float Metres) const;
	float GradeAt(float Metres) const;
	// The climb at Metres, or the next one within Reach, with its start and top unwrapped past Here.
	const FClimb* ClimbAt(float Metres, bool& bOn, float& Start, float& Top, float Reach) const;
};
