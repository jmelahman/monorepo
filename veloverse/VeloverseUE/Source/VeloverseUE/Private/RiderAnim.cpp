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
	LoopLength = Rider->LoopLength;
	Effort = Rider->Effort;
}

namespace
{
// Evaluates a single 60-frame take (frames 0..59, PlayLength ~1.967s) smoothly across the full
// LoopLength (2.0s) by blending frame 59 back into frame 0 over the closing 1/60th of a turn.
void SampleLooping(const UAnimSequence* Take, float Time, float LoopLength, FAnimationPoseData& Out)
{
	const float PlayLen = Take->GetPlayLength();
	const float Wrapped = LoopLength > 0.f ? FMath::Fmod(FMath::Fmod(Time, LoopLength) + LoopLength, LoopLength) : 0.f;
	if (Wrapped <= PlayLen || LoopLength <= PlayLen + 1e-4f)
	{
		Take->GetAnimationPose(Out, FAnimExtractContext(static_cast<double>(FMath::Clamp(Wrapped, 0.f, PlayLen)), true));
		return;
	}
	const float Alpha = FMath::Clamp((Wrapped - PlayLen) / (LoopLength - PlayLen), 0.f, 1.f);
	Take->GetAnimationPose(Out, FAnimExtractContext(static_cast<double>(PlayLen), true));
	FCompactPose WrapPose;
	FBlendedCurve WrapCurve;
	UE::Anim::FStackAttributeContainer WrapAttrs;
	WrapPose.SetBoneContainer(&Out.GetPose().GetBoneContainer());
	WrapCurve.InitFrom(Out.GetCurve());
	FAnimationPoseData First = {WrapPose, WrapCurve, WrapAttrs};
	Take->GetAnimationPose(First, FAnimExtractContext(0.0, true));
	FAnimationRuntime::BlendTwoPosesTogetherInPlace(Out, First, 1.f - Alpha);
}
} // namespace

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
	FAnimationPoseData Out(Output);
	SampleLooping(Takes[Low], Time, LoopLength, Out);
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
	SampleLooping(Takes[Low + 1], Time, LoopLength, Next);
	FAnimationRuntime::BlendTwoPosesTogetherInPlace(Out, Next, 1.f - High);
	return true;
}
