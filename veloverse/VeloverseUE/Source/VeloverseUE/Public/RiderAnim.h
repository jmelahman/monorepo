#pragma once

#include "Animation/AnimInstance.h"
#include "Animation/AnimInstanceProxy.h"
#include "CoreMinimal.h"
#include "RiderAnim.generated.h"

class UAnimSequence;

// The rider's three pedal efforts (rider/README.md: Pedal_Easy, Pedal, Pedal_Hard) blended on the
// worker thread, so the rider needs no Anim Blueprint. The takes share the crank timing, so all are
// sampled at one Time and the two either side of Effort are mixed.
USTRUCT()
struct FRiderAnimProxy : public FAnimInstanceProxy
{
	GENERATED_BODY()

	FRiderAnimProxy() = default;
	explicit FRiderAnimProxy(UAnimInstance* Instance) : FAnimInstanceProxy(Instance) {}

	virtual void PreUpdate(UAnimInstance* Instance, float DeltaSeconds) override;
	virtual bool Evaluate(FPoseContext& Output) override;

	TArray<const UAnimSequence*, TInlineAllocator<3>> Takes;
	float Time = 0.f;
	float Effort = 1.f;
};

UCLASS(Transient, NotBlueprintable)
class VELOVERSEUE_API URiderAnim : public UAnimInstance
{
	GENERATED_BODY()

public:
	// Easiest first. Effort is an index into them: 0 Easy, 1 Pedal, 2 Hard, fractions between.
	UPROPERTY(Transient) TArray<TObjectPtr<UAnimSequence>> Takes;
	float Time = 0.f; // seconds into the loop
	float Effort = 1.f;

protected:
	virtual FAnimInstanceProxy* CreateAnimInstanceProxy() override { return new FRiderAnimProxy(this); }
	virtual void DestroyAnimInstanceProxy(FAnimInstanceProxy* Proxy) override { delete static_cast<FRiderAnimProxy*>(Proxy); }
};
