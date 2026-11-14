#include "RiderAnim.h"

#include "AnimationRuntime.h"
#include "Animation/AnimNodeBase.h"
#include "Animation/AnimSequence.h"
#include "Animation/AttributesRuntime.h"

void FRiderAnimProxy::PreUpdate(UAnimInstance* Instance, float DeltaSeconds)
{
	FAnimInstanceProxy::PreUpdate(Instance, DeltaSeconds);
	const URiderAnim* Rider = CastChecked<URiderAnim>(Instance);
	Takes.Reset();
	for (const UAnimSequence* Take : Rider->Takes)
	{
		if (Take)
		{
			Takes.Add(Take);
		}
	}
	Time = Rider->Time;
	Effort = Rider->Effort;
}

bool FRiderAnimProxy::Evaluate(FPoseContext& Output)
{
	if (Takes.IsEmpty())
	{
		Output.ResetToRefPose();
		return true;
	}
	const int32 Last = Takes.Num() - 1;
	const float At = FMath::Clamp(Effort, 0.f, float(Last));
	const int32 Low = FMath::Min(FMath::FloorToInt32(At), FMath::Max(Last - 1, 0));
	const float High = At - float(Low); // the weight of the next effort up
	const FAnimExtractContext Context(double(Time));
	FAnimationPoseData Out(Output);
	Takes[Low]->GetAnimationPose(Out, Context);
	if (Low == Last || High < 0.01f)
	{
		return true;
	}
	FCompactPose Pose;
	FBlendedCurve Curve;
	UE::Anim::FStackAttributeContainer Attributes;
	Pose.SetBoneContainer(&Output.Pose.GetBoneContainer());
	Curve.InitFrom(Output.Curve);
	FAnimationPoseData Next = {Pose, Curve, Attributes};
	Takes[Low + 1]->GetAnimationPose(Next, Context);
	FAnimationRuntime::BlendTwoPosesTogetherInPlace(Out, Next, 1.f - High);
	return true;
}
