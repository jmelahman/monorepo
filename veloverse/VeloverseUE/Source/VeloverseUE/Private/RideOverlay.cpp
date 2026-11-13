#include "RideOverlay.h"

#include "Blueprint/WidgetTree.h"
#include "Components/CanvasPanel.h"
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

// The weather's icon in a 22 px square at X, Y, drawn on the canvas's 24-unit grid.
void Icon(const FPainter& P, float X, float Y, ESky Weather)
{
	const float K = 22.f / 24.f;
	auto Place = [&](TArray<FVector2f> Points)
	{
		for (FVector2f& Point : Points)
		{
			Point = FVector2f(X, Y) + Point * K;
		}
		P.Lines(MoveTemp(Points), Sky, 1.8f);
	};
	if (Weather == ESky::Clear)
	{
		TArray<FVector2f> Disc;
		FPainter::Arc(Disc, FVector2f(12.f, 12.f), 4.f, 0.f, 360.f);
		Place(Disc);
		for (int32 I = 0; I < 8; ++I)
		{
			const float T = FMath::DegreesToRadians(45.f * I);
			const FVector2f D(FMath::Cos(T), FMath::Sin(T));
			Place({FVector2f(12.f, 12.f) + D * 6.5f, FVector2f(12.f, 12.f) + D * 9.f});
		}
		return;
	}
	const float Up = Weather == ESky::Rain ? 0.f : 2.f;
	TArray<FVector2f> Cloud;
	FPainter::Arc(Cloud, FVector2f(7.5f, 12.f + Up), 3.5f, 90.f, 290.f);
	FPainter::Arc(Cloud, FVector2f(12.5f, 10.f + Up), 4.5f, 200.f, 340.f);
	FPainter::Arc(Cloud, FVector2f(17.f, 12.5f + Up), 3.5f, 250.f, 450.f);
	const FVector2f First = Cloud[0];
	Cloud.Add(First);
	Place(Cloud);
	if (Weather == ESky::Rain)
	{
		for (const float Drop : {9.f, 13.f, 17.f})
		{
			Place({FVector2f(Drop, 19.f), FVector2f(Drop - 1.f, 21.f)});
		}
	}
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
	// The rider's HUD colour, unlocked with levels and picked on the profile page.
	const FLinearColor Accent = Pawn->Profile.Accent();

	// ---- Top left: power ------------------------------------------------------------------------
	{
		const int32 Zone = Pawn->Zone(Watts);
		const FLinearColor ZoneColour = Hex(ZoneHex[Zone]);
		const float X = 20.f, Y = 20.f, Wide = 196.f;
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
	{
		const float Wide = 196.f, Tall = 130.f, X = W - 20.f - Wide, Top = 20.f;
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
		P.Text(X0, Top + 25.f, bClimbing ? TEXT("THIS CLIMB") : TEXT("NEXT 3 KM"), Label, Dim);
		const FString Badge = FString::Printf(TEXT("%+.1f %%"), Grade * 100.f);
		const float BadgeW = P.Width(Badge, Strong) + 14.f;
		P.Round(Right - BadgeW, Top + 11.f, BadgeW, 18.f, 6.f, GradeColour(FMath::Max(Grade, 0.006f)));
		P.Text(Right - 7.f, Top + 25.f, Badge, Strong, BadgeText, EAlign::Right);

		// The profile: a climb from its foot to past its top, or the next 3 km.
		float From, To;
		if (bClimbing)
		{
			const float Long = FMath::Max(Peak - Start, 300.f);
			From = Start - 0.08f * Long;
			To = Peak + 0.25f * Long;
		}
		else
		{
			From = Here - 150.f;
			To = Here + 2850.f;
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

	// ---- The weather, under the panel -----------------------------------------------------------
	{
		const ESky Now = SkyOf(Track.WeatherAt(Here));
		FString Change;
		for (float Ahead = SampleM; Ahead <= 6000.f; Ahead += SampleM)
		{
			float M = Here + Ahead;
			if (M > LengthM)
			{
				if (!bClosed)
				{
					break;
				}
				M -= LengthM;
			}
			const ESky Then = SkyOf(Track.WeatherAt(M));
			if (Then != Now)
			{
				const TCHAR* What = Then == ESky::Rain ? TEXT("rain") : Now == ESky::Rain ? TEXT("clears") : Then == ESky::Clear ? TEXT("clearing") : TEXT("cloud");
				Change = FString::Printf(TEXT("%s in %s"), What, *Km(Ahead));
				break;
			}
		}
		const TCHAR* Names[] = {TEXT("Clear"), TEXT("Cloudy"), TEXT("Rain")};
		const FSlateFontInfo Big = P.Font("Bold", 14.f);
		const FSlateFontInfo Small = P.Font("Regular", 12.f);
		const float Wide = 10.f + 22.f + 8.f + P.Width(Names[static_cast<int32>(Now)], Big) +
		                   (Change.IsEmpty() ? 0.f : 8.f + P.Width(Change, Small)) + 14.f;
		const float X = W - 20.f - Wide, Y = WeatherTop;
		P.Panel(X, Y, Wide, 36.f, 12.f);
		Icon(P, X + 10.f, Y + 7.f, Now);
		const float NameW = P.Text(X + 40.f, Y + 23.f, Names[static_cast<int32>(Now)], Big, SkyText);
		if (!Change.IsEmpty())
		{
			P.Text(X + 48.f + NameW, Y + 23.f, Change, Small, Dim);
		}
	}

	// ---- Bottom middle: the workout ---------------------------------------------------------------
	// The steps as blocks as tall as their target and coloured by zone, a cursor at now; over them
	// the step with its target and time left, and what's next.
	if (Pawn->Workout.Active())
	{
		const Veloverse::FWorkoutPlayer& Player = Pawn->Workout;
		const Veloverse::FWorkout& Plan = Player.Workout();
		const float Wide = FMath::Min(620.f, W - 40.f), Tall = 112.f, X = (W - Wide) * 0.5f, Y = H - 20.f - Tall;
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
		};
		TArray<FItem> Items = {
			{TEXT("SPEED"), Veloverse::Units::Number(Veloverse::Units::Speed(Pawn->Speed), 1), Veloverse::Units::SpeedUnit()},
			{TEXT("CADENCE"), FString::Printf(TEXT("%.0f"), Pawn->Reading.CadenceRpm), TEXT("rpm")},
		};
		if (Pawn->Reading.HeartRateBpm > 0.0)
		{
			Items.Add({TEXT("HEART"), FString::Printf(TEXT("%.0f"), Pawn->Reading.HeartRateBpm), TEXT("bpm")});
		}
		Items.Add({TEXT("TIME"), Clock(Pawn->Elapsed), TEXT("")});
		Items.Add({TEXT("DISTANCE"), Veloverse::Units::Number(Veloverse::Units::Long(Pawn->Ridden() / 100.0), 1),
		           Veloverse::Units::LongUnit()});
		Items.Add({TEXT("CLIMBED"), Veloverse::Units::Number(Veloverse::Units::Height(Pawn->ClimbedMetres), 0),
		           Veloverse::Units::HeightUnit()});
		const FSlateFontInfo Label = P.Font("Bold", 11.f, 0.08f);
		const FSlateFontInfo Value = P.Font("Bold", 20.f);
		const FSlateFontInfo Unit = P.Font("Medium", 13.f);
		TArray<float> Widths;
		float Total = 44.f + 28.f * (Items.Num() - 1);
		for (const FItem& Item : Items)
		{
			const float V = P.Width(Item.Value, Value) + (Item.Unit.IsEmpty() ? 0.f : 3.f + P.Width(Item.Unit, Unit));
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
			const float VW = P.Width(Item.Value, Value);
			const float UW = Item.Unit.IsEmpty() ? 0.f : 3.f + P.Width(Item.Unit, Unit);
			const float Left = Mid - (VW + UW) * 0.5f;
			P.Text(Left, Y + 43.f, Item.Value, Value, Ink);
			if (!Item.Unit.IsEmpty())
			{
				P.Text(Left + VW + 3.f, Y + 43.f, Item.Unit, Unit, Ink);
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
	return P.Layer;
}
