#include "RideOverlay.h"

#include "Blueprint/WidgetTree.h"
#include "Components/CanvasPanel.h"
#include "Dock.h"
#include "RideMenuStyle.h"
#include "RidePawn.h"
#include "RideTrack.h"
#include "SlatePainter.h"
#include "Units.h"

using namespace VeloversePaint;

namespace
{
const FLinearColor Ink = Hex(0xF4F1EA);
const FLinearColor Dim = Hex(0xF4F1EA, 0.72f);
const FLinearColor BadgeText = Hex(0x121212);
const FLinearColor Sky = Hex(0x7CD0FF);
const FLinearColor SkyText = Hex(0xBDE6FF);

const uint32 ZoneHex[7] = {0xA7B0BA, 0x4FA3F7, 0x3CCB7F, 0xF7D046, 0xFF9A3C, 0xF0503C, 0xB86CF5};
const TCHAR* ZoneNames[7] = {TEXT("Recovery"),  TEXT("Endurance"), TEXT("Tempo"), TEXT("Threshold"),
                             TEXT("VO2 max"),   TEXT("Anaerobic"), TEXT("Sprint")};

// Grade on the profiles: yellow at 0 % to red at 8 % and steeper, mixed in sRGB as the canvas
// did; flat and downhill neutral.
FLinearColor GradeColour(float Grade)
{
	if (Grade < 0.005f)
	{
		return Hex(0xF4F1EA, 0.3f);
	}
	const float T = FMath::Clamp(Grade / 0.08f, 0.f, 1.f);
	auto Mix = [T](int32 A, int32 B) { return static_cast<uint8>(FMath::RoundToInt32(FMath::Lerp(float(A), float(B), T))); };
	return FLinearColor::FromSRGBColor(FColor(Mix(0xF5, 0xE8), Mix(0xC8, 0x45), Mix(0x42, 0x2C)));
}

const TCHAR* GradeWord(float Grade)
{
	return Grade < 0.03f ? TEXT("Gentle") : Grade < 0.05f ? TEXT("Moderate") : TEXT("Steep");
}

enum class ESky : uint8
{
	Clear,
	Cloudy,
	Rain,
};

ESky SkyOf(const FRideWeather& W)
{
	if (W.Rain > 0.3f)
	{
		return ESky::Rain;
	}
	return W.Lux < 40000.f || W.Fog > 0.02f ? ESky::Cloudy : ESky::Clear;
}

FString Clock(float Seconds)
{
	const int32 S = FMath::FloorToInt32(Seconds);
	return FString::Printf(TEXT("%d:%02d:%02d"), S / 3600, S / 60 % 60, S % 60);
}

// Far off in kilometres (or miles), close in metres (or feet) rounded to 50.
FString Km(float Metres)
{
	if (Metres >= (Veloverse::Units::bImperial ? 160.f : 950.f))
	{
		return Veloverse::Units::Distance(Metres);
	}
	const double Close = FMath::Max(50.0, FMath::RoundToDouble(Veloverse::Units::Height(Metres) / 50.0) * 50.0);
	return Veloverse::Units::Number(Close, 0) + TEXT(" ") + Veloverse::Units::HeightUnit();
}

// The weather's icon in a 22 px square at X, Y.
void Icon(const FPainter& P, float X, float Y, ESky Weather)
{
	P.Icon(Weather == ESky::Clear ? VeloverseMenu::Icons::Sun : Weather == ESky::Rain ? VeloverseMenu::Icons::CloudRain : VeloverseMenu::Icons::Cloud, X, Y, 22.f, Sky);
}
} // namespace

TSharedRef<SWidget> URideOverlay::RebuildWidget()
{
	if (WidgetTree && !WidgetTree->RootWidget)
	{
		WidgetTree->RootWidget = WidgetTree->ConstructWidget<UCanvasPanel>();
	}
	SetVisibility(ESlateVisibility::HitTestInvisible);
	return Super::RebuildWidget();
}

const ARidePawn* URideOverlay::Rider() const
{
	const APlayerController* Player = GetOwningPlayer();
	return Player ? Cast<ARidePawn>(Player->GetPawn()) : nullptr;
}

void URideOverlay::NativeTick(const FGeometry& Geometry, float DeltaTime)
{
	Super::NativeTick(Geometry, DeltaTime);
	const ARidePawn* Pawn = Rider();
	if (!Pawn)
	{
		return;
	}
	if (Pawn->Track && Surveyed.Get() != Pawn->Track)
	{
		Survey(*Pawn->Track);
	}
	if (Pawn->Track && ClimbsOf.Get() != Pawn->Track && Pawn->Segments.UsesTrack(Pawn->Track))
	{
		TakeClimbs(Pawn->Segments);
	}
	Watts += (Pawn->Reading.Watts - Watts) * (1.f - FMath::Exp(-DeltaTime / 0.8f));
	FrameSeconds = FrameSeconds > 0.f ? FrameSeconds + (DeltaTime - FrameSeconds) * (1.f - FMath::Exp(-DeltaTime / 0.5f))
	                                  : DeltaTime;
}

void URideOverlay::Survey(const URideTrack& Track)
{
	Surveyed = &Track;
	LengthM = Track.Length() / 100.f;
	bClosed = Track.bClosed;
	Heights.Reset();
	Grades.Reset();
	Climbs.Reset();
	const int32 N = FMath::FloorToInt32(LengthM / SampleM) + 1;
	for (int32 I = 0; I < N; ++I)
	{
		Heights.Add(Track.Height(FMath::Min(I * SampleM, LengthM) * 100.f) / 100.f);
	}
	if (N < 2)
	{
		return;
	}
	TArray<float> Raw;
	for (int32 I = 0; I < N; ++I)
	{
		const int32 J = FMath::Min(I + 1, N - 1), K = J - 1;
		Raw.Add((Heights[J] - Heights[K]) / SampleM);
	}
	for (int32 I = 0; I < N; ++I)
	{
		float Sum = 0.f;
		int32 Count = 0;
		for (int32 J = FMath::Max(0, I - 2); J <= FMath::Min(N - 1, I + 2); ++J)
		{
			Sum += Raw[J];
			++Count;
		}
		Grades.Add(Sum / Count);
	}
}

// The climbs are the route's segments, the ones timed and kept as bests, so the panel and the
// segment banner agree on where a climb starts and ends.
void URideOverlay::TakeClimbs(const FRideSegments& Segments)
{
	ClimbsOf = Surveyed;
	Climbs.Reset();
	for (int32 I = 0; I < Segments.Num(); ++I)
	{
		const Veloverse::FSegment& S = Segments.Segment(I);
		if (!S.bClimb)
		{
			continue;
		}
		FClimb C;
		C.Start = static_cast<float>(S.Start);
		C.Top = C.Start + static_cast<float>(S.Length(LengthM)); // past LengthM when it wraps the seam
		C.Rise = static_cast<float>(S.Rise);
		for (float M = C.Start; M <= C.Top; M += SampleM)
		{
			C.Steepest = FMath::Max(C.Steepest, GradeAt(M));
		}
		Climbs.Add(C);
	}
	Climbs.Sort([](const FClimb& A, const FClimb& B) { return A.Start < B.Start; });
}

float URideOverlay::HeightAt(float Metres) const
{
	if (Heights.Num() < 2)
	{
		return 0.f;
	}
	Metres = bClosed ? FMath::Fmod(FMath::Fmod(Metres, LengthM) + LengthM, LengthM) : FMath::Clamp(Metres, 0.f, LengthM);
	const float F = Metres / SampleM;
	const int32 I = FMath::Clamp(FMath::FloorToInt32(F), 0, Heights.Num() - 2);
	return FMath::Lerp(Heights[I], Heights[I + 1], FMath::Clamp(F - I, 0.f, 1.f));
}

float URideOverlay::GradeAt(float Metres) const
{
	if (Grades.IsEmpty())
	{
		return 0.f;
	}
	Metres = bClosed ? FMath::Fmod(FMath::Fmod(Metres, LengthM) + LengthM, LengthM) : FMath::Clamp(Metres, 0.f, LengthM);
	return Grades[FMath::Clamp(FMath::RoundToInt32(Metres / SampleM), 0, Grades.Num() - 1)];
}

const URideOverlay::FClimb* URideOverlay::ClimbAt(float Here, bool& bOn, float& Start, float& Top, float Reach) const
{
	bOn = false;
	for (const FClimb& C : Climbs)
	{
		// On it, or on the far side of the seam of one that wraps it.
		const float At = bClosed && Here < C.Start && C.Top > LengthM ? Here + LengthM : Here;
		if (C.Start <= At && At < C.Top)
		{
			bOn = true;
			Start = C.Start;
			Top = C.Top;
			return &C;
		}
	}
	for (const FClimb& C : Climbs)
	{
		if (C.Start > Here)
		{
			Start = C.Start;
			Top = C.Top;
			return C.Start - Here <= Reach ? &C : nullptr;
		}
	}
	if (bClosed && !Climbs.IsEmpty())
	{
		Start = Climbs[0].Start + LengthM;
		Top = Climbs[0].Top + LengthM;
		return Start - Here <= Reach ? &Climbs[0] : nullptr;
	}
	return nullptr;
}

int32 URideOverlay::NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
                                FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
                                bool bParentEnabled) const
{
	Layer = Super::NativePaint(Args, Geometry, Culling, Out, Layer, Style, bParentEnabled);
	const ARidePawn* Pawn = Rider();
	const FVector2f Local = Geometry.GetLocalSize();
	if (!Pawn || !Pawn->bStarted || !Pawn->Track || Heights.Num() < 2 || Local.Y <= 0.f)
	{
		return Layer;
	}
	const URideTrack& Track = *Pawn->Track;
	const FPainter P{Geometry, Out, Layer, Local.Y / 720.f};
	const float W = Local.X / P.S; // the frame's width in canvas units: 1280 at 16:9
	const float H = 720.f;
	const float Here = Track.Wrap(static_cast<float>(Pawn->Distance)) / 100.f;
	// The rider's HUD colour, picked on the profile page.
	const FLinearColor Accent = Pawn->Profile.Accent();

	const Veloverse::FDockLayout Dock =
		Veloverse::LayoutDock(W, H, Pawn->bFreeLook ? 0.f : Pawn->Dock.Shown);
	if (Pawn->Profile.bGraphs)
	{
		PaintGraphs(P, W, H);
	}

	// The panels, unless the rider has put them away (H).
	Pawn->TimePill = {};
	Pawn->WeatherPill = {};
	if (Pawn->Profile.bHud)
	{
		// ---- Top left: power ------------------------------------------------------------------------
		{
			const int32 Zone = Pawn->Zone(Watts);
			const FLinearColor ZoneColour = Hex(ZoneHex[Zone]);
			const float X = 20.f, Y = 20.f, Wide = 220.f;
			P.Panel(X, Y, Wide, 130.f, 14.f);
			P.Text(X + 16.f, Y + 25.f, TEXT("POWER"), P.Font("Bold", 12.f, 0.08f), Dim);
			const FString Badge = FString::Printf(TEXT("Z%d %s"), Zone + 1, ZoneNames[Zone]);
			const FSlateFontInfo BadgeFont = P.Font("Bold", 12.f);
			const float BadgeW = P.Width(Badge, BadgeFont) + 14.f;
			P.Round(X + Wide - 16.f - BadgeW, Y + 11.f, BadgeW, 18.f, 6.f, ZoneColour);
			P.Text(X + Wide - 16.f - 7.f, Y + 25.f, Badge, BadgeFont, BadgeText, EAlign::Right);
			const float Number = P.Text(X + 16.f, Y + 86.f, FString::Printf(TEXT("%.0f"), Watts), P.Font("Black", 60.f, -0.02f), Ink);
			P.Text(X + 16.f + Number + 4.f, Y + 86.f, TEXT("W"), P.Font("Bold", 22.f), Ink);
			const float Gap = 3.f, Bar = (Wide - 32.f - 6.f * Gap) / 7.f, Foot = Y + 116.f;
			for (int32 I = 0; I < 7; ++I)
			{
				const float Tall = I == Zone ? 9.f : 5.f;
				P.Round(X + 16.f + I * (Bar + Gap), Foot - Tall, Bar, Tall, 2.f, I == Zone ? ZoneColour : Hex(0xF4F1EA, 0.25f));
			}
		}

		// ---- Top right: the road ahead or this climb, and the loop ---------------------------------
		// The power panel's size, mirrored: its label and badge row, the profile where the watts are,
		// and the stretch bar level with the zone bar.
		float WeatherTop;
		constexpr float MapPanelWide = 268.f;
		{
			const float Wide = MapPanelWide, Tall = 130.f, X = W - 20.f - Wide, Top = 20.f;
			const float X0 = X + 16.f, Inner = Wide - 32.f, Right = X0 + Inner;
			bool bOn = false;
			float Start = 0.f, Peak = 0.f;
			const FClimb* Climb = ClimbAt(Here, bOn, Start, Peak, 5000.f);
			const bool bClimbing = Climb && bOn && HeightAt(Peak) - HeightAt(Here) >= 3.f;
			P.Panel(X, Top, Wide, Tall, 14.f);
			const FSlateFontInfo Label = P.Font("Bold", 12.f, 0.08f);
			const FSlateFontInfo Strong = P.Font("Bold", 12.f);
			const FSlateFontInfo Soft = P.Font("Medium", 12.f);
			const float Grade = Pawn->Grade;
			P.Text(X0, Top + 25.f, bClimbing ? TEXT("THIS CLIMB") : (Veloverse::Units::bImperial ? TEXT("NEXT 2 MI") : TEXT("NEXT 3 KM")), Label, Dim);
			const FString Badge = FString::Printf(TEXT("%+.1f %%"), Grade * 100.f);
			const float BadgeW = P.Width(Badge, Strong) + 14.f;
			P.Round(Right - BadgeW, Top + 11.f, BadgeW, 18.f, 6.f, GradeColour(FMath::Max(Grade, 0.006f)));
			P.Text(Right - 7.f, Top + 25.f, Badge, Strong, BadgeText, EAlign::Right);

			// The profile: a climb from its foot to past its top, or the next 3 km (2 miles).
			float From, To;
			if (bClimbing)
			{
				const float Long = FMath::Max(Peak - Start, 300.f);
				From = Start - 0.08f * Long;
				To = Peak + 0.25f * Long;
			}
			else
			{
				// 3 km, or 2 miles: the window the label names.
				const float Window = Veloverse::Units::bImperial ? 2.f * static_cast<float>(Veloverse::Units::MetresPerMile) : 3000.f;
				From = Here - 0.05f * Window;
				To = From + Window;
			}
			if (!bClosed)
			{
				const float Span = To - From;
				From = FMath::Clamp(From, 0.f, FMath::Max(LengthM - Span, 0.f));
				To = FMath::Min(From + Span, LengthM);
			}
			{
				const float PX = X0, PY = Top + 36.f, PW = Inner, PH = 52.f;
				constexpr int32 Columns = 96;
				TArray<float> Hs, Gs;
				float Lo = TNumericLimits<float>::Max(), Hi = TNumericLimits<float>::Lowest();
				for (int32 C = 0; C <= Columns; ++C)
				{
					const float M = FMath::Lerp(From, To, float(C) / Columns);
					Hs.Add(HeightAt(M));
					Gs.Add(GradeAt(M));
					Lo = FMath::Min(Lo, Hs.Last());
					Hi = FMath::Max(Hi, Hs.Last());
				}
				const float Span = FMath::Max(Hi - Lo, 20.f);
				auto YOf = [&](float Metres) { return PY + PH * (0.22f + 0.70f * (1.f - (Metres - Lo) / Span)); };
				auto XOf = [&](float Metres) { return PX + PW * (Metres - From) / FMath::Max(To - From, 1.f); };
				const float Col = PW / Columns;
				const float RiderX = XOf(Here < From ? Here + LengthM : Here);
				TArray<FVector2f> Line;
				for (int32 C = 0; C < Columns; ++C)
				{
					const float Cx = PX + C * Col;
					const float Cy = YOf(0.5f * (Hs[C] + Hs[C + 1]));
					P.Rect(Cx, Cy, Col + 0.4f, PY + PH - Cy, GradeColour(Gs[C]));
					if (bClimbing && Cx + Col <= RiderX)
					{
						P.Rect(Cx, Cy, Col + 0.4f, PY + PH - Cy, Hex(0x0E1012, 0.45f));
					}
				}
				for (int32 C = 0; C <= Columns; ++C)
				{
					Line.Add(FVector2f(PX + C * Col, YOf(Hs[C])));
				}
				P.Rect(PX, PY + PH - 1.f, PW, 1.f, Hex(0xF4F1EA, 0.2f));
				P.Lines(MoveTemp(Line), Ink, 2.f);
				if (Climb)
				{
					const float TopX = XOf(Peak);
					if (TopX > PX + 20.f && TopX < PX + PW - 20.f)
					{
						const FString Mark = bClimbing ? Veloverse::Units::Rise(HeightAt(Peak) - HeightAt(Here)) + TEXT(" to go")
						                               : TEXT("+") + Veloverse::Units::Rise(Climb->Rise);
						const FSlateFontInfo MarkFont = P.Font("Bold", 11.f);
						const float Half = P.Width(Mark, MarkFont) * 0.5f;
						P.Text(FMath::Clamp(TopX, PX + Half, PX + PW - Half), FMath::Max(YOf(HeightAt(Peak)) - 6.f, PY + 9.f), Mark, MarkFont,
						       Ink, EAlign::Centre);
					}
				}
				if (RiderX >= PX && RiderX <= PX + PW)
				{
					P.Rect(RiderX - 1.f, PY + 4.f, 2.f, PH - 5.f, FLinearColor::White);
					P.Round(RiderX - 5.f, YOf(HeightAt(Here)) - 5.f, 10.f, 10.f, 5.f, FLinearColor::White, Hex(0x141414), 2.f);
				}
			}

			// The loop: the stretch you're in and the next, over a bar of the stretches with how far round.
			const TArray<FRideStretch>& Stretches = Track.Stretches;
			int32 In = INDEX_NONE;
			for (int32 I = 0; I < Stretches.Num(); ++I)
			{
				In = Stretches[I].Metres <= Here ? I : In;
			}
			const FString Name = In == INDEX_NONE ? FString(TEXT("Loop")) : Stretches[In].Name;
			FString NextName, NextIn;
			if (Stretches.IsValidIndex(In + 1))
			{
				NextName = Stretches[In + 1].Name.ToLower();
				NextIn = Km(Stretches[In + 1].Metres - Here);
			}
			else if (bClosed && Stretches.Num() > 1)
			{
				NextName = Stretches[0].Name.ToLower();
				NextIn = Km(LengthM - Here + Stretches[0].Metres);
			}
			else if (!bClosed)
			{
				NextName = TEXT("finish");
				NextIn = Km(LengthM - Here);
			}
			const FSlateFontInfo NameFont = P.Font("Bold", 12.f, 0.06f);
			const float Line = Top + 104.f;
			const float NameW = P.Text(X0, Line, Name.ToUpper(), NameFont, Ink);
			if (!NextIn.IsEmpty())
			{
				// As much of "· desert in 23.6 km" as fits.
				const FString Fits[] = {FString::Printf(TEXT("· %s in %s"), *NextName, *NextIn), FString::Printf(TEXT("· %s %s"), *NextName, *NextIn),
				                        FString::Printf(TEXT("· %s"), *NextIn)};
				for (const FString& Next : Fits)
				{
					if (NameW + Space + P.Width(Next, Soft) <= Inner || &Next == &Fits[2])
					{
						P.Text(X0 + NameW + Space, Line, Next, Soft, Dim);
						break;
					}
				}
			}
			const float Gap = 3.f, Foot = Top + 116.f;
			const int32 Parts = FMath::Max(Stretches.Num(), 1);
			const float Usable = Inner - Gap * (Parts - 1);
			float Sx = X0;
			for (int32 I = 0; I < Parts; ++I)
			{
				const float A = Stretches.IsValidIndex(I) ? Stretches[I].Metres : 0.f;
				const float B = Stretches.IsValidIndex(I + 1) ? Stretches[I + 1].Metres : LengthM;
				const float Wd = Usable * FMath::Max(B - A, 0.f) / FMath::Max(LengthM, 1.f);
				const bool bHere = I == In || (In == INDEX_NONE && I == 0);
				P.Round(Sx, Foot - (bHere ? 9.f : 5.f), Wd, bHere ? 9.f : 5.f, 2.f, bHere ? Accent : Hex(0xF4F1EA, 0.25f));
				Sx += Wd + Gap;
			}
			// Where you are along it, a tick through the bar.
			const float Done = Usable * FMath::Clamp(Here / FMath::Max(LengthM, 1.f), 0.f, 1.f) + Gap * FMath::Max(In, 0);
			P.Rect(X0 + Done - 1.f, Foot - 12.f, 2.f, 12.f, FLinearColor::White);
			WeatherTop = Top + Tall + 8.f;
		}

		// ---- Daytime & Weather switchers under the minimap (fixed equal widths = minimap width) ----
		{
			const int32 Fixed = ARidePawn::Automated() ? 0 : Pawn->Profile.Weather;
			auto SkyAt = [&Track, Fixed](float M) {
				return Fixed > 0 ? static_cast<ESky>(FMath::Clamp(Fixed - 1, 0, 2)) : SkyOf(Track.WeatherAt(M));
			};
			const ESky Now = SkyAt(Here);
			const TCHAR* TimeNames[] = {TEXT("Auto"), TEXT("Morning"), TEXT("Midday"), TEXT("Evening")};
			const TCHAR* WeatherNames[] = {TEXT("Auto"), TEXT("Clear"), TEXT("Overcast"), TEXT("Rain")};
			const int32 TimeIdx = FMath::Clamp(Pawn->Profile.TimeOfDay, 0, 3);
			const int32 WeatherIdx = FMath::Clamp(Fixed, 0, 3);
			const FSlateFontInfo Big = P.Font("Bold", 13.f);
			const float Gap = 8.f;
			const float PillW = (MapPanelWide - Gap) * 0.5f; // 130 + 8 + 130 = 268 (static!)
			const float Y = WeatherTop;
			const float TimeX = W - 20.f - MapPanelWide;
			const float WeatherX = TimeX + PillW + Gap;

			// Left pill: Daytime switcher
			P.Panel(TimeX, Y, PillW, 36.f, 12.f);
			Pawn->TimePill = {TimeX, Y, PillW, 36.f};
			if (Pawn->bTimeHover)
			{
				P.Round(TimeX, Y, PillW, 36.f, 12.f, Hex(0xF4F1EA, 0.12f));
			}
			P.Icon(VeloverseMenu::Icons::Sun, TimeX + 10.f, Y + 8.f, 20.f, Hex(0xF2C864));
			P.Text(TimeX + 38.f, Y + 22.5f, TimeNames[TimeIdx], Big, SkyText);

			// Right pill: Weather switcher
			P.Panel(WeatherX, Y, PillW, 36.f, 12.f);
			Pawn->WeatherPill = {WeatherX, Y, PillW, 36.f};
			if (Pawn->bWeatherHover)
			{
				P.Round(WeatherX, Y, PillW, 36.f, 12.f, Hex(0xF4F1EA, 0.12f));
			}
			Icon(P, WeatherX + 10.f, Y + 7.f, Now);
			P.Text(WeatherX + 38.f, Y + 22.5f, WeatherNames[WeatherIdx], Big, SkyText);
		}

		// ---- Bottom left: how to pause, while the dock's away ----------------------------------------
		if (!Pawn->Dock.Visible())
		{
			const FString Hint = Pawn->bFreeLook ? TEXT("Tab · Dock    Esc · Pause") : TEXT("Esc · Pause");
			const FSlateFontInfo Font = P.Font("Bold", 11.f);
			const float Wide = P.Width(Hint, Font) + 28.f, Tall = 32.f, X = 20.f, Y = Dock.Floor - 20.f - Tall;
			P.Panel(X, Y, Wide, Tall, 16.f);
			P.Text(X + Wide * 0.5f, Y + 20.f, Hint, Font, Ink, EAlign::Centre);
		}

		// ---- Bottom middle: the workout ---------------------------------------------------------------
		// The steps as blocks as tall as their target and coloured by zone, a cursor at now; over them
		// the step with its target and time left, and what's next.
		if (Pawn->Workout.Active())
		{
			const Veloverse::FWorkoutPlayer& Player = Pawn->Workout;
			const Veloverse::FWorkout& Plan = Player.Workout();
			const float Wide = FMath::Min(620.f, W - 40.f), Tall = 112.f, X = (W - Wide) * 0.5f, Y = Dock.Clear() - 20.f - Tall;
			const float X0 = X + 16.f, Inner = Wide - 32.f;
			P.Panel(X, Y, Wide, Tall, 14.f);
			const FSlateFontInfo Label = P.Font("Bold", 11.f, 0.08f);
			const FSlateFontInfo Big = P.Font("Bold", 20.f);
			const FSlateFontInfo Soft = P.Font("Medium", 12.f);
			const Veloverse::FWorkoutStep* Step = Player.Step();
			const TOptional<double> Target = Pawn->WorkoutTarget();
			FString Head = UTF8_TO_TCHAR(Plan.Name.c_str());
			Head = Head.ToUpper();
			P.Text(X0, Y + 22.f, Head, Label, Dim);
			FString Mode = Pawn->bErg ? TEXT("ERG") : TEXT("ERG OFF");
			if (!FMath::IsNearlyZero(Pawn->WorkoutBias))
			{
				Mode = FString::Printf(TEXT("%+.0f%%  "), Pawn->WorkoutBias * 100.f) + Mode;
			}
			const float ModeW = P.Text(X0 + Inner, Y + 22.f, Mode, Label, Pawn->bErg ? Accent : Dim, EAlign::Right);
			if (Player.Done() || !Step)
			{
				P.Text(X0, Y + 50.f, TEXT("Workout done"), Big, Ink);
			}
			else
			{
				const FString What = Step->Label.empty() ? FString(TEXT("Ride")) : FString(UTF8_TO_TCHAR(Step->Label.c_str()));
				const FString Aim = Target ? FString::Printf(TEXT("%.0f W"), *Target) : FString(TEXT("Free ride"));
				float Tx = X0 + P.Text(X0, Y + 50.f, Aim, Big, Target ? Hex(ZoneHex[Pawn->Zone(*Target)]) : Ink);
				Tx += 10.f + P.Text(Tx + 10.f, Y + 50.f, What, Soft, Ink);
				if (Step->Cadence > 0.0)
				{
					P.Text(Tx + 10.f, Y + 50.f, FString::Printf(TEXT("%.0f rpm"), Step->Cadence), Soft, Dim);
				}
				P.Text(X0 + Inner, Y + 50.f, Clock(static_cast<float>(Player.StepLeft())), Big, Ink, EAlign::Right);
				if (const Veloverse::FWorkoutStep* Next = Player.NextStep())
				{
					const FString Then = Next->bFree ? FString(TEXT("free"))
					                                 : FString::Printf(TEXT("%.0f W"), Next->Start * (1.0 + Pawn->WorkoutBias) * Pawn->Ftp);
					const FString Line = FString::Printf(TEXT("then %s for %s"), *Then, *Clock(static_cast<float>(Next->Seconds)));
					P.Text(X0 + Inner - ModeW - 18.f, Y + 22.f, Line, Soft, Dim, EAlign::Right);
				}
			}
			// The blocks, one column every 2 units, against the hardest share in the plan.
			const double Total = FMath::Max(Plan.Seconds(), 1.0);
			double Peak = 1.2;
			for (const Veloverse::FWorkoutStep& S : Plan.Steps)
			{
				Peak = FMath::Max(Peak, FMath::Max(S.Start, S.End));
			}
			const float Foot = Y + Tall - 12.f, Room = 36.f, Col = 2.f;
			size_t Index = 0;
			double StepStart = 0.0;
			for (float Cx = 0.f; Cx < Inner; Cx += Col)
			{
				const double T = (Cx + Col * 0.5f) / Inner * Total;
				while (Index + 1 < Plan.Steps.size() && T >= StepStart + Plan.Steps[Index].Seconds)
				{
					StepStart += Plan.Steps[Index].Seconds;
					++Index;
				}
				const Veloverse::FWorkoutStep& S = Plan.Steps[Index];
				const double Share = S.bFree ? 0.5 : S.At(T - StepStart);
				const float Tall2 = FMath::Max(3.f, static_cast<float>(Share / Peak) * Room);
				const bool bPast = T < Player.Elapsed();
				const FLinearColor Colour = S.bFree ? Hex(0xF4F1EA, 0.3f) : Hex(ZoneHex[Pawn->Zone(static_cast<float>(Share * Pawn->Ftp))], bPast ? 0.45f : 0.95f);
				P.Rect(X0 + Cx, Foot - Tall2, Col, Tall2, Colour);
			}
			const float Now = Inner * FMath::Clamp(static_cast<float>(Player.Elapsed() / Total), 0.f, 1.f);
			P.Rect(X0 + Now - 1.f, Foot - Room - 4.f, 2.f, Room + 4.f, FLinearColor::White);
		}

		// ---- Top middle: the ride so far -------------------------------------------------------------
		{
			struct FItem
			{
				const TCHAR* Label;
				FString Value, Unit;
				const TCHAR* Room; // the widest value the column makes room for, so the panel holds its width
			};
			TArray<FItem> Items = {
				{TEXT("SPEED"), Veloverse::Units::Number(Veloverse::Units::Speed(Pawn->Speed), 1), Veloverse::Units::SpeedUnit(), TEXT("00.0")},
				{TEXT("CADENCE"), FString::Printf(TEXT("%.0f"), Pawn->Reading.CadenceRpm), TEXT("rpm"), TEXT("000")},
			};
			if (Pawn->Reading.HeartRateBpm > 0.0)
			{
				Items.Add({TEXT("HEART"), FString::Printf(TEXT("%.0f"), Pawn->Reading.HeartRateBpm), TEXT("bpm"), TEXT("000")});
			}
			Items.Add({TEXT("TIME"), Clock(Pawn->Elapsed), TEXT(""), TEXT("0:00:00")});
			Items.Add({TEXT("DISTANCE"), Veloverse::Units::Number(Veloverse::Units::Long(Pawn->Ridden() / 100.0), 1),
			           Veloverse::Units::LongUnit(), TEXT("000.0")});
			Items.Add({TEXT("CLIMBED"), Veloverse::Units::Number(Veloverse::Units::Height(Pawn->ClimbedMetres), 0),
			           Veloverse::Units::HeightUnit(), TEXT("0000")});
			const FSlateFontInfo Label = P.Font("Bold", 11.f, 0.08f);
			const FSlateFontInfo Value = P.Font("Bold", 20.f);
			const FSlateFontInfo Unit = P.Font("Medium", 13.f);
			// The font's digits are proportional ("1" is narrower than "0"), so a value measured as it
			// is made the panel breathe as the numbers changed (the user, 14 Nov). Digits are drawn in
			// cells as wide as the widest digit (tabular figures), each column is as wide as its Room,
			// and each value sits right-aligned in that room, so the units stay put too.
			float DigitW = 0.f;
			for (TCHAR C = TEXT('0'); C <= TEXT('9'); ++C)
			{
				DigitW = FMath::Max(DigitW, P.Width(FString::Chr(C), Value));
			}
			auto Figures = [&](const FString& Text, const float* DrawX, float DrawY)
			{
				float Run = 0.f;
				FString Rest;
				auto Flush = [&]
				{
					if (DrawX && !Rest.IsEmpty())
					{
						P.Text(*DrawX + Run, DrawY, Rest, Value, Ink);
					}
					Run += P.Width(Rest, Value);
					Rest.Reset();
				};
				for (const TCHAR C : Text)
				{
					if (!FChar::IsDigit(C))
					{
						Rest.AppendChar(C);
						continue;
					}
					Flush();
					if (DrawX)
					{
						const FString Digit = FString::Chr(C);
						P.Text(*DrawX + Run + (DigitW - P.Width(Digit, Value)) * 0.5f, DrawY, Digit, Value, Ink);
					}
					Run += DigitW;
				}
				Flush();
				return Run;
			};
			TArray<float> Widths, Rooms;
			float Total = 44.f + 28.f * (Items.Num() - 1);
			for (const FItem& Item : Items)
			{
				Rooms.Add(FMath::Max(Figures(Item.Room, nullptr, 0.f), Figures(Item.Value, nullptr, 0.f)));
				const float V = Rooms.Last() + (Item.Unit.IsEmpty() ? 0.f : 3.f + P.Width(Item.Unit, Unit));
				Widths.Add(FMath::Max(P.Width(Item.Label, Label), V));
				Total += Widths.Last();
			}
			// Level with the power and route panels, between them.
			const float Tall = 54.f, Y = 20.f;
			float X = (W - Total) * 0.5f;
			P.Panel(X, Y, Total, Tall, 12.f);
			X += 22.f;
			for (int32 I = 0; I < Items.Num(); ++I)
			{
				const FItem& Item = Items[I];
				const float Mid = X + Widths[I] * 0.5f;
				P.Text(Mid, Y + 19.f, Item.Label, Label, Dim, EAlign::Centre);
				const float UW = Item.Unit.IsEmpty() ? 0.f : 3.f + P.Width(Item.Unit, Unit);
				const float Left = Mid - (Rooms[I] + UW) * 0.5f;
				const float VX = Left + Rooms[I] - Figures(Item.Value, nullptr, 0.f);
				Figures(Item.Value, &VX, Y + 43.f);
				if (!Item.Unit.IsEmpty())
				{
					P.Text(Left + Rooms[I] + 3.f, Y + 43.f, Item.Unit, Unit, Ink);
				}
				X += Widths[I] + 28.f;
			}
		}

		// ---- Under the ride: the segment -------------------------------------------------------------
		// A climb coming up, the effort on it against the best (the ghost), or for a while after the top,
		// how it went.
		{
			const FSlateFontInfo Label = P.Font("Bold", 11.f, 0.08f);
			const FSlateFontInfo Name = P.Font("Bold", 16.f);
			const FSlateFontInfo Soft = P.Font("Medium", 13.f);
			const FLinearColor Ahead = Hex(0x3CCB7F), Behind = Hex(0xF0503C), Gold = Hex(0xF7D046);
			const float Wide = 420.f, Tall = 50.f, X = (W - Wide) * 0.5f, Y = 82.f;
			const TArray<FSegmentResult>& Results = Pawn->Segments.Results();
			const FSegmentResult* Recent =
				Results.Num() && Pawn->Elapsed - Results.Last().FinishedAt < 10.0 ? &Results.Last() : nullptr;
			const TOptional<FRideSegments::FLive> Live = Pawn->Segments.Live(Pawn->Distance / 100.0);
			auto Time = [](double Seconds)
			{
				const int32 S = FMath::FloorToInt32(Seconds);
				return S >= 3600 ? Clock(static_cast<float>(Seconds)) : FString::Printf(TEXT("%d:%02d"), S / 60, S % 60);
			};
			if (Recent)
			{
				P.Panel(X, Y, Wide, Tall, 12.f);
				const bool bPr = Recent->Ranking.Pr(Recent->Effort.Seconds) || Recent->Ranking.Of == 1; // a first time sets the best
				P.Text(X + 16.f, Y + 19.f, bPr ? TEXT("NEW BEST") : TEXT("SEGMENT DONE"), Label, bPr ? Gold : Dim);
				P.Text(X + 16.f, Y + 41.f, Recent->Name, Name, Ink);
				FString Line = FString::Printf(TEXT("%s · %.0f W"), *Time(Recent->Effort.Seconds), Recent->Effort.AverageWatts);
				if (bPr && Recent->Ranking.PreviousBest)
				{
					Line += FString::Printf(TEXT(" · %.1f s faster"), *Recent->Ranking.PreviousBest - Recent->Effort.Seconds);
				}
				else if (!bPr)
				{
					Line += FString::Printf(TEXT(" · %d of %d"), Recent->Ranking.Rank, Recent->Ranking.Of);
				}
				P.Text(X + Wide - 16.f, Y + 41.f, Line, Soft, bPr ? Gold : Ink, EAlign::Right);
			}
			else if (Live && Live->bOn)
			{
				P.Panel(X, Y, Wide, Tall, 12.f);
				P.Text(X + 16.f, Y + 19.f, FString::Printf(TEXT("%s · %s TO GO"), *Live->Name.ToUpper(), *Km(static_cast<float>(Live->ToGo)).ToUpper()),
				       Label, Dim);
				P.Text(X + 16.f, Y + 41.f, Time(Live->Elapsed), Name, Ink);
				if (Live->Gap)
				{
					const double Gap = *Live->Gap;
					const FString Text = FString::Printf(TEXT("%s%s vs best"), Gap <= 0.0 ? TEXT("-") : TEXT("+"),
					                                     FMath::Abs(Gap) < 60.0
					                                         ? *FString::Printf(TEXT("%.1f s"), FMath::Abs(Gap))
					                                         : *Time(FMath::Abs(Gap)));
					P.Text(X + Wide - 16.f, Y + 41.f, Text, Name, Gap <= 0.0 ? Ahead : Behind, EAlign::Right);
				}
				else
				{
					P.Text(X + Wide - 16.f, Y + 41.f, TEXT("first time up"), Soft, Dim, EAlign::Right);
				}
				// How far up, a bar along the foot.
				const float Done = FMath::Clamp(static_cast<float>(1.0 - Live->ToGo / FMath::Max(Live->Length, 1.0)), 0.f, 1.f);
				P.Round(X + 12.f, Y + Tall - 5.f, (Wide - 24.f) * Done, 3.f, 1.5f, Accent);
			}
			else if (Live)
			{
				P.Panel(X, Y, Wide, Tall, 12.f);
				P.Text(X + 16.f, Y + 19.f, FString::Printf(TEXT("SEGMENT IN %s"), *Km(static_cast<float>(Live->ToGo)).ToUpper()), Label, Accent);
				P.Text(X + 16.f, Y + 41.f, Live->Name, Name, Ink);
				FString Line = FString::Printf(TEXT("%s at %.1f%%"), *Km(static_cast<float>(Live->Length)), Live->Grade * 100.0);
				if (Live->Best)
				{
					Line += FString::Printf(TEXT(" · best %s"), *Time(*Live->Best));
				}
				P.Text(X + Wide - 16.f, Y + 41.f, Line, Soft, Dim, EAlign::Right);
			}
		}
	}

	// ---- Bottom right: the frame rate, if the rider asked for it (Graphics), with or without the HUD --
	if (Pawn->Profile.bFps && FrameSeconds > 0.f)
	{
		const FString Text = FString::Printf(TEXT("%.0f FPS  ·  %.1f ms"), 1.f / FrameSeconds, FrameSeconds * 1000.f);
		const FSlateFontInfo Font = P.Font("Bold", 11.f);
		const float Wide = P.Width(Text, Font) + 28.f, Tall = 32.f, X = W - 20.f - Wide, Y = Dock.Floor - 20.f - Tall;
		P.Panel(X, Y, Wide, Tall, 16.f);
		P.Text(X + Wide * 0.5f, Y + 20.f, Text, Font, Ink, EAlign::Centre);
	}

	// ---- Bottom middle: the dock, over everything ------------------------------------------------
	if (Dock.Shown > 0.f)
	{
		PaintDock(P, Dock, Accent);
	}
	// ---- Route-switch streaming curtain: hides grass/Nanite pop-in while the new route settles ----
	if (Pawn->RouteLoadTimer > 0.f)
	{
		const float Alpha = FMath::Clamp(Pawn->RouteLoadTimer / 0.65f, 0.f, 1.f);
		const float Progress = 1.f - FMath::Clamp(Pawn->RouteLoadTimer / ARidePawn::RouteLoadDuration, 0.f, 1.f);
		P.Rect(0.f, 0.f, W, H, Hex(0x0B0E0D, 0.96f * Alpha));
		const FString Title = Pawn->RouteLoadTitle.IsEmpty() ? FString(TEXT("Loading route")) : Pawn->RouteLoadTitle;
		P.Text(W * 0.5f, H * 0.45f, Title, P.Serif(38.f), WithAlpha(Ink, Alpha), EAlign::Centre);
		const float BarW = 280.f, BarH = 5.f;
		const float BarX = (W - BarW) * 0.5f, BarY = H * 0.45f + 28.f;
		P.Round(BarX, BarY, BarW, BarH, 3.f, Hex(0xF4F1EA, 0.14f * Alpha));
		P.Round(BarX, BarY, BarW * FMath::Clamp(Progress, 0.06f, 1.f), BarH, 3.f, WithAlpha(Accent, Alpha));
	}
	return P.Layer;
}

void URideOverlay::PaintGraphs(const FPainter& P, float W, float H) const
{
	// Up to ten minutes across the whole foot of the screen, behind the dock: power as a line in the
	// colour of each moment's zone, heart rate as a red one on its own scale. Lines only, no labels
	// or backing; a faint dark under-stroke keeps them readable over bright ground. The ride starts
	// at the left and runs right; past ten minutes the oldest scrolls off the left (the user, 15 Nov).
	const ARidePawn* Pawn = Rider();
	const std::vector<Veloverse::FSample>& Samples = Pawn->Recorder.Samples();
	if (Samples.size() < 2)
	{
		return;
	}
	constexpr double Window = 600.0;
	const float PlotTop = H - Veloverse::FDockLayout::GraphsTall + 18.f, PlotFoot = H - 6.f, PlotTall = PlotFoot - PlotTop;
	const double Now = Samples.back().Seconds;
	double Peak = Pawn->Ftp * 1.5, HrLo = 1e9, HrHi = 0.0;
	size_t First = Samples.size();
	while (First > 0 && Now - Samples[First - 1].Seconds < Window)
	{
		--First;
		const Veloverse::FSample& S = Samples[First];
		Peak = FMath::Max(Peak, S.Watts);
		if (S.HeartRate > 0.0)
		{
			HrLo = FMath::Min(HrLo, S.HeartRate);
			HrHi = FMath::Max(HrHi, S.HeartRate);
		}
	}
	const double Start = FMath::Max(Samples.front().Seconds, Now - Window);
	auto XAt = [&](double Seconds) { return static_cast<float>((Seconds - Start) / Window) * W; };
	auto Stroke = [&](const TArray<FVector2f>& Line, const FLinearColor& Colour)
	{
		if (Line.Num() > 1)
		{
			P.Lines(Line, Hex(0x080A09, 0.35f), 3.f);
			P.Lines(Line, Colour, 1.5f);
		}
	};
	// Power, in runs of one zone: each step takes the zone of the sample it ends at, and a run
	// shares its first point with the last one's end, so the line stays unbroken.
	auto PowerAt = [&](const Veloverse::FSample& S)
	{ return FVector2f(XAt(S.Seconds), PlotFoot - PlotTall * static_cast<float>(FMath::Clamp(S.Watts / Peak, 0.0, 1.0))); };
	TArray<FVector2f> Run;
	int32 RunZone = -1;
	for (size_t I = First; I < Samples.size(); ++I)
	{
		const Veloverse::FSample& S = Samples[I];
		const int32 Zone = Pawn->Zone(static_cast<float>(S.Watts));
		if (Zone != RunZone && Run.Num() > 0)
		{
			const FVector2f Joint = Run.Last();
			if (Run.Num() > 1)
			{
				Stroke(Run, Hex(ZoneHex[RunZone]));
			}
			Run.Reset();
			Run.Add(Joint);
		}
		RunZone = Zone;
		Run.Add(PowerAt(S));
	}
	if (RunZone >= 0)
	{
		Stroke(Run, Hex(ZoneHex[RunZone]));
	}
	if (HrHi > 0.0)
	{
		// Its own scale, from a little under the lowest to a little over the highest.
		const double Lo = FMath::Max(40.0, HrLo - 10.0), Hi = FMath::Max(HrHi + 10.0, Lo + 40.0);
		TArray<FVector2f> Heart;
		for (size_t I = First; I < Samples.size(); ++I)
		{
			const Veloverse::FSample& S = Samples[I];
			if (S.HeartRate > 0.0)
			{
				Heart.Add({XAt(S.Seconds), PlotFoot - PlotTall * static_cast<float>((S.HeartRate - Lo) / (Hi - Lo))});
			}
		}
		Stroke(Heart, VeloverseMenu::Heart);
	}
}

void URideOverlay::PaintDock(const FPainter& P, const Veloverse::FDockLayout& L, const FLinearColor& Accent) const
{
	using Veloverse::EDock;
	namespace Icons = VeloverseMenu::Icons;
	const ARidePawn* Pawn = Rider();
	const float A = L.Shown;
	P.Round(L.Panel.X, L.Panel.Y, L.Panel.W, L.Panel.H, 16.f, Hex(0x0E1110, 0.78f * A));
	P.Border(L.Panel.X, L.Panel.Y, L.Panel.W, L.Panel.H, 16.f, Hex(0xF4F1EA, 0.12f * A));
	const TArray<ERideView> Views = Pawn->Views();
	const TCHAR* ViewNames[] = {TEXT("Eyes"), TEXT("Over the bars"), TEXT("Behind"), TEXT("Dolly")};
	const TCHAR* MotionNames[] = {TEXT("Off"), TEXT("Low"), TEXT("Medium"), TEXT("High")};
	const int32 MotionLvl = Pawn->Profile.MotionLevel();
	struct FButton
	{
		const TCHAR* Icon;
		FString Name;
		const TCHAR* Key;
		bool bOn, bDim;
	};
	const FButton Buttons[Veloverse::DockCount] = {
		{Icons::Eye, FString::Printf(TEXT("View: %s"), ViewNames[static_cast<int32>(Pawn->View())]), TEXT("V"), false, Views.Num() < 2},
		{Icons::Bike, FString::Printf(TEXT("Motion: %s"), MotionNames[MotionLvl]), TEXT("M"), MotionLvl > 0, false},
		{Icons::Pulse, TEXT("Graphs"), TEXT("G"), Pawn->Profile.bGraphs, false},
		{Icons::Hud, TEXT("HUD"), TEXT("H"), Pawn->Profile.bHud, false},
		{Icons::Route, TEXT("Change route"), TEXT("R"), false, false},
		{Icons::Move, TEXT("Free look"), TEXT("Tab"), Pawn->bFreeLook, false},
		{Icons::Pause, TEXT("Pause"), TEXT("Esc"), false, false},
	};
	for (int32 I = 0; I < Veloverse::DockCount; ++I)
	{
		const Veloverse::FDockBox& B = L.Buttons[I];
		const FButton& Each = Buttons[I];
		const bool bHover = Pawn->DockHover == I;
		if (Each.bOn || bHover)
		{
			const FLinearColor Fill = Each.bOn ? WithAlpha(Accent, (bHover ? 0.3f : 0.2f) * A) : Hex(0xF4F1EA, 0.12f * A);
			P.Round(B.X, B.Y, B.W, B.H, 12.f, Fill);
		}
		const FLinearColor Colour =
			Each.bOn ? WithAlpha(Accent, A) : Hex(0xF4F1EA, (Each.bDim ? 0.35f : bHover ? 1.f : 0.82f) * A);
		P.Icon(Each.Icon, B.X + (B.W - 22.f) * 0.5f, B.Y + (B.H - 22.f) * 0.5f, 22.f, Colour);
	}
	// The hovered button's name and key, over it.
	if (L.Shown >= 0.6f && Pawn->DockHover >= 0 && Pawn->DockHover < Veloverse::DockCount)
	{
		const FButton& Each = Buttons[Pawn->DockHover];
		const Veloverse::FDockBox& B = L.Buttons[Pawn->DockHover];
		const FSlateFontInfo Font = P.Font("Bold", 12.f), KeyFont = P.Font("Bold", 10.f);
		const float NameW = P.Width(Each.Name, Font), KeyW = P.Width(Each.Key, KeyFont) + 10.f;
		const float Wide = 12.f + NameW + 8.f + KeyW + 8.f, Tall = 28.f;
		const float X = B.X + B.W * 0.5f - Wide * 0.5f; // the dock is centred, so this stays on screen
		const float Y = L.Panel.Y - 8.f - Tall;
		P.Round(X, Y, Wide, Tall, 8.f, Hex(0x0E1110, 0.9f * A));
		P.Text(X + 12.f, Y + 18.5f, Each.Name, Font, Hex(0xF4F1EA, A));
		P.Round(X + 12.f + NameW + 8.f, Y + 6.f, KeyW, 16.f, 4.f, Hex(0xF4F1EA, 0.16f * A));
		P.Text(X + 12.f + NameW + 8.f + KeyW * 0.5f, Y + 18.f, Each.Key, KeyFont, Hex(0xF4F1EA, 0.85f * A), EAlign::Centre);
	}
}
