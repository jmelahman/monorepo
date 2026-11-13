// Drawing in a design frame's units for widgets painted in code (the HUD and the menus): boxes,
// rounded panels, text with its baseline where asked, lines, gradients, images and the 24-unit SVG
// icons the design canvas draws. The frame is scaled to the screen's height, so what's drawn keeps
// its size against the picture: S is screen pixels per frame unit.
#pragma once

#include "CoreMinimal.h"
#include "Brushes/SlateColorBrush.h"
#include "Brushes/SlateRoundedBoxBrush.h"
#include "Fonts/FontMeasure.h"
#include "Framework/Application/SlateApplication.h"
#include "Misc/Paths.h"
#include "Rendering/DrawElements.h"
#include "Rendering/SlateRenderer.h"
#include "Styling/CoreStyle.h"

namespace VeloversePaint
{
// The canvas's colours, written as it wrote them: sRGB hex.
inline FLinearColor Hex(uint32 Rgb, float Alpha = 1.f)
{
	FLinearColor C = FLinearColor::FromSRGBColor(FColor((Rgb >> 16) & 255, (Rgb >> 8) & 255, Rgb & 255));
	C.A = Alpha;
	return C;
}

inline FLinearColor WithAlpha(FLinearColor C, float Alpha)
{
	C.A = Alpha;
	return C;
}

enum class EAlign : uint8
{
	Left,
	Centre,
	Right,
};

// Slate drops a run's leading space, so text that follows another run starts after this gap.
constexpr float Space = 4.f;

// Fraunces SemiBold, the titles' serif, from Content/Menu (scripts/make_menu_map.py copies it there);
// Roboto Bold when it isn't there.
inline const FString& SerifFile()
{
	static const FString File = FPaths::ConvertRelativePathToFull(FPaths::ProjectContentDir() / TEXT("Menu/Fraunces-SemiBold.ttf"));
	return File;
}

// Every draw goes on a layer of its own, so nothing is batched out of order.
struct FPainter
{
	const FGeometry& G;
	FSlateWindowElementList& Out;
	mutable int32 Layer;
	float S;
	TSharedRef<FSlateFontMeasure> Measure = FSlateApplication::Get().GetRenderer()->GetFontMeasureService();
	// What Panel draws: its fill, and the 1 px edge over it.
	FLinearColor PanelFill = Hex(0x0E1012, 0.64f);
	FLinearColor PanelEdge = FLinearColor(1.f, 1.f, 1.f, 0.08f);

	// Inter's weights become Roboto's: 500 Medium, 600 and 700 Bold, 800 Black. Px is the CSS size;
	// Slate sizes are points at 96 DPI. Tracking is in ems.
	FSlateFontInfo Font(FName Face, float Px, float Tracking = 0.f) const
	{
		FSlateFontInfo F = FCoreStyle::GetDefaultFontStyle(Face, Px * 0.75f * S);
		F.LetterSpacing = FMath::RoundToInt32(Tracking * 1000.f);
		return F;
	}

	FSlateFontInfo Serif(float Px) const
	{
		static const bool bThere = FPaths::FileExists(SerifFile());
		if (!bThere)
		{
			return Font("Bold", Px);
		}
		return FSlateFontInfo(SerifFile(), Px * 0.75f * S);
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

	// An edge only, as CSS draws a border: inside the box.
	void Border(float X, float Y, float W, float H, float Radius, const FLinearColor& Colour, float Width = 1.f) const
	{
		// The tint fills the box and the brush's outline colour draws the edge, so a clear tint is edge only.
		const FSlateRoundedBoxBrush Brush(FLinearColor::White, Radius * S, Colour, Width * S);
		FSlateDrawElement::MakeBox(Out, ++Layer, At(X, Y, W, H), &Brush, ESlateDrawEffect::None, FLinearColor::Transparent);
	}

	// The fill, then the edge over it. A brush's outline sits outside its fill, so an outline on the
	// fill's own brush shows as a light ring on the scene.
	void Panel(float X, float Y, float W, float H, float Radius) const
	{
		Round(X, Y, W, H, Radius, PanelFill);
		Border(X, Y, W, H, Radius, PanelEdge);
	}

	void Disc(float Cx, float Cy, float R, const FLinearColor& Colour) const { Round(Cx - R, Cy - R, 2.f * R, 2.f * R, R, Colour); }

	// A gradient across the box: left to right (bAcross) or top to bottom, through Stops at shares
	// of the way.
	void Gradient(float X, float Y, float W, float H, const TArray<TPair<float, FLinearColor>>& Stops, bool bAcross) const
	{
		TArray<FSlateGradientStop> Slate;
		for (const TPair<float, FLinearColor>& Stop : Stops)
		{
			const FVector2f Pos = bAcross ? FVector2f(Stop.Key * W * S, 0.f) : FVector2f(0.f, Stop.Key * H * S);
			Slate.Add(FSlateGradientStop(Pos, Stop.Value));
		}
		FSlateDrawElement::MakeGradient(Out, ++Layer, At(X, Y, W, H), MoveTemp(Slate), bAcross ? Orient_Vertical : Orient_Horizontal);
	}

	void Image(float X, float Y, float W, float H, const FSlateBrush* Brush, const FLinearColor& Tint = FLinearColor::White) const
	{
		FSlateDrawElement::MakeBox(Out, ++Layer, At(X, Y, W, H), Brush, ESlateDrawEffect::None, Tint);
	}

	float Width(const FString& Text, const FSlateFontInfo& F) const { return Measure->Measure(Text, F, 1.f).X / S; }

	// Writes Text with its baseline at Y, from X, round X, or back to X; returns its width.
	float Text(float X, float Y, const FString& Text, const FSlateFontInfo& F, const FLinearColor& Colour,
	           EAlign Align = EAlign::Left) const
	{
		if (Text.IsEmpty())
		{
			return 0.f;
		}
		const FVector2D Size = Measure->Measure(Text, F, 1.f);
		const float W = Size.X / S;
		const float Ascent = (Measure->GetMaxCharacterHeight(F) + Measure->GetBaseline(F)) / S;
		const float Left = Align == EAlign::Left ? X : Align == EAlign::Centre ? X - W * 0.5f : X - W;
		FSlateDrawElement::MakeText(Out, ++Layer, At(Left, Y - Ascent, W, Size.Y / S), Text, F, ESlateDrawEffect::None,
		                            Colour);
		return W;
	}

	// Text cut to Width with an ellipsis.
	FString Fit(const FString& Text, const FSlateFontInfo& F, float MaxWidth) const
	{
		if (Width(Text, F) <= MaxWidth)
		{
			return Text;
		}
		FString Cut = Text;
		while (Cut.Len() > 1 && Width(Cut + TEXT("..."), F) > MaxWidth)
		{
			Cut.LeftChopInline(1);
		}
		return Cut.TrimEnd() + TEXT("...");
	}

	// Text wrapped to Width, a line every LineHeight from the baseline at Y; returns the lines.
	int32 Wrap(float X, float Y, float MaxWidth, float LineHeight, const FString& Text, const FSlateFontInfo& F,
	           const FLinearColor& Colour, int32 MaxLines = 99) const
	{
		TArray<FString> Words;
		Text.ParseIntoArray(Words, TEXT(" "));
		FString Line;
		int32 Lines = 0;
		for (int32 I = 0; I < Words.Num(); ++I)
		{
			const FString Next = Line.IsEmpty() ? Words[I] : Line + TEXT(" ") + Words[I];
			if (!Line.IsEmpty() && Width(Next, F) > MaxWidth)
			{
				if (Lines == MaxLines - 1)
				{
					this->Text(X, Y + Lines * LineHeight, Fit(Next, F, MaxWidth), F, Colour);
					return Lines + 1;
				}
				this->Text(X, Y + Lines * LineHeight, Line, F, Colour);
				++Lines;
				Line = Words[I];
			}
			else
			{
				Line = Next;
			}
		}
		if (!Line.IsEmpty())
		{
			this->Text(X, Y + Lines * LineHeight, Line, F, Colour);
			++Lines;
		}
		return Lines;
	}

	void Lines(TArray<FVector2f> Points, const FLinearColor& Colour, float Thickness) const
	{
		if (Points.Num() < 2)
		{
			return;
		}
		for (FVector2f& P : Points)
		{
			P *= S;
		}
		FSlateDrawElement::MakeLines(Out, ++Layer, G.ToPaintGeometry(), MoveTemp(Points), ESlateDrawEffect::None,
		                             Colour, true, Thickness * S);
	}

	// A line in dashes On long with gaps Off between, along the polyline.
	void Dashed(const TArray<FVector2f>& Points, const FLinearColor& Colour, float Thickness, float On, float Off) const
	{
		float Phase = 0.f; // into the current dash-and-gap
		const float Period = On + Off;
		TArray<FVector2f> Dash;
		for (int32 I = 1; I < Points.Num(); ++I)
		{
			const FVector2f A = Points[I - 1], B = Points[I];
			const float Len = FVector2f::Distance(A, B);
			float Done = 0.f;
			while (Done < Len)
			{
				const bool bOn = Phase < On;
				const float Step = FMath::Min(Len - Done, (bOn ? On : Period) - Phase);
				const FVector2f From = A + (B - A) * (Done / Len), To = A + (B - A) * ((Done + Step) / Len);
				if (bOn)
				{
					if (Dash.IsEmpty())
					{
						Dash.Add(From);
					}
					Dash.Add(To);
				}
				Done += Step;
				Phase += Step;
				if (Phase >= Period - 1e-3f)
				{
					Phase = 0.f;
				}
				if (Phase >= On - 1e-3f && !Dash.IsEmpty())
				{
					Lines(MoveTemp(Dash), Colour, Thickness);
					Dash.Reset();
				}
			}
		}
		Lines(MoveTemp(Dash), Colour, Thickness);
	}

	// The area between a line and a baseline: a column of boxes a step wide, on one layer so they batch.
	void Area(const TArray<FVector2f>& Top, float Base, const FLinearColor& Colour, float Step = 2.f) const
	{
		if (Top.Num() < 2)
		{
			return;
		}
		const FSlateColorBrush Brush(FLinearColor::White);
		const int32 At0 = ++Layer;
		int32 Seg = 1;
		for (float X = Top[0].X; X < Top.Last().X; X += Step)
		{
			while (Seg < Top.Num() - 1 && Top[Seg].X < X)
			{
				++Seg;
			}
			const FVector2f A = Top[Seg - 1], B = Top[Seg];
			const float T = B.X > A.X ? FMath::Clamp((X + Step * 0.5f - A.X) / (B.X - A.X), 0.f, 1.f) : 0.f;
			const float Y = FMath::Lerp(A.Y, B.Y, T);
			const float Lo = FMath::Min(Y, Base), Hi = FMath::Max(Y, Base);
			if (Hi - Lo > 0.01f)
			{
				FSlateDrawElement::MakeBox(Out, At0, At(X, Lo, FMath::Min(Step, Top.Last().X - X) + 0.05f, Hi - Lo), &Brush,
				                           ESlateDrawEffect::None, Colour);
			}
		}
	}

	// An arc round Centre, from A to B degrees, clockwise on screen (y down).
	static void Arc(TArray<FVector2f>& Points, FVector2f Centre, float R, float A, float B)
	{
		const int32 Steps = FMath::Max(4, FMath::CeilToInt32(FMath::Abs(B - A) / 15.f));
		for (int32 I = 0; I <= Steps; ++I)
		{
			const float T = FMath::DegreesToRadians(FMath::Lerp(A, B, float(I) / Steps));
			Points.Add(Centre + FVector2f(FMath::Cos(T), FMath::Sin(T)) * R);
		}
	}

	// An SVG path's outline (M L H V A C Q Z, and their relative forms) at X, Y and Scale frame
	// units a path unit, as the stroked polylines it's made of: the design's icons.
	static TArray<TArray<FVector2f>> Path(const TCHAR* D, float X = 0.f, float Y = 0.f, float Scale = 1.f);

	// An icon drawn the way the canvas strokes it: a 24-unit path in a Size square.
	void Icon(const TCHAR* D, float X, float Y, float Size, const FLinearColor& Colour, float Stroke = 2.f) const
	{
		for (TArray<FVector2f>& Run : Path(D, X, Y, Size / 24.f))
		{
			Lines(MoveTemp(Run), Colour, Stroke * Size / 24.f);
		}
	}
};

// ---- SVG paths -----------------------------------------------------------------------------------

namespace Svg
{
struct FReader
{
	const TCHAR* P;
	void Skip()
	{
		while (*P == ' ' || *P == ',' || *P == '\n' || *P == '\t')
		{
			++P;
		}
	}
	bool Number() // a number comes next
	{
		Skip();
		return (*P >= '0' && *P <= '9') || *P == '-' || *P == '+' || *P == '.';
	}
	float Read()
	{
		Skip();
		const TCHAR* Start = P;
		if (*P == '-' || *P == '+')
		{
			++P;
		}
		bool bDot = false;
		while ((*P >= '0' && *P <= '9') || (*P == '.' && !bDot))
		{
			bDot = bDot || *P == '.';
			++P;
		}
		if (*P == 'e' || *P == 'E')
		{
			++P;
			if (*P == '-' || *P == '+')
			{
				++P;
			}
			while (*P >= '0' && *P <= '9')
			{
				++P;
			}
		}
		return FCString::Atof(*FString::ConstructFromPtrSize(Start, static_cast<int32>(P - Start)));
	}
	// An arc's flags are single digits that may run together: "a7 7 0 100 14".
	bool Flag()
	{
		Skip();
		const bool bOn = *P == '1';
		if (*P == '0' || *P == '1')
		{
			++P;
		}
		return bOn;
	}
};

// SVG's endpoint arc, as points after From (F.6.5 of the SVG spec).
inline void ArcTo(TArray<FVector2f>& Run, FVector2f From, float Rx, float Ry, float Rotation, bool bLarge, bool bSweep, FVector2f To)
{
	if (Rx == 0.f || Ry == 0.f || From.Equals(To, 1e-4f))
	{
		Run.Add(To);
		return;
	}
	Rx = FMath::Abs(Rx);
	Ry = FMath::Abs(Ry);
	const float Phi = FMath::DegreesToRadians(Rotation);
	const float C = FMath::Cos(Phi), Sn = FMath::Sin(Phi);
	const FVector2f Mid = (From - To) * 0.5f;
	const FVector2f P1(C * Mid.X + Sn * Mid.Y, -Sn * Mid.X + C * Mid.Y);
	const float Lambda = P1.X * P1.X / (Rx * Rx) + P1.Y * P1.Y / (Ry * Ry);
	if (Lambda > 1.f)
	{
		Rx *= FMath::Sqrt(Lambda);
		Ry *= FMath::Sqrt(Lambda);
	}
	const float Num = Rx * Rx * Ry * Ry - Rx * Rx * P1.Y * P1.Y - Ry * Ry * P1.X * P1.X;
	const float Den = Rx * Rx * P1.Y * P1.Y + Ry * Ry * P1.X * P1.X;
	float K = Den > 0.f ? FMath::Sqrt(FMath::Max(0.f, Num / Den)) : 0.f;
	K = bLarge == bSweep ? -K : K;
	const FVector2f Cp(K * Rx * P1.Y / Ry, -K * Ry * P1.X / Rx);
	const FVector2f Centre(C * Cp.X - Sn * Cp.Y + (From.X + To.X) * 0.5f, Sn * Cp.X + C * Cp.Y + (From.Y + To.Y) * 0.5f);
	auto Angle = [](FVector2f U, FVector2f V)
	{
		const float A = FMath::Atan2(U.X * V.Y - U.Y * V.X, U.X * V.X + U.Y * V.Y);
		return A;
	};
	const FVector2f U((P1.X - Cp.X) / Rx, (P1.Y - Cp.Y) / Ry), V((-P1.X - Cp.X) / Rx, (-P1.Y - Cp.Y) / Ry);
	const float Theta = Angle(FVector2f(1.f, 0.f), U);
	float Delta = Angle(U, V);
	if (!bSweep && Delta > 0.f)
	{
		Delta -= 2.f * PI;
	}
	else if (bSweep && Delta < 0.f)
	{
		Delta += 2.f * PI;
	}
	const int32 Steps = FMath::Max(4, FMath::CeilToInt32(FMath::Abs(Delta) / FMath::DegreesToRadians(12.f)));
	for (int32 I = 1; I <= Steps; ++I)
	{
		const float T = Theta + Delta * I / Steps;
		const float Ex = Rx * FMath::Cos(T), Ey = Ry * FMath::Sin(T);
		Run.Add(FVector2f(C * Ex - Sn * Ey + Centre.X, Sn * Ex + C * Ey + Centre.Y));
	}
}
} // namespace Svg

inline TArray<TArray<FVector2f>> FPainter::Path(const TCHAR* D, float X, float Y, float Scale)
{
	TArray<TArray<FVector2f>> Runs;
	TArray<FVector2f> Run;
	Svg::FReader R{D};
	FVector2f Pen(0.f, 0.f), Start(0.f, 0.f);
	TCHAR Command = 'M';
	auto Flush = [&]
	{
		if (Run.Num() > 1)
		{
			Runs.Add(MoveTemp(Run));
		}
		Run.Reset();
	};
	while (true)
	{
		R.Skip();
		if (*R.P == 0)
		{
			break;
		}
		if (!R.Number())
		{
			Command = *R.P++;
			if (Command == 'Z' || Command == 'z')
			{
				if (!Run.IsEmpty())
				{
					Run.Add(Start);
				}
				Flush();
				Pen = Start;
				continue;
			}
		}
		const bool bRel = FChar::IsLower(Command);
		const FVector2f Base = bRel ? Pen : FVector2f(0.f, 0.f);
		switch (FChar::ToUpper(Command))
		{
		case 'M':
		{
			Flush();
			const float Px = R.Read(), Py = R.Read();
			Pen = Base + FVector2f(Px, Py);
			Start = Pen;
			Run.Add(Pen);
			Command = bRel ? 'l' : 'L'; // more pairs after a move are lines
			break;
		}
		case 'L':
		{
			const float Px = R.Read(), Py = R.Read();
			Pen = Base + FVector2f(Px, Py);
			Run.Add(Pen);
			break;
		}
		case 'H':
			Pen.X = (bRel ? Pen.X : 0.f) + R.Read();
			Run.Add(Pen);
			break;
		case 'V':
			Pen.Y = (bRel ? Pen.Y : 0.f) + R.Read();
			Run.Add(Pen);
			break;
		case 'C':
		{
			FVector2f C[3];
			for (FVector2f& Each : C)
			{
				const float Px = R.Read(), Py = R.Read();
				Each = Base + FVector2f(Px, Py);
			}
			const FVector2f A = Pen;
			for (int32 I = 1; I <= 10; ++I)
			{
				const float T = I / 10.f, U = 1.f - T;
				Run.Add(A * (U * U * U) + C[0] * (3.f * U * U * T) + C[1] * (3.f * U * T * T) + C[2] * (T * T * T));
			}
			Pen = C[2];
			break;
		}
		case 'Q':
		{
			const float Ax = R.Read(), Ay = R.Read(), Bx = R.Read(), By = R.Read();
			const FVector2f Ctrl = Base + FVector2f(Ax, Ay), End = Base + FVector2f(Bx, By);
			const FVector2f A = Pen;
			for (int32 I = 1; I <= 8; ++I)
			{
				const float T = I / 8.f, U = 1.f - T;
				Run.Add(A * (U * U) + Ctrl * (2.f * U * T) + End * (T * T));
			}
			Pen = End;
			break;
		}
		case 'A':
		{
			const float Rx = R.Read(), Ry = R.Read(), Rot = R.Read();
			const bool bLarge = R.Flag(), bSweep = R.Flag();
			const float Px = R.Read(), Py = R.Read();
			const FVector2f To = Base + FVector2f(Px, Py);
			if (Run.IsEmpty())
			{
				Run.Add(Pen);
			}
			Svg::ArcTo(Run, Pen, Rx, Ry, Rot, bLarge, bSweep, To);
			Pen = To;
			break;
		}
		default:
			R.Read(); // a command this doesn't know: step over its numbers
			break;
		}
	}
	Flush();
	for (TArray<FVector2f>& Each : Runs)
	{
		for (FVector2f& P : Each)
		{
			P = FVector2f(X, Y) + P * Scale;
		}
	}
	return Runs;
}
} // namespace VeloversePaint
