#include "RideHUD.h"

#include "Engine/Canvas.h"
#include "Engine/Engine.h"
#include "Engine/Font.h"
#include "RidePawn.h"
#include "RideTrack.h"

void ARideHUD::DrawHUD()
{
	Super::DrawHUD();
	const ARidePawn* Rider = Cast<ARidePawn>(GetOwningPawn());
	if (!Rider || !Canvas || !Rider->bStarted)
	{
		return;
	}
	const int32 Seconds = FMath::FloorToInt32(Rider->Elapsed);
	TArray<FString> Rows = {
		FString::Printf(TEXT("%.1f km/h"), Rider->Speed * 3.6f),
		FString::Printf(TEXT("%.0f W   %.0f rpm"), Rider->Reading.Watts, Rider->Reading.CadenceRpm),
		FString::Printf(TEXT("%+.1f %%"), Rider->Grade * 100.f),
		FString::Printf(TEXT("%.2f / %.1f km"), Rider->Distance / 100000.f, Rider->TrackLength() / 100000.f),
		FString::Printf(TEXT("%d:%02d:%02d"), Seconds / 3600, Seconds / 60 % 60, Seconds % 60),
	};
	if (Rider->Reading.HeartRateBpm > 0.0)
	{
		Rows.Insert(FString::Printf(TEXT("%.0f bpm"), Rider->Reading.HeartRateBpm), 2);
	}
	UFont* Font = GEngine->GetLargeFont();
	const float Scale = Canvas->ClipY / 720.f;
	if (Rider->Track && Plan.IsEmpty())
	{
		Survey(*Rider->Track);
	}
	if (!Plan.IsEmpty())
	{
		DrawMap(*Rider, Scale);
		DrawProfile(*Rider, Scale);
	}
	const float Line = 30.f * Scale;
	float Y = Canvas->ClipY - Line * (Rows.Num() + 1.5f);
	DrawRect(FLinearColor(0.f, 0.f, 0.f, 0.35f), 24.f * Scale, Y - 10.f * Scale, 260.f * Scale,
	         Line * Rows.Num() + 20.f * Scale);
	for (const FString& Row : Rows)
	{
		DrawText(Row, FLinearColor::White, 40.f * Scale, Y, Font, Scale * 1.2f);
		Y += Line;
	}
	const FString Help = FString::Printf(TEXT("%s   W/S power   Esc menu"), *Rider->TrainerName());
	DrawText(Help, FLinearColor(1.f, 1.f, 1.f, 0.6f), 24.f * Scale, Canvas->ClipY - Line, GEngine->GetSmallFont(), Scale);
}

void ARideHUD::Survey(const URideTrack& Track)
{
	const float Length = Track.Length();
	if (Length <= 0.f)
	{
		return;
	}
	constexpr int32 Samples = 400;
	FBox2D Bounds(ForceInit);
	TArray<FVector2D> Points;
	TArray<float> Heights;
	for (int32 I = 0; I <= Samples; ++I)
	{
		const float S = Length * I / Samples;
		FVector Position;
		float Yaw;
		Track.At(FMath::Min(S, Length - 1.f), Position, Yaw);
		// Seen from above with north up: Unreal's +X is north, +Y east.
		Points.Add(FVector2D(Position.Y, -Position.X));
		Bounds += Points.Last();
		Heights.Add(Position.Z / 100.f);
	}
	const FVector2D Size = Bounds.GetSize();
	const float Span = FMath::Max(Size.X, Size.Y);
	const FVector2D Pad = (FVector2D(Span, Span) - Size) * 0.5;
	for (const FVector2D& P : Points)
	{
		Plan.Add((P - Bounds.Min + Pad) / Span);
	}
	LowestMetres = FMath::Min(Heights);
	HighestMetres = FMath::Max(Heights);
	const float Rise = FMath::Max(HighestMetres - LowestMetres, 10.f);
	for (int32 I = 0; I <= Samples; ++I)
	{
		Profile.Add(FVector2D(float(I) / Samples, 1.f - (Heights[I] - LowestMetres) / Rise));
	}
}

void ARideHUD::Polyline(const TArray<FVector2D>& Points, FVector2D Origin, FVector2D Size, FLinearColor Colour,
                        float Thickness)
{
	for (int32 I = 1; I < Points.Num(); ++I)
	{
		const FVector2D A = Origin + Points[I - 1] * Size;
		const FVector2D B = Origin + Points[I] * Size;
		DrawLine(A.X, A.Y, B.X, B.Y, Colour, Thickness);
	}
}

void ARideHUD::DrawMap(const ARidePawn& Rider, float Scale)
{
	const float Side = 170.f * Scale;
	const FVector2D Origin(Canvas->ClipX - Side - 24.f * Scale, 24.f * Scale);
	DrawRect(FLinearColor(0.f, 0.f, 0.f, 0.35f), Origin.X - 10.f * Scale, Origin.Y - 10.f * Scale, Side + 20.f * Scale,
	         Side + 20.f * Scale);
	Polyline(Plan, Origin, FVector2D(Side, Side), FLinearColor(1.f, 1.f, 1.f, 0.7f), 2.f * Scale);
	const float Along = Rider.TrackLength() > 0.f ? FMath::Fmod(Rider.Distance, Rider.TrackLength()) / Rider.TrackLength() : 0.f;
	const FVector2D Dot = Origin + Plan[FMath::Clamp(FMath::RoundToInt32(Along * (Plan.Num() - 1)), 0, Plan.Num() - 1)] * Side;
	const float R = 5.f * Scale;
	DrawRect(FLinearColor(1.f, 0.35f, 0.2f), Dot.X - R, Dot.Y - R, 2.f * R, 2.f * R);
}

void ARideHUD::DrawProfile(const ARidePawn& Rider, float Scale)
{
	const FVector2D Size(360.f * Scale, 70.f * Scale);
	const FVector2D Origin(Canvas->ClipX - Size.X - 24.f * Scale, Canvas->ClipY - Size.Y - 40.f * Scale);
	DrawRect(FLinearColor(0.f, 0.f, 0.f, 0.35f), Origin.X - 10.f * Scale, Origin.Y - 22.f * Scale, Size.X + 20.f * Scale,
	         Size.Y + 32.f * Scale);
	const float Along = Rider.TrackLength() > 0.f ? FMath::Fmod(Rider.Distance, Rider.TrackLength()) / Rider.TrackLength() : 0.f;
	// The road behind is solid, the road ahead faint.
	const int32 Here = FMath::Clamp(FMath::RoundToInt32(Along * (Profile.Num() - 1)), 0, Profile.Num() - 1);
	TArray<FVector2D> Behind(Profile.GetData(), Here + 1);
	TArray<FVector2D> Ahead(Profile.GetData() + Here, Profile.Num() - Here);
	Polyline(Ahead, Origin, Size, FLinearColor(1.f, 1.f, 1.f, 0.45f), 2.f * Scale);
	Polyline(Behind, Origin, Size, FLinearColor(1.f, 0.35f, 0.2f), 2.5f * Scale);
	const FVector2D Dot = Origin + Profile[Here] * Size;
	DrawLine(Dot.X, Origin.Y, Dot.X, Origin.Y + Size.Y, FLinearColor(1.f, 1.f, 1.f, 0.5f), 1.f);
	DrawText(FString::Printf(TEXT("%.0f - %.0f m"), LowestMetres, HighestMetres), FLinearColor(1.f, 1.f, 1.f, 0.7f),
	         Origin.X, Origin.Y - 20.f * Scale, GEngine->GetSmallFont(), Scale);
}
