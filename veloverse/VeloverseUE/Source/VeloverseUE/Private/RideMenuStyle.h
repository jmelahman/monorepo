// The menus' palette and controls, shared by RideMenu.cpp (state and input) and RideMenuPages.cpp
// (the pages): the design canvas's colours, and buttons, choices and sliders that paint
// themselves and add their hit boxes.
#pragma once

#include "CoreMinimal.h"
#include "RideMenu.h"
#include "SlatePainter.h"

namespace VeloverseMenu
{
using namespace VeloversePaint;

inline const FLinearColor Ink = Hex(0xF3EEE4);
inline const FLinearColor Muted = Hex(0xBDB6A8);
inline const FLinearColor Soft = Hex(0xD6D0C4);
inline const FLinearColor Faint = Hex(0xA9A294);
inline const FLinearColor Night = Hex(0x080A09); // the shade over the world
inline const FLinearColor Ok = Hex(0x7FD69A);
inline const FLinearColor Sky = Hex(0x7CD0FF);
inline const FLinearColor Heart = Hex(0xF0503C);
inline const FLinearColor Fatigue = Hex(0xC99AF7);

inline FLinearColor InkA(float Alpha) { return WithAlpha(Ink, Alpha); }

inline const uint32 ZoneHex[7] = {0xA7B0BA, 0x4FA3F7, 0x3CCB7F, 0xF7D046, 0xFF9A3C, 0xF0503C, 0xB86CF5};
inline const TCHAR* const ZoneNames[7] = {TEXT("Recovery"), TEXT("Endurance"), TEXT("Tempo"), TEXT("Threshold"),
                                          TEXT("VO2 max"),  TEXT("Anaerobic"), TEXT("Sprint")};
inline const uint32 HeartHex[5] = {0xA7B0BA, 0x4FA3F7, 0x3CCB7F, 0xFF9A3C, 0xF0503C};
inline const TCHAR* const HeartNames[5] = {TEXT("Z1 Easy"), TEXT("Z2 Steady"), TEXT("Z3 Moderate"), TEXT("Z4 Hard"),
                                           TEXT("Z5 Max")};

// The islands, in the order the stats list them, and each one's colour on the maps and charts.
inline const TCHAR* const IslandNames[5] = {TEXT("The Coast"), TEXT("Greenwood"), TEXT("Vael"), TEXT("The Moor"),
                                            TEXT("Red Sands")};
inline FLinearColor IslandColour(const FString& Name)
{
	static const uint32 Hexes[5] = {0x8EC5E8, 0x4E9A5B, 0xD9D4CC, 0xA77BC0, 0xD9773F};
	for (int32 I = 0; I < 5; ++I)
	{
		if (Name.Contains(IslandNames[I]))
		{
			return Hex(Hexes[I]);
		}
	}
	return Muted;
}
// The name an island shows on screen. The hub's roads carry "The Coast" in the world's data, but the
// route headers, the map and the ride summary call the island Harbour Isle.
inline FString IslandLabel(const FString& Name)
{
	return Name == TEXT("The Coast") ? FString(TEXT("Harbour Isle")) : Name;
}

// The design's icons: Lucide's (lucide.dev, ISC; Fonts/Lucide.ttf, lucide-static 0.460.0), each a
// glyph of its icon font. Drawn as text, they are anti-aliased and joined cleanly at any size and
// alpha; the old stroked paths notched at the corners whenever they faded (the dock).
namespace Icons
{
inline const TCHAR* Back = TEXT("\uE04C"); // arrow-left
inline const TCHAR* Arrow = TEXT("\uE04D"); // arrow-right
inline const TCHAR* Chevron = TEXT("\uE073"); // chevron-right
inline const TCHAR* ChevronLeft = TEXT("\uE072"); // chevron-left
inline const TCHAR* Route = TEXT("\uE543"); // route
inline const TCHAR* Stats = TEXT("\uE2A3"); // chart-column
inline const TCHAR* Workout = TEXT("\uE1E0"); // timer
inline const TCHAR* Signal = TEXT("\uE146"); // radio
inline const TCHAR* Gear = TEXT("\uE158"); // settings
inline const TCHAR* Leave = TEXT("\uE144"); // power
inline const TCHAR* Bluetooth = TEXT("\uE060"); // bluetooth
inline const TCHAR* Trainer = TEXT("\uE4E6"); // circle-gauge
inline const TCHAR* HeartIcon = TEXT("\uE0F5"); // heart
inline const TCHAR* Bolt = TEXT("\uE1B4"); // zap
inline const TCHAR* Cadence = TEXT("\uE14D"); // rotate-cw
inline const TCHAR* Check = TEXT("\uE070"); // check
inline const TCHAR* Download = TEXT("\uE0B6"); // download
inline const TCHAR* Screen = TEXT("\uE121"); // monitor
inline const TCHAR* Sliders = TEXT("\uE29A"); // sliders-horizontal
inline const TCHAR* Bike = TEXT("\uE1D2"); // bike
inline const TCHAR* Rider = TEXT("\uE19F"); // user
inline const TCHAR* Globe = TEXT("\uE0EB"); // globe
inline const TCHAR* Lock = TEXT("\uE10F"); // lock
inline const TCHAR* Eye = TEXT("\uE0BE"); // eye
inline const TCHAR* Pulse = TEXT("\uE038"); // activity
inline const TCHAR* Hud = TEXT("\uE130"); // panels-top-left
inline const TCHAR* Move = TEXT("\uE53B"); // scan-eye
inline const TCHAR* Pause = TEXT("\uE132"); // pause
inline const TCHAR* Sun = TEXT("\uE17C"); // sun
inline const TCHAR* Cloud = TEXT("\uE08C"); // cloud
inline const TCHAR* CloudRain = TEXT("\uE092"); // cloud-rain
} // namespace Icons

// "1,140"
inline FString Thousands(double Value)
{
	const int64 N = FMath::RoundToInt64(FMath::Abs(Value));
	FString Digits = FString::Printf(TEXT("%lld"), N);
	for (int32 At = Digits.Len() - 3; At > 0; At -= 3)
	{
		Digits.InsertAt(At, TEXT(','));
	}
	return Value < -0.5 ? TEXT("-") + Digits : Digits;
}

// 1:02:03, or 4:05 under an hour.
inline FString Clock(double Seconds, bool bAlwaysHours = true)
{
	const int64 S = FMath::FloorToInt64(FMath::Max(Seconds, 0.0));
	if (!bAlwaysHours && S < 3600)
	{
		return FString::Printf(TEXT("%lld:%02lld"), S / 60, S % 60);
	}
	return FString::Printf(TEXT("%lld:%02lld:%02lld"), S / 3600, S / 60 % 60, S % 60);
}

// "212 W", or a dash for a ride too short to have it.
inline FString Watts(double W) { return W > 0.0 ? FString::Printf(TEXT("%.0f W"), W) : FString(TEXT("–")); }

inline const TCHAR* MonthName(int32 Month)
{
	static const TCHAR* Months[] = {TEXT("Jan"), TEXT("Feb"), TEXT("Mar"), TEXT("Apr"), TEXT("May"), TEXT("Jun"),
	                                TEXT("Jul"), TEXT("Aug"), TEXT("Sep"), TEXT("Oct"), TEXT("Nov"), TEXT("Dec")};
	return Months[FMath::Clamp(Month, 1, 12) - 1];
}

inline const TCHAR* DayName(const FDateTime& When)
{
	static const TCHAR* Days[] = {TEXT("Monday"), TEXT("Tuesday"), TEXT("Wednesday"), TEXT("Thursday"),
	                              TEXT("Friday"), TEXT("Saturday"), TEXT("Sunday")};
	return Days[static_cast<int32>(When.GetDayOfWeek())];
}

// "Today 18:20", "Yesterday 07:05", "Tue 11 Nov".
inline FString RideDay(const FDateTime& When)
{
	const FDateTime Today = FDateTime::Now().GetDate();
	if (When.GetDate() == Today)
	{
		return TEXT("Today ") + When.ToString(TEXT("%H:%M"));
	}
	if (When.GetDate() == Today - FTimespan::FromDays(1.0))
	{
		return TEXT("Yesterday ") + When.ToString(TEXT("%H:%M"));
	}
	return FString::Printf(TEXT("%.3s %d %s"), DayName(When), When.GetDay(), MonthName(When.GetMonth()));
}

// A baseline that centres text of size Px on Y.
inline float Mid(float Y, float Px) { return Y + 0.36f * Px; }

// Dark text on a light accent, light on a dark one.
inline FLinearColor TextOn(const FLinearColor& Fill)
{
	return Fill.GetLuminance() > 0.3f ? Hex(0x17130A) : Ink;
}

enum class EButton : uint8
{
	Primary, // filled with the accent
	Outline,
	Quiet, // no edge until hovered
};
} // namespace VeloverseMenu

struct URideMenu::FPaintContext
{
	const VeloversePaint::FPainter& P;
	URideMenu& Menu; // for the hits' actions
	ARidePawn* Pawn;
	float W; // the frame's width: 1920 at 16:9
	FLinearColor Accent, OnAccent;
	double Time; // seconds, for what pulses
	float Dt = 0.f; // seconds since the last paint, for what eases

	// How far Key has come towards hovered (bOn), 0 to 1: eased out over about 150 ms, quick to
	// start and gentle to settle. Once per hit per frame. The keys' focus moves without easing,
	// so stepping through the menu with them never lags.
	float Lift(const FString& Key, bool bOn) const
	{
		float& Value = Menu.Lifts.FindOrAdd(Key);
		Value = Menu.bKeys ? (bOn ? 1.f : 0.f) : FMath::Lerp(Value, bOn ? 1.f : 0.f, 1.f - FMath::Exp(-Dt / 0.05f));
		return Value;
	}
	float Lift(const FString& Key) const { return Lift(Key, Hot(Key)); }

	FHit& Hit(const FString& Key, float X, float Y, float Wd, float H) const
	{
		FHit& Out = Menu.Hits.AddDefaulted_GetRef();
		Out.Key = Key;
		Out.X = X;
		Out.Y = Y;
		Out.W = Wd;
		Out.H = H;
		return Out;
	}
	bool Hot(const FString& Key) const { return Menu.Hover == Key || (Menu.bKeys && Menu.Focus == Key); }
	// The keys' focus, drawn round what they'd press.
	void Ring(const FString& Key, float X, float Y, float Wd, float H, float R) const
	{
		if (Menu.bKeys && Menu.Focus == Key)
		{
			P.Border(X - 4.f, Y - 4.f, Wd + 8.f, H + 8.f, R + 4.f, VeloverseMenu::Ink, 2.f);
		}
	}

	// A panel as the boards draw them: a dark glass with a faint edge.
	void Panel(float X, float Y, float Wd, float H, float R = 16.f, float Alpha = 0.8f) const
	{
		P.Round(X, Y, Wd, H, R, VeloversePaint::Hex(0x0A0C0B, Alpha));
		P.Border(X, Y, Wd, H, R, VeloverseMenu::InkA(0.14f));
	}

	float ButtonWidth(const FString& Label, float Px, bool bIcon, float Pad = 28.f) const
	{
		return P.Width(Label, P.Font("Bold", Px)) + 2.f * Pad + (bIcon ? 30.f : 0.f);
	}

	void Button(const FString& Key, float X, float Y, float Wd, float H, const FString& Label,
	            VeloverseMenu::EButton Style, TFunction<void()> Click, const TCHAR* Icon = nullptr, float Px = 19.f,
	            bool bSerif = false, float R = 12.f, bool bIconAfter = false) const
	{
		using namespace VeloverseMenu;
		// Hovered, the button grows 4% about its middle as it brightens (the hit stays put, so its
		// edge can't flicker under the pointer).
		Hit(Key, X, Y, Wd, H).OnClick = MoveTemp(Click);
		const float L = Lift(Key);
		const float Grow = 1.f + 0.04f * L;
		X -= Wd * (Grow - 1.f) * 0.5f;
		Y -= H * (Grow - 1.f) * 0.5f;
		Wd *= Grow;
		H *= Grow;
		R *= Grow;
		Px *= Grow;
		FLinearColor Fg = Ink;
		switch (Style)
		{
		case EButton::Primary:
			P.Round(X, Y, Wd, H, R, Accent * (1.f + 0.12f * L));
			Fg = OnAccent;
			break;
		case EButton::Outline:
			P.Round(X, Y, Wd, H, R, InkA(0.1f * L));
			P.Border(X, Y, Wd, H, R, InkA(0.28f + 0.14f * L));
			break;
		case EButton::Quiet:
			P.Round(X, Y, Wd, H, R, InkA(0.1f * L));
			break;
		}
		const FSlateFontInfo F = bSerif ? P.Serif(Px) : P.Font(Style == EButton::Primary ? "Bold" : "Medium", Px);
		const float TextW = P.Width(Label, F);
		const float IconW = (Icon ? 30.f : 0.f) * Grow;
		const float Left = X + (Wd - TextW - IconW) * 0.5f;
		if (Icon)
		{
			// Before the words, or after them for a way on; an arrow after them leans on 3 px.
			const float IconPx = 22.f * Grow;
			P.Icon(Icon, bIconAfter ? Left + TextW + 8.f * Grow + 3.f * L : Left, Y + (H - IconPx) * 0.5f, IconPx, Fg, 2.2f);
		}
		P.Text(bIconAfter ? Left : Left + IconW, Mid(Y + H * 0.5f, Px), Label, F, Fg);
		Ring(Key, X, Y, Wd, H, R);
	}

	// The round back button at the top left of every page but the start.
	void BackButton(TFunction<void()> Click) const
	{
		using namespace VeloverseMenu;
		const FString Key = TEXT("back");
		const float L = Lift(Key);
		const float Rad = 26.f * (1.f + 0.08f * L); // grows about its middle, and the arrow leans back
		P.Round(90.f - Rad, 74.f - Rad, 2.f * Rad, 2.f * Rad, Rad, InkA(0.12f * L));
		P.Border(90.f - Rad, 74.f - Rad, 2.f * Rad, 2.f * Rad, Rad, InkA(0.22f + 0.14f * L));
		P.Icon(Icons::Back, 78.f - 3.f * L, 62.f, 24.f, Ink);
		Ring(Key, 64.f, 48.f, 52.f, 52.f, 26.f);
		Hit(Key, 64.f, 48.f, 52.f, 52.f).OnClick = MoveTemp(Click);
	}

	// A choice of a few, side by side, the chosen one filled: returns its width.
	float SegmentedWidth(const TArray<FString>& Options, float Px = 16.f, float MinW = 112.f) const
	{
		float Wd = 10.f;
		for (const FString& O : Options)
		{
			Wd += FMath::Max(MinW, P.Width(O, P.Font("Bold", Px)) + 36.f);
		}
		return Wd;
	}
	float Segmented(const FString& Key, float X, float Y, const TArray<FString>& Options, int32 Chosen,
	                TFunction<void(int32)> Pick, float H = 48.f, float Px = 16.f, float MinW = 112.f) const
	{
		using namespace VeloverseMenu;
		const float Wd = SegmentedWidth(Options, Px, MinW);
		P.Round(X, Y, Wd, H + 10.f, 12.f, InkA(0.06f));
		float At = X + 5.f;
		const TSharedRef<TFunction<void(int32)>> Shared = MakeShared<TFunction<void(int32)>>(MoveTemp(Pick));
		const int32 Count = Options.Num();
		for (int32 I = 0; I < Count; ++I)
		{
			const FString Each = Key + FString::FromInt(I);
			const FSlateFontInfo F = P.Font("Bold", Px);
			const float Bw = FMath::Max(MinW, P.Width(Options[I], F) + 36.f);
			const bool bOn = I == Chosen;
			const float L = Lift(Each);
			if (bOn)
			{
				P.Round(At, Y + 5.f, Bw, H, 8.f, Accent);
			}
			else if (L > 0.001f)
			{
				P.Round(At, Y + 5.f, Bw, H, 8.f, InkA(0.1f * L));
			}
			P.Text(At + Bw * 0.5f, Mid(Y + 5.f + H * 0.5f, Px), Options[I], F, bOn ? OnAccent : Ink, EAlign::Centre);
			Ring(Each, At, Y + 5.f, Bw, H, 8.f);
			FHit& Out = Hit(Each, At, Y + 5.f, Bw, H);
			Out.OnClick = [Shared, I] { (*Shared)(I); };
			const int32 From = Chosen >= 0 ? Chosen : I;
			Out.OnLeft = [Shared, From] { (*Shared)(FMath::Max(From - 1, 0)); };
			Out.OnRight = [Shared, From, Count] { (*Shared)(FMath::Min(From + 1, Count - 1)); };
			At += Bw;
		}
		return Wd;
	}

	// A slider over Wd with its value written after it; Set takes the share along the track.
	void Slider(const FString& Key, float X, float Cy, float Wd, float Share, const FString& Value,
	            TFunction<void(float)> Set, TFunction<void(int32)> Step) const
	{
		using namespace VeloverseMenu;
		const float Track = Wd - 106.f;
		Share = FMath::Clamp(Share, 0.f, 1.f);
		const bool bHot = Hot(Key) || Menu.Dragging == Key;
		P.Round(X, Cy - 3.f, Track, 6.f, 3.f, InkA(0.14f));
		P.Round(X, Cy - 3.f, FMath::Max(Track * Share, 6.f), 6.f, 3.f, Accent);
		P.Disc(X + Track * Share, Cy, 10.f + 3.f * Lift(Key, bHot), Ink);
		P.Text(X + Wd, Mid(Cy, 18.f), Value, P.Font("Bold", 18.f), Ink, EAlign::Right);
		Ring(Key, X - 12.f, Cy - 20.f, Track + 24.f, 40.f, 20.f);
		FHit& Out = Hit(Key, X - 12.f, Cy - 20.f, Track + 24.f, 40.f);
		const TSharedRef<TFunction<void(int32)>> Shared = MakeShared<TFunction<void(int32)>>(MoveTemp(Step));
		Out.OnDrag = [Set = MoveTemp(Set), Track](float At)
		{ Set(FMath::Clamp((At * (Track + 24.f) - 12.f) / Track, 0.f, 1.f)); };
		Out.OnLeft = [Shared] { (*Shared)(-1); };
		Out.OnRight = [Shared] { (*Shared)(1); };
	}
};
