#include "RideOverlay.h"

#include "Blueprint/WidgetTree.h"
#include "Brushes/SlateColorBrush.h"
#include "Brushes/SlateRoundedBoxBrush.h"
#include "Components/CanvasPanel.h"
#include "Fonts/FontMeasure.h"
#include "Framework/Application/SlateApplication.h"
#include "Rendering/DrawElements.h"
#include "Rendering/SlateRenderer.h"
#include "RidePawn.h"
#include "RideTrack.h"
#include "Styling/CoreStyle.h"

namespace
{
// The canvas's colours, written as it wrote them: sRGB hex.
FLinearColor Hex(uint32 Rgb, float Alpha = 1.f)
{
	FLinearColor C = FLinearColor::FromSRGBColor(FColor((Rgb >> 16) & 255, (Rgb >> 8) & 255, Rgb & 255));
	C.A = Alpha;
	return C;
}

const FLinearColor Ink = Hex(0xF4F1EA);
const FLinearColor Dim = Hex(0xF4F1EA, 0.72f);
const FLinearColor Fill = Hex(0x0E1012, 0.64f);
const FLinearColor Edge(1.f, 1.f, 1.f, 0.08f);
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

// Far off in kilometres, close in metres rounded to 50.
FString Km(float Metres)
{
	if (Metres >= 950.f)
	{
		return FString::Printf(TEXT("%.1f km"), Metres / 1000.f);
	}
	return FString::Printf(TEXT("%.0f m"), FMath::Max(50.f, FMath::RoundToFloat(Metres / 50.f) * 50.f));
}

// Slate drops a run's leading space, so text that follows another run starts after this gap.
constexpr float Space = 4.f;

enum class EAlign : uint8
{
	Left,
	Centre,
	Right,
};

// Draws in the canvas's units: a 1280x720 frame scaled to the screen's height, so the panels keep
// their size against the picture. Every draw goes on a layer of its own, so nothing is batched
// out of order.
struct FPainter
{
	const FGeometry& G;
	FSlateWindowElementList& Out;
	mutable int32 Layer;
	float S;
	TSharedRef<FSlateFontMeasure> Measure = FSlateApplication::Get().GetRenderer()->GetFontMeasureService();

	// Inter's weights become Roboto's: 500 Medium, 600 and 700 Bold, 800 Black. Px is the CSS size;
	// Slate sizes are points at 96 DPI.
	FSlateFontInfo Font(FName Face, float Px, float Tracking = 0.f) const
	{
		FSlateFontInfo F = FCoreStyle::GetDefaultFontStyle(Face, Px * 0.75f * S);
		F.LetterSpacing = FMath::RoundToInt32(Tracking * 1000.f);
		return F;
	}

	FPaintGeometry At(float X, float Y, float W, float H) const
	{
		return G.ToPaintGeometry(FVector2f(W * S, H * S), FSlateLayoutTransform(FVector2f(X * S, Y * S)));
	}

	void Rect(float X, float Y, float W, float H, const FLinearColor& Colour) const
	{
		const FSlateColorBrush Brush(FLinearColor::White);
		FSlateDrawElement::MakeBox(Out, ++Layer, At(X, Y, W, H), &Brush, ESlateDrawEffect::None, Colour);
	}

	void Round(float X, float Y, float W, float H, float Radius, const FLinearColor& Colour,
	           const FLinearColor& Outline = FLinearColor::Transparent, float OutlineWidth = 0.f) const
	{
		// Slate takes a box's colour from the draw's tint, not the brush's.
		const FSlateRoundedBoxBrush Brush(FLinearColor::White, Radius * S, Outline, OutlineWidth * S);
		FSlateDrawElement::MakeBox(Out, ++Layer, At(X, Y, W, H), &Brush, ESlateDrawEffect::None, Colour);
	}

	// The fill, then the edge over it as CSS draws a border. A brush's outline sits outside its fill,
	// so an outline on the fill's own brush shows as a light ring on the scene.
	void Panel(float X, float Y, float W, float H, float Radius) const
	{
		Round(X, Y, W, H, Radius, Fill);
		// The tint fills the box and the brush's outline colour draws the edge, so a clear tint is edge only.
		const FSlateRoundedBoxBrush Brush(FLinearColor::White, Radius * S, Edge, 1.f * S);
		FSlateDrawElement::MakeBox(Out, ++Layer, At(X, Y, W, H), &Brush, ESlateDrawEffect::None, FLinearColor::Transparent);
	}

	float Width(const FString& Text, const FSlateFontInfo& F) const
	{
		return Measure->Measure(Text, F, 1.f).X / S;
	}

	// Writes Text with its baseline at Y, from X, round X, or back to X; returns its width.
	float Text(float X, float Y, const FString& Text, const FSlateFontInfo& F, const FLinearColor& Colour,
	           EAlign Align = EAlign::Left) const
	{
		const FVector2D Size = Measure->Measure(Text, F, 1.f);
		const float W = Size.X / S;
		const float Ascent = (Measure->GetMaxCharacterHeight(F) + Measure->GetBaseline(F)) / S;
		const float Left = Align == EAlign::Left ? X : Align == EAlign::Centre ? X - W * 0.5f : X - W;
		FSlateDrawElement::MakeText(Out, ++Layer, At(Left, Y - Ascent, W, Size.Y / S), Text, F, ESlateDrawEffect::None,
		                            Colour);
		return W;
	}

	void Lines(TArray<FVector2f> Points, const FLinearColor& Colour, float Thickness) const
	{
		for (FVector2f& P : Points)
		{
			P *= S;
		}
		FSlateDrawElement::MakeLines(Out, ++Layer, G.ToPaintGeometry(), MoveTemp(Points), ESlateDrawEffect::None,
		                             Colour, true, Thickness * S);
	}

	// An arc round Centre, from A to B degrees, clockwise on screen (y down).
	static void Arc(TArray<FVector2f>& Points, FVector2f Centre, float R, float A, float B)
	{
		const int32 Steps = FMath::Max(4, FMath::CeilToInt32((B - A) / 15.f));
		for (int32 I = 0; I <= Steps; ++I)
		{
			const float T = FMath::DegreesToRadians(FMath::Lerp(A, B, float(I) / Steps));
			Points.Add(Centre + FVector2f(FMath::Cos(T), FMath::Sin(T)) * R);
		}
	}

	// The weather's icon in a 22 px square at X, Y, drawn on the canvas's 24-unit grid.
	void Icon(float X, float Y, ESky Weather) const
	{
		const float K = 22.f / 24.f;
		auto Place = [&](TArray<FVector2f> Points)
		{
			for (FVector2f& P : Points)
			{
				P = FVector2f(X, Y) + P * K;
			}
			Lines(MoveTemp(Points), Sky, 1.8f);
		};
		if (Weather == ESky::Clear)
		{
			TArray<FVector2f> Disc;
			Arc(Disc, FVector2f(12.f, 12.f), 4.f, 0.f, 360.f);
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
		Arc(Cloud, FVector2f(7.5f, 12.f + Up), 3.5f, 90.f, 290.f);
		Arc(Cloud, FVector2f(12.5f, 10.f + Up), 4.5f, 200.f, 340.f);
		Arc(Cloud, FVector2f(17.f, 12.5f + Up), 3.5f, 250.f, 450.f);
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
};
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
	// Climbs: runs of 2 % and steeper, joined across a short let-up, that rise 8 m or more.
	TArray<TPair<int32, int32>> Runs;
	for (int32 I = 0; I < N; ++I)
	{
		if (Grades[I] < 0.02f)
		{
			continue;
		}
		int32 End = I;
		while (End + 1 < N && Grades[End + 1] >= 0.02f)
		{
			++End;
		}
		if (!Runs.IsEmpty() && (I - Runs.Last().Value) * SampleM < 120.f &&
		    Heights[I] >= Heights[Runs.Last().Value] - 3.f)
		{
			Runs.Last().Value = End;
		}
		else
		{
			Runs.Add({I, End});
		}
		I = End;
	}
	for (const auto& [A, B] : Runs)
	{
		int32 Top = A;
		float Steepest = 0.f;
		for (int32 I = A; I <= FMath::Min(B + 1, N - 1); ++I)
		{
			Top = Heights[I] > Heights[Top] ? I : Top;
			Steepest = FMath::Max(Steepest, Grades[FMath::Min(I, B)]);
		}
		const float Rise = Heights[Top] - Heights[A];
		if (Rise >= 8.f && (Top - A) * SampleM >= 150.f)
		{
			Climbs.Add({A * SampleM, Top * SampleM, Rise, Steepest});
		}
	}
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
		if (C.Start <= Here && Here < C.Top)
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
	float WeatherTop;
	{
		const float Wide = 316.f, X = W - 20.f - Wide, X0 = X + 14.f, Inner = 288.f, Right = X0 + Inner;
		const float Top = 20.f;
		bool bOn = false;
		float Start = 0.f, Peak = 0.f;
		const FClimb* Climb = ClimbAt(Here, bOn, Start, Peak, 5000.f);
		const bool bClimbing = Climb && bOn && HeightAt(Peak) - HeightAt(Here) >= 3.f;
		const float ProfileH = bClimbing ? 76.f : 64.f;
		const float Height = 10.f + 14.5f + 6.f + (bClimbing ? 36.f : 0.f) + ProfileH + 6.f + 5.f + 6.f + 14.5f + 6.f + 5.f + 10.f;
		P.Panel(X, Top, Wide, Height, 14.f);
		float Y = Top + 10.f;
		const FSlateFontInfo Label = P.Font("Bold", 12.f, 0.08f);
		const FSlateFontInfo Strong = P.Font("Bold", 12.f);
		const FSlateFontInfo Soft = P.Font("Medium", 12.f);
		const float Grade = Pawn->Grade;
		if (bClimbing)
		{
			P.Text(X0, Y + 11.5f, TEXT("THIS CLIMB"), Label, Dim);
			const FString Rest = FString::Printf(TEXT("· %s"), *Km(Peak - Here));
			const float RestW = P.Text(Right, Y + 11.5f, Rest, Soft, Dim, EAlign::Right) + Space;
			P.Text(Right - RestW, Y + 11.5f, FString::Printf(TEXT("%.0f m to the top"), HeightAt(Peak) - HeightAt(Here)), Strong, Ink,
			       EAlign::Right);
			Y += 14.5f + 6.f;
			const float Base = Y + 26.f;
			const float Num = P.Text(X0, Base, FString::Printf(TEXT("%.1f"), FMath::Max(Grade, 0.f) * 100.f), P.Font("Black", 30.f, -0.02f), Ink);
			const float Pct = P.Text(X0 + Num + 3.f, Base, TEXT("%"), P.Font("Bold", 15.f), Ink);
			const FString Word = GradeWord(Grade);
			const float WordW = P.Width(Word, Strong) + 14.f;
			const float BadgeX = X0 + Num + 3.f + Pct + 10.f;
			P.Round(BadgeX, Base - 13.5f, WordW, 18.f, 6.f, GradeColour(FMath::Max(Grade, 0.006f)));
			P.Text(BadgeX + 7.f, Base, Word, Strong, BadgeText);
			P.Text(Right, Base, FString::Printf(TEXT("max %.1f %%"), Climb->Steepest * 100.f), Soft, Dim, EAlign::Right);
			Y += 30.f + 6.f;
		}
		else
		{
			P.Text(X0, Y + 11.5f, TEXT("NEXT 3 KM"), Label, Dim);
			const FString After = Climb ? FString::Printf(TEXT("now · climb in %s"), *Km(Start - Here)) : FString(TEXT("now"));
			const float AfterW = P.Text(Right, Y + 11.5f, After, Soft, Dim, EAlign::Right) + Space;
			P.Text(Right - AfterW, Y + 11.5f, FString::Printf(TEXT("%+.1f %%"), Grade * 100.f), Strong, Ink, EAlign::Right);
			Y += 14.5f + 6.f;
		}

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
			const float PX = X0, PY = Y, PW = Inner, PH = ProfileH;
			constexpr int32 Columns = 144;
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
			auto YOf = [&](float Metres) { return PY + PH * (0.16f + 0.72f * (1.f - (Metres - Lo) / Span)); };
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
				if (TopX > PX + 12.f && TopX < PX + PW - 12.f)
				{
					const FString Mark = bClimbing ? FString(TEXT("top")) : FString::Printf(TEXT("+%.0f m"), Climb->Rise);
					P.Text(TopX, FMath::Max(YOf(HeightAt(Peak)) - 6.f, PY + 10.f), Mark, P.Font("Bold", 11.f), Ink, EAlign::Centre);
				}
			}
			if (RiderX >= PX && RiderX <= PX + PW)
			{
				P.Rect(RiderX - 1.f, PY + 4.f, 2.f, PH - 5.f, FLinearColor::White);
				P.Round(RiderX - 6.f, YOf(HeightAt(Here)) - 6.f, 12.f, 12.f, 6.f, FLinearColor::White, Hex(0x141414), 2.f);
			}
			Y += PH + 6.f;
		}

		// The loop: the stretch you're in, the next, and how far round.
		P.Rect(X0, Y + 2.f, Inner, 1.f, Hex(0xF4F1EA, 0.12f));
		Y += 5.f + 6.f;
		const TArray<FRideStretch>& Stretches = Track.Stretches;
		int32 In = INDEX_NONE;
		for (int32 I = 0; I < Stretches.Num(); ++I)
		{
			In = Stretches[I].Metres <= Here ? I : In;
		}
		FString Name = In == INDEX_NONE ? FString(TEXT("Loop")) : Stretches[In].Name;
		FString Next;
		if (Stretches.IsValidIndex(In + 1))
		{
			Next = FString::Printf(TEXT("· %s in %s"), *Stretches[In + 1].Name.ToLower(), *Km(Stretches[In + 1].Metres - Here));
		}
		else if (bClosed && Stretches.Num() > 1)
		{
			Next = FString::Printf(TEXT("· %s in %s"), *Stretches[0].Name.ToLower(),
			                       *Km(LengthM - Here + Stretches[0].Metres));
		}
		else if (!bClosed)
		{
			Next = FString::Printf(TEXT("· finish in %s"), *Km(LengthM - Here));
		}
		const float NameW = P.Text(X0, Y + 11.5f, Name.ToUpper(), P.Font("Bold", 12.f, 0.06f), Ink);
		P.Text(X0 + NameW + Space, Y + 11.5f, Next, Soft, Dim);
		const float OfW = P.Text(Right, Y + 11.5f, FString::Printf(TEXT("/ %.1f km"), LengthM / 1000.f), Soft, Dim, EAlign::Right) + Space;
		P.Text(Right - OfW, Y + 11.5f, FString::Printf(TEXT("%.1f"), Here / 1000.f), Strong, Ink, EAlign::Right);
		Y += 14.5f + 6.f;
		const float Gap = 2.f;
		const int32 Parts = FMath::Max(Stretches.Num(), 1);
		const float Usable = Inner - Gap * (Parts - 1);
		float Sx = X0;
		for (int32 I = 0; I < Parts; ++I)
		{
			const float A = Stretches.IsValidIndex(I) ? Stretches[I].Metres : 0.f;
			const float B = Stretches.IsValidIndex(I + 1) ? Stretches[I + 1].Metres : LengthM;
			const float Wd = Usable * FMath::Max(B - A, 0.f) / FMath::Max(LengthM, 1.f);
			P.Rect(Sx, Y, Wd, 5.f, Hex(0xF4F1EA, 0.28f));
			Sx += Wd + Gap;
		}
		const float Done = Inner * FMath::Clamp(Here / FMath::Max(LengthM, 1.f), 0.f, 1.f);
		P.Rect(X0, Y, Done, 5.f, Hex(0xF4F1EA, 0.6f));
		P.Rect(X0 + Done - 1.f, Y, 3.f, 5.f, FLinearColor::White);
		WeatherTop = Top + Height + 8.f;
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
		P.Icon(X + 10.f, Y + 7.f, Now);
		const float NameW = P.Text(X + 40.f, Y + 23.f, Names[static_cast<int32>(Now)], Big, SkyText);
		if (!Change.IsEmpty())
		{
			P.Text(X + 48.f + NameW, Y + 23.f, Change, Small, Dim);
		}
	}

	// ---- Top middle: the ride so far -------------------------------------------------------------
	{
		struct FItem
		{
			const TCHAR* Label;
			FString Value, Unit;
		};
		TArray<FItem> Items = {
			{TEXT("SPEED"), FString::Printf(TEXT("%.1f"), Pawn->Speed * 3.6f), TEXT("km/h")},
			{TEXT("CADENCE"), FString::Printf(TEXT("%.0f"), Pawn->Reading.CadenceRpm), TEXT("rpm")},
		};
		if (Pawn->Reading.HeartRateBpm > 0.0)
		{
			Items.Add({TEXT("HEART"), FString::Printf(TEXT("%.0f"), Pawn->Reading.HeartRateBpm), TEXT("bpm")});
		}
		Items.Add({TEXT("TIME"), Clock(Pawn->Elapsed), TEXT("")});
		Items.Add({TEXT("CLIMBED"), FString::Printf(TEXT("%.0f"), Pawn->ClimbedMetres), TEXT("m")});
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
	return P.Layer;
}
