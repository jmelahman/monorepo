#include "RideAudio.h"

#include "Components/AudioComponent.h"
#include "Sound/SoundWaveProcedural.h"

namespace
{
constexpr int32 SampleRate = 44100;
constexpr int32 NumChannels = 2;
constexpr float InvSampleRate = 1.f / static_cast<float>(SampleRate);
constexpr float TwoPi = 6.28318530718f;
}

void URideAudio::Start(AActor* Owner)
{
	if (!Owner || Component)
	{
		return;
	}
	Wave = NewObject<USoundWaveProcedural>(this);
	Wave->SetSampleRate(SampleRate);
	Wave->NumChannels = NumChannels;
	Wave->Duration = INDEFINITELY_LOOPING_DURATION;
	Wave->SoundGroup = SOUNDGROUP_Default;
	Wave->bLooping = false;

	Component = NewObject<UAudioComponent>(Owner);
	Component->bAutoActivate = false;
	Component->bAllowSpatialization = false;
	Component->bIsUISound = true;
	Component->SetSound(Wave);
	Component->RegisterComponent();
	Component->Play();
}

void URideAudio::Stop()
{
	if (Component)
	{
		Component->Stop();
		Component->DestroyComponent();
		Component = nullptr;
	}
	Wave = nullptr;
}

float URideAudio::NextRand()
{
	// Xorshift32 -> [-1, 1]
	Rng ^= Rng << 13;
	Rng ^= Rng >> 17;
	Rng ^= Rng << 5;
	return (static_cast<float>(Rng & 0x00FFFFFFu) / 8388608.f) - 1.f;
}

float URideAudio::NextPink(float& b0, float& b1, float& b2, float& b3, float& b4, float& b5, float& b6)
{
	const float w = NextRand();
	b0 = 0.99886f * b0 + w * 0.0555179f;
	b1 = 0.99332f * b1 + w * 0.0750759f;
	b2 = 0.96900f * b2 + w * 0.1538520f;
	b3 = 0.86650f * b3 + w * 0.3104856f;
	b4 = 0.55000f * b4 + w * 0.5329522f;
	b5 = -0.7616f * b5 - w * 0.0168980f;
	const float out = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362f) * 0.11f;
	b6 = w * 0.115926f;
	return out;
}

void URideAudio::Update(float Dt, const FRideAudioState& State)
{
	Cur = State;
	if (!Wave || !Component)
	{
		return;
	}
	if (!Component->IsPlaying())
	{
		Component->Play();
	}

	// Keep ~100 ms buffered ahead of the audio thread so frame hitches never cause clicks.
	const int32 BytesPerFrame = NumChannels * static_cast<int32>(sizeof(int16));
	const int32 TargetBytes = (SampleRate / 10) * BytesPerFrame; // 100 ms
	const int32 Queued = Wave->GetAvailableAudioByteCount();
	if (Queued >= TargetBytes)
	{
		return;
	}

	const int32 NeedFrames = FMath::Clamp((TargetBytes - Queued) / BytesPerFrame, 256, SampleRate / 8);
	SynthesizeChunk(NeedFrames, ScratchPcm);
	Wave->QueueAudio(reinterpret_cast<const uint8*>(ScratchPcm.GetData()),
	                 ScratchPcm.Num() * static_cast<int32>(sizeof(int16)));
}

void URideAudio::SynthesizeChunk(int32 NumFrames, TArray<int16>& OutPcm)
{
	OutPcm.SetNumUninitialized(NumFrames * NumChannels);

	const float TargetActive = Cur.bActive ? 1.f : 0.f;
	const float TargetSpeed = Cur.bActive ? FMath::Clamp(Cur.Speed, 0.f, 25.f) : 0.f;
	const bool bCoasting = Cur.bActive && TargetSpeed > 2.0f && Cur.Cadence < 10.f;
	const bool bPedaling = Cur.bActive && Cur.Cadence >= 10.f && TargetSpeed > 0.3f;
	const float TargetCoast = bCoasting ? 1.f : 0.f;
	const float TargetPedal = bPedaling ? FMath::Clamp(Cur.Watts / 220.f, 0.25f, 1.4f) : 0.f;
	const float TargetSurf = (Cur.bActive && Cur.bCoastal)
	                             ? FMath::Clamp(1.f - Cur.AltitudeM / 55.f, 0.15f, 1.f)
	                             : 0.f;
	const float TargetBirds = (Cur.bActive && Cur.bParkOrForest)
	                              ? FMath::Clamp(1.f - (TargetSpeed - 6.f) / 10.f, 0.15f, 1.f)
	                              : 0.f;
	const float TargetOffRoad = Cur.bOffRoad ? 1.f : 0.f;

	const float Slew = 1.f - FMath::Exp(-6.f * InvSampleRate);
	const float SlowSlew = 1.f - FMath::Exp(-1.5f * InvSampleRate);

	for (int32 i = 0; i < NumFrames; ++i)
	{
		TimeSec += InvSampleRate;
		SmoothSpeed += (TargetSpeed - SmoothSpeed) * Slew;
		SmoothCoast += (TargetCoast - SmoothCoast) * Slew;
		SmoothPedal += (TargetPedal - SmoothPedal) * Slew;
		SmoothSurf += (TargetSurf - SmoothSurf) * SlowSlew;
		SmoothBirds += (TargetBirds - SmoothBirds) * SlowSlew;
		SmoothOffRoad += (TargetOffRoad - SmoothOffRoad) * SlowSlew;
		SmoothWind += (TargetActive - SmoothWind) * SlowSlew;

		const float PL = NextPink(PinkL[0], PinkL[1], PinkL[2], PinkL[3], PinkL[4], PinkL[5], PinkL[6]);
		const float PR = NextPink(PinkR[0], PinkR[1], PinkR[2], PinkR[3], PinkR[4], PinkR[5], PinkR[6]);

		// 1. Wind Rush (velocity-swept stereo bandpass pink noise + subtle gust modulation)
		const float SpeedNorm = FMath::Clamp(SmoothSpeed / 14.f, 0.f, 1.6f);
		const float Gust = 0.85f + 0.15f * FMath::Sin(static_cast<float>(TimeSec * 1.15));
		const float WindFreq = FMath::Lerp(150.f, 680.f, FMath::Clamp(SpeedNorm, 0.f, 1.f));
		const float WindF = FMath::Clamp(TwoPi * WindFreq * InvSampleRate, 0.01f, 0.45f);
		const float WindQ = 0.65f;
		const float WindHpL = PL - WindLpL - WindQ * WindBpL;
		WindBpL += WindF * WindHpL;
		WindLpL += WindF * WindBpL;
		const float WindHpR = PR - WindLpR - WindQ * WindBpR;
		WindBpR += WindF * WindHpR;
		WindLpR += WindF * WindBpR;
		const float WindGain = 0.24f * SpeedNorm * SpeedNorm * Gust * SmoothWind;

		// 2. Tire Roll Hum (warm low road hum + granular gravel/dirt crunch when off-road)
		const float TireFreq = FMath::Lerp(95.f, 240.f, FMath::Clamp(SpeedNorm, 0.f, 1.f));
		const float TireF = FMath::Clamp(TwoPi * TireFreq * InvSampleRate, 0.01f, 0.2f);
		const float TireIn = 0.5f * (PL + PR);
		TireBp += TireF * (TireIn - TireLp - 0.85f * TireBp);
		TireLp += TireF * TireBp;
		const float TireGain = 0.14f * FMath::Clamp(SmoothSpeed / 8.f, 0.f, 1.2f) * SmoothWind;

		float GravelSample = 0.f;
		if (SmoothOffRoad > 0.01f && SmoothSpeed > 0.5f)
		{
			const float Imp = (NextRand() > (0.9965f - 0.002f * FMath::Min(SpeedNorm, 1.f))) ? NextRand() : 0.f;
			GravelBp = 0.82f * GravelBp + 0.18f * (Imp * 2.2f + 0.15f * PL);
			GravelSample = GravelBp * 0.22f * SmoothOffRoad * FMath::Clamp(SmoothSpeed / 7.f, 0.f, 1.2f);
		}

		// 3. High-Engagement Road Freehub Ratchet (36 clicks per 2.11 m wheel rev when coasting)
		float FreehubSample = 0.f;
		if (SmoothCoast > 0.005f)
		{
			const double ClicksPerSec = FMath::Clamp(static_cast<double>(SmoothSpeed) * (36.0 / 2.11), 8.0, 340.0);
			FreehubPhase += ClicksPerSec * InvSampleRate;
			if (FreehubPhase >= 1.0)
			{
				FreehubPhase -= 1.0;
				FreehubEnv = 1.f;
			}
			FreehubOsc1 += 2650.0 * TwoPi * InvSampleRate;
			FreehubOsc2 += 5480.0 * TwoPi * InvSampleRate;
			const float Ping = 0.65f * FMath::Sin(static_cast<float>(FreehubOsc1)) +
			                   0.35f * FMath::Sin(static_cast<float>(FreehubOsc2));
			FreehubSample = Ping * FreehubEnv * 0.065f * SmoothCoast;
			FreehubEnv *= 0.972f; // ~1.6 ms crisp metallic decay
		}

		// 4. Subtle Drivetrain Chain Purr (rhythmic pedal stroke modulation)
		float ChainSample = 0.f;
		if (SmoothPedal > 0.01f)
		{
			const double StrokeHz = FMath::Clamp(static_cast<double>(Cur.Cadence) / 30.0, 1.0, 4.5);
			ChainPhase += StrokeHz * TwoPi * InvSampleRate;
			const float Pulse = 0.6f + 0.4f * FMath::Sin(static_cast<float>(ChainPhase));
			ChainSample = TireBp * Pulse * 0.045f * SmoothPedal;
		}

		// 5. Coastal Ocean Surf Ambience (slow rolling stereo wave swells)
		float SurfL = 0.f, SurfR = 0.f;
		if (SmoothSurf > 0.005f)
		{
			const float SwellL = 0.5f + 0.5f * FMath::Sin(static_cast<float>(TimeSec * 0.78));
			const float SwellR = 0.5f + 0.5f * FMath::Sin(static_cast<float>(TimeSec * 0.61 + 1.4));
			const float WaveL = SwellL * SwellL;
			const float WaveR = SwellR * SwellR;
			SurfLpL += 0.035f * (PL - SurfLpL);
			SurfLpR += 0.035f * (PR - SurfLpR);
			FoamBpL += 0.16f * ((PL - SurfLpL) - FoamBpL);
			FoamBpR += 0.16f * ((PR - SurfLpR) - FoamBpR);
			SurfL = (SurfLpL * (0.28f + 0.45f * WaveL) + FoamBpL * 0.16f * WaveL * WaveL) * 0.34f * SmoothSurf;
			SurfR = (SurfLpR * (0.28f + 0.45f * WaveR) + FoamBpR * 0.16f * WaveR * WaveR) * 0.34f * SmoothSurf;
		}

		// 6. Lincoln Park & Greenwood Birdsong (occasional soft melodic chirps)
		float BirdL = 0.f, BirdR = 0.f;
		if (SmoothBirds > 0.02f)
		{
			if (BirdNotesLeft <= 0)
			{
				NextBirdSec -= InvSampleRate;
				if (NextBirdSec <= 0.f)
				{
					BirdNotesLeft = 2 + (static_cast<int32>(Rng) & 1);
					BirdBaseHz = 2350.f + 1150.f * (0.5f + 0.5f * NextRand());
					BirdSweepHz = 450.f + 550.f * NextRand();
					BirdPan = 0.18f + 0.64f * (0.5f + 0.5f * NextRand());
					BirdDur = 0.11f + 0.06f * (0.5f + 0.5f * NextRand());
					BirdTime = 0.f;
					NextBirdSec = 3.6f + 4.8f * (0.5f + 0.5f * NextRand());
				}
			}
			else
			{
				BirdTime += InvSampleRate;
				const float u = BirdTime / BirdDur;
				if (u < 1.f)
				{
					const float Env = FMath::Sin(u * PI) * FMath::Sin(u * PI);
					const float Hz = BirdBaseHz + BirdSweepHz * FMath::Sin(u * PI);
					BirdPhase += static_cast<double>(Hz) * TwoPi * InvSampleRate;
					const float Sig = FMath::Sin(static_cast<float>(BirdPhase)) * Env * 0.032f * SmoothBirds;
					BirdL = Sig * (1.f - BirdPan);
					BirdR = Sig * BirdPan;
				}
				else if (BirdTime > BirdDur + 0.09f)
				{
					--BirdNotesLeft;
					BirdTime = 0.f;
					BirdBaseHz *= 0.94f + 0.12f * (0.5f + 0.5f * NextRand());
				}
			}
		}

		// Master stereo mix with gentle soft-clip saturation
		const float MixL = WindBpL * WindGain + TireLp * TireGain + GravelSample + FreehubSample +
		                   ChainSample + SurfL + BirdL;
		const float MixR = WindBpR * WindGain + TireLp * TireGain + GravelSample + FreehubSample +
		                   ChainSample + SurfR + BirdR;
		const float OutL = FMath::Tanh(MixL * 1.15f);
		const float OutR = FMath::Tanh(MixR * 1.15f);

		OutPcm[i * 2 + 0] = static_cast<int16>(FMath::Clamp(OutL, -1.f, 1.f) * 32000.f);
		OutPcm[i * 2 + 1] = static_cast<int16>(FMath::Clamp(OutR, -1.f, 1.f) * 32000.f);
	}
}
