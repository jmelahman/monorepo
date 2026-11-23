// Procedural real-time cycling & biome soundscape (wind rush, paved/gravel tire hum,
// high-engagement road freehub coasting ratchet, drivetrain purr, coastal surf, and birdsong).
#pragma once

#include "CoreMinimal.h"
#include "UObject/Object.h"
#include "RideAudio.generated.h"

class UAudioComponent;
class USoundWaveProcedural;

struct FRideAudioState
{
	float Speed = 0.f;      // m/s
	float Cadence = 0.f;    // rpm (0 when coasting)
	float Watts = 0.f;
	float AltitudeM = 0.f;
	bool bOffRoad = false;
	bool bCoastal = false;
	bool bParkOrForest = false;
	bool bActive = false;   // false while paused or on start menu
	float MasterGain = 0.85f;
	float BikeGain = 1.f;
	float AmbienceGain = 1.f;
	float Rain = 0.f;       // 0..1 active rainfall intensity
	float Wet = 0.f;        // 0..1 wet road surface (tire spray hiss)
};

UCLASS()
class VELOVERSEUE_API URideAudio : public UObject
{
	GENERATED_BODY()

public:
	void Start(AActor* Owner);
	void Update(float Dt, const FRideAudioState& State);
	void Stop();

private:
	void SynthesizeChunk(int32 NumFrames, TArray<int16>& OutPcm);
	float NextRand();
	float NextPink(float& b0, float& b1, float& b2, float& b3, float& b4, float& b5, float& b6);

	UPROPERTY(Transient) TObjectPtr<USoundWaveProcedural> Wave;
	UPROPERTY(Transient) TObjectPtr<UAudioComponent> Component;

	FRideAudioState Cur;
	float SmoothSpeed = 0.f;
	float SmoothWind = 0.f;
	float SmoothCoast = 0.f;
	float SmoothPedal = 0.f;
	float SmoothSurf = 0.f;
	float SmoothBirds = 0.f;
	float SmoothOffRoad = 0.f;
	float SmoothRain = 0.f;
	float SmoothWet = 0.f;

	// Deterministic fast PRNG & stereo pink-noise filter states
	uint32 Rng = 0x5A17C391u;
	float PinkL[7] = {};
	float PinkR[7] = {};

	// 2-pole SVF states (wind L/R, tire, surf L/R, gravel, rain L/R)
	float WindLpL = 0.f, WindBpL = 0.f;
	float WindLpR = 0.f, WindBpR = 0.f;
	float TireLp = 0.f, TireBp = 0.f;
	float SurfLpL = 0.f, SurfLpR = 0.f;
	float FoamBpL = 0.f, FoamBpR = 0.f;
	float GravelBp = 0.f;
	float RainLpL = 0.f, RainBpL = 0.f;
	float RainLpR = 0.f, RainBpR = 0.f;
	float DropL = 0.f, DropR = 0.f;

	// Freehub ratchet, drivetrain, surf swells, and birdsong oscillators
	double TimeSec = 0.0;
	double FreehubPhase = 0.0;
	float FreehubEnv = 0.f;
	double FreehubOsc1 = 0.0;
	double FreehubOsc2 = 0.0;
	double ChainPhase = 0.0;

	// Birdsong motif state
	float NextBirdSec = 2.8f;
	float BirdTime = 0.f;
	float BirdDur = 0.f;
	float BirdBaseHz = 2800.f;
	float BirdSweepHz = 700.f;
	float BirdPan = 0.5f;
	int32 BirdNotesLeft = 0;
	double BirdPhase = 0.0;

	TArray<int16> ScratchPcm;
};
