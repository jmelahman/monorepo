#include "RideTrack.h"

#include "Algo/BinarySearch.h"

float URideTrack::Length() const
{
	if (Centre.Num() < 2)
	{
		return 0.f;
	}
	return (bClosed ? Centre.Num() : Centre.Num() - 1) * Step;
}

float URideTrack::Wrap(float S) const
{
	const float L = Length();
	if (L <= 0.f)
	{
		return 0.f;
	}
	return bClosed ? FMath::Fmod(FMath::Fmod(S, L) + L, L) : FMath::Clamp(S, 0.f, L);
}

int32 URideTrack::Index(int32 I) const
{
	const int32 N = Centre.Num();
	return bClosed ? ((I % N) + N) % N : FMath::Clamp(I, 0, N - 1);
}

void URideTrack::At(float S, FVector& OutPosition, float& OutYaw) const
{
	if (Centre.Num() < 2)
	{
		OutPosition = Centre.Num() ? Centre[0] : FVector::ZeroVector;
		OutYaw = 0.f;
		return;
	}
	S = Wrap(S);
	const float F = S / Step;
	const int32 I = bClosed ? FMath::Min(FMath::FloorToInt32(F), Centre.Num() - 1)
	                        : FMath::Min(FMath::FloorToInt32(F), Centre.Num() - 2);
	const float T = F - I;
	OutPosition = FMath::Lerp(Centre[I], Centre[Index(I + 1)], T);
	// Yaw from points a few metres either side, as coast_layout.CoastPath.at does.
	const FVector Along = Centre[Index(I + 3)] - Centre[Index(I - 2)];
	OutYaw = FMath::RadiansToDegrees(FMath::Atan2(Along.Y, Along.X));
}

FVector URideTrack::Point(float S, float Lateral, float Up) const
{
	FVector P;
	float Yaw;
	At(S, P, Yaw);
	const float R = FMath::DegreesToRadians(Yaw);
	return FVector(P.X - FMath::Sin(R) * Lateral, P.Y + FMath::Cos(R) * Lateral, P.Z + Up);
}

float URideTrack::Height(float S) const
{
	FVector P;
	float Yaw;
	At(S, P, Yaw);
	return P.Z;
}

float URideTrack::Grade(float S, float Span) const
{
	const float Half = Span * 0.5f;
	float Back = S - Half, Ahead = S + Half;
	if (!bClosed)
	{
		Back = FMath::Max(Back, 0.f);
		Ahead = FMath::Min(Ahead, Length());
	}
	const float Run = Ahead - Back;
	return Run > 1.f ? (Height(Ahead) - Height(Back)) / Run : 0.f;
}

FRideWeather URideTrack::WeatherAt(float Metres) const
{
	if (Weather.Num() == 0)
	{
		return FRideWeather();
	}
	if (Metres <= Weather[0].Metres)
	{
		return Weather[0];
	}
	if (Metres >= Weather.Last().Metres)
	{
		return Weather.Last();
	}
	// Rows are a few metres apart, so straight interpolation is close to the sim's easing.
	const int32 Hi = Algo::UpperBoundBy(Weather, Metres, &FRideWeather::Metres);
	const FRideWeather& A = Weather[Hi - 1];
	const FRideWeather& B = Weather[Hi];
	const float T = (Metres - A.Metres) / FMath::Max(B.Metres - A.Metres, 1e-3f);
	FRideWeather W;
	W.Metres = Metres;
	W.Lux = FMath::Lerp(A.Lux, B.Lux, T);
	W.Elevation = FMath::Lerp(A.Elevation, B.Elevation, T);
	W.Sky = FMath::Lerp(A.Sky, B.Sky, T);
	W.Fog = FMath::Lerp(A.Fog, B.Fog, T);
	W.Rain = FMath::Lerp(A.Rain, B.Rain, T);
	return W;
}
