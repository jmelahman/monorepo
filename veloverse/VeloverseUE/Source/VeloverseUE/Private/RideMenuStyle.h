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

// The design's icons, on a 24-unit grid.
namespace Icons
{
inline const TCHAR* Back = TEXT("M19 12H5M11 6l-6 6 6 6");
inline const TCHAR* Arrow = TEXT("M5 12h14M13 6l6 6-6 6");
inline const TCHAR* Chevron = TEXT("M9 6l6 6-6 6");
inline const TCHAR* ChevronLeft = TEXT("M15 6l-6 6 6 6");
inline const TCHAR* Map = TEXT("M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2zM9 4v14M15 6v14");
inline const TCHAR* Stats = TEXT("M4 20V10M10 20V4M16 20v-7M22 20H2");
inline const TCHAR* Workout = TEXT("M3 17l5-6 4 4 4-7 5 9");
inline const TCHAR* Signal = TEXT("M4 12a8 8 0 0116 0M7.5 12a4.5 4.5 0 019 0M12 12v8");
inline const TCHAR* Gear = TEXT("M12 9a3 3 0 100 6 3 3 0 000-6zM4 12h2M18 12h2M12 4v2M12 18v2M6.3 6.3l1.4 1.4M16.3 "
                                "16.3l1.4 1.4M6.3 17.7l1.4-1.4M16.3 7.7l1.4-1.4");
inline const TCHAR* Leave = TEXT("M10 4H5v16h5M14 8l4 4-4 4M18 12H9");
inline const TCHAR* Bluetooth = TEXT("M7 7l10 10-5 5V2l5 5L7 17");
inline const TCHAR* Trainer = TEXT("M12 3a7 7 0 100 14 7 7 0 000-14zM12 8.5a1.5 1.5 0 100 3 1.5 1.5 0 000-3zM8 16.5L6 "
                                   "21M16 16.5l2 4.5M4 21h16");
// Two round lobes meeting in a point.
inline const TCHAR* HeartIcon = TEXT("M12 20.5L4.6 13.2C2.4 11 2.4 7.4 4.6 5.3C6.7 3.2 10 3.4 12 5.8C14 3.4 17.3 3.2 "
                                     "19.4 5.3C21.6 7.4 21.6 11 19.4 13.2z");
inline const TCHAR* Bolt = TEXT("M13 2L4.5 13.5H11L10 22l8.5-11.5H12z");
inline const TCHAR* Cadence = TEXT("M20 12a8 8 0 11-2.3-5.7M20 4v4h-4M12 12l3-2");
inline const TCHAR* Check = TEXT("M5 12l5 5 9-10");
inline const TCHAR* Download = TEXT("M12 4v11M7 10l5 5 5-5M5 20h14");
inline const TCHAR* Screen = TEXT("M3 5h18v12H3zM8 21h8M12 17v4");
inline const TCHAR* Layers = TEXT("M12 3l9 5-9 5-9-5zM3 13l9 5 9-5M3 17.5l9 5 9-5");
inline const TCHAR* Bike = TEXT("M5 17a3 3 0 100 .1M19 17a3 3 0 100 .1M5 17l4-8h6l4 8M9 9l3 8");
inline const TCHAR* Rider = TEXT("M12 4a3.5 3.5 0 100 7 3.5 3.5 0 000-7zM5 20a7 7 0 0114 0");
inline const TCHAR* Globe = TEXT("M12 3a9 9 0 100 18 9 9 0 000-18zM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18");
inline const TCHAR* Lock = TEXT("M6 11h12v9H6zM9 11V8a3 3 0 016 0v3");
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
		const bool bHot = Hot(Key);
		FLinearColor Fg = Ink;
		switch (Style)
		{
		case EButton::Primary:
			P.Round(X, Y, Wd, H, R, bHot ? Accent * 1.12f : Accent);
			Fg = OnAccent;
			break;
		case EButton::Outline:
			P.Round(X, Y, Wd, H, R, InkA(bHot ? 0.1f : 0.f));
			P.Border(X, Y, Wd, H, R, InkA(0.28f));
			break;
		case EButton::Quiet:
			P.Round(X, Y, Wd, H, R, InkA(bHot ? 0.1f : 0.f));
			break;
		}
		const FSlateFontInfo F = bSerif ? P.Serif(Px) : P.Font(Style == EButton::Primary ? "Bold" : "Medium", Px);
		const float TextW = P.Width(Label, F);
		const float IconW = Icon ? 30.f : 0.f;
		const float Left = X + (Wd - TextW - IconW) * 0.5f;
		if (Icon)
		{
			// Before the words, or after them for a way on.
			P.Icon(Icon, bIconAfter ? Left + TextW + 8.f : Left, Y + H * 0.5f - 11.f, 22.f, Fg, 2.2f);
		}
		P.Text(bIconAfter ? Left : Left + IconW, Mid(Y + H * 0.5f, Px), Label, F, Fg);
		Ring(Key, X, Y, Wd, H, R);
		Hit(Key, X, Y, Wd, H).OnClick = MoveTemp(Click);
	}

	// The round back button at the top left of every page but the start.
	void BackButton(TFunction<void()> Click) const
	{
		using namespace VeloverseMenu;
		const FString Key = TEXT("back");
		P.Round(64.f, 48.f, 52.f, 52.f, 26.f, InkA(Hot(Key) ? 0.12f : 0.f));
		P.Border(64.f, 48.f, 52.f, 52.f, 26.f, InkA(0.22f));
		P.Icon(Icons::Back, 78.f, 62.f, 24.f, Ink);
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
			if (bOn)
			{
				P.Round(At, Y + 5.f, Bw, H, 8.f, Accent);
			}
			else if (Hot(Each))
			{
				P.Round(At, Y + 5.f, Bw, H, 8.f, InkA(0.1f));
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
		P.Disc(X + Track * Share, Cy, bHot ? 12.f : 10.f, Ink);
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
