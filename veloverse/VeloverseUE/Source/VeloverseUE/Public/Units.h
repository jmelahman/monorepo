// Metric or imperial, as the rider chose (FRideProfile::bImperial): everything that shows a
// distance, a speed or a height reads it from here. ARidePawn sets it when the profile loads or
// changes.
#pragma once

#include "CoreMinimal.h"

namespace Veloverse::Units
{
inline bool bImperial = false;

constexpr double MetresPerMile = 1609.344;
constexpr double FeetPerMetre = 3.28084;

// Kilometres or miles, from metres.
inline double Long(double Metres) { return bImperial ? Metres / MetresPerMile : Metres / 1000.0; }
inline const TCHAR* LongUnit() { return bImperial ? TEXT("mi") : TEXT("km"); }

// km/h or mph, from metres a second.
inline double Speed(double Mps) { return bImperial ? Mps * 3600.0 / MetresPerMile : Mps * 3.6; }
inline const TCHAR* SpeedUnit() { return bImperial ? TEXT("mph") : TEXT("km/h"); }

// Metres or feet, from metres.
inline double Height(double Metres) { return bImperial ? Metres * FeetPerMetre : Metres; }
inline const TCHAR* HeightUnit() { return bImperial ? TEXT("ft") : TEXT("m"); }

// A number to 0, 1 or 2 places (FString::Printf wants its format spelled out).
inline FString Number(double Value, int32 Places)
{
	return Places <= 0   ? FString::Printf(TEXT("%.0f"), Value)
	       : Places == 1 ? FString::Printf(TEXT("%.1f"), Value)
	                     : FString::Printf(TEXT("%.2f"), Value);
}

// "12.3 km" or "7.6 mi", from metres.
inline FString Distance(double Metres, int32 Places = 1)
{
	return Number(Long(Metres), Places) + TEXT(" ") + LongUnit();
}

// "120 m" or "394 ft", from metres.
inline FString Rise(double Metres) { return Number(Height(Metres), 0) + TEXT(" ") + HeightUnit(); }
} // namespace Veloverse::Units
