#include "RideChart.h"

#include "Blueprint/WidgetTree.h"
#include "Components/CanvasPanel.h"
#include "Rendering/DrawElements.h"
#include "Styling/CoreStyle.h"

namespace
{
FLinearColor Hex(uint32 Rgb, float Alpha = 1.f)
{
	FLinearColor C = FLinearColor::FromSRGBColor(FColor((Rgb >> 16) & 255, (Rgb >> 8) & 255, Rgb & 255));
	C.A = Alpha;
	return C;
}
// The HUD's zone colours.
const uint32 ZoneHex[7] = {0xA7B0BA, 0x4FA3F7, 0x3CCB7F, 0xF7D046, 0xFF9A3C, 0xF0503C, 0xB86CF5};
} // namespace

TSharedRef<SWidget> URideChart::RebuildWidget()
{
	if (WidgetTree && !WidgetTree->RootWidget)
	{
		WidgetTree->RootWidget = WidgetTree->ConstructWidget<UCanvasPanel>();
	}
	SetVisibility(ESlateVisibility::HitTestInvisible);
	return Super::RebuildWidget();
}

void URideChart::SetRide(const std::vector<Veloverse::FSample>& Samples, float InFtp)
{
	Ftp = FMath::Max(InFtp, 1.f);
	Power.Reset();
	Elevation.Reset();
	FMemory::Memzero(Zones);
	const int32 N = static_cast<int32>(Samples.size());
	if (N == 0)
	{
		return;
	}
	for (const Veloverse::FSample& S : Samples)
	{
		Zones[Veloverse::PowerZone(S.Watts, Ftp)] += 1.f;
	}
	// Each column the 30 s average round its middle, so a long ride's chart reads as efforts, not
	// noise; a short one has fewer columns than seconds would allow.
	const int32 Count = FMath::Min(Columns, N);
	const int32 Half = 15;
	for (int32 C = 0; C < Count; ++C)
	{
		const int32 Mid = (C * N + N / 2) / Count;
		const int32 From = FMath::Max(0, Mid - Half), To = FMath::Min(N, Mid + Half + 1);
		double Sum = 0.0;
		for (int32 I = From; I < To; ++I)
		{
			Sum += Samples[I].Watts;
		}
		Power.Add(static_cast<float>(Sum / (To - From)));
		Elevation.Add(static_cast<float>(Samples[FMath::Min(Mid, N - 1)].Elevation));
	}
	Invalidate(EInvalidateWidgetReason::Paint);
}

int32 URideChart::NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
                              FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
                              bool bParentEnabled) const
{
	Layer = Super::NativePaint(Args, Geometry, Culling, Out, Layer, Style, bParentEnabled);
	const FVector2f Size = Geometry.GetLocalSize();
	const float W = Size.X, StripH = 10.f, Gap = 8.f, H = Size.Y - StripH - Gap;
	const FSlateColorBrush White(FLinearColor::White);
	auto Box = [&](float X, float Y, float BW, float BH, const FLinearColor& Colour)
	{
		FSlateDrawElement::MakeBox(Out, ++Layer,
		                           Geometry.ToPaintGeometry(FVector2f(BW, BH), FSlateLayoutTransform(FVector2f(X, Y))),
		                           &White, ESlateDrawEffect::None, Colour);
	};
	Box(0.f, 0.f, W, H, FLinearColor(1.f, 1.f, 1.f, 0.04f));
	const int32 Count = Power.Num();
	if (Count < 2)
	{
		return Layer;
	}
	// The road's height, as a faint area under everything.
	float Low = Elevation[0], High = Elevation[0];
	for (const float E : Elevation)
	{
		Low = FMath::Min(Low, E);
		High = FMath::Max(High, E);
	}
	const float Span = FMath::Max(High - Low, 20.f); // a flat ride stays low, not a full-height block
	const float Step = W / Count;
	for (int32 C = 0; C < Count; ++C)
	{
		const float Tall = 6.f + (Elevation[C] - Low) / Span * H * 0.45f;
		Box(C * Step, H - Tall, Step + 0.5f, Tall, FLinearColor(1.f, 1.f, 1.f, 0.07f));
	}
	// FTP as a dashed line, and power up to 1.6 FTP or the ride's peak.
	float Top = Ftp * 1.6f;
	for (const float P : Power)
	{
		Top = FMath::Max(Top, P * 1.05f);
	}
	const float FtpY = H - Ftp / Top * H;
	for (float X = 0.f; X < W; X += 12.f)
	{
		Box(X, FtpY, 6.f, 1.f, FLinearColor(1.f, 1.f, 1.f, 0.25f));
	}
	TArray<FVector2f> Points;
	TArray<FLinearColor> Colours;
	for (int32 C = 0; C < Count; ++C)
	{
		Points.Add(FVector2f((C + 0.5f) * Step, H - FMath::Min(Power[C], Top) / Top * H));
		Colours.Add(Hex(ZoneHex[Veloverse::PowerZone(Power[C], Ftp)]));
	}
	FSlateDrawElement::MakeLines(Out, ++Layer, Geometry.ToPaintGeometry(), Points, Colours, ESlateDrawEffect::None,
	                             FLinearColor::White, true, 2.f);
	FSlateDrawElement::MakeText(Out, ++Layer,
	                            Geometry.ToPaintGeometry(FVector2f(80.f, 14.f), FSlateLayoutTransform(FVector2f(4.f, FtpY - 15.f))),
	                            FString::Printf(TEXT("FTP %.0f W"), Ftp), FCoreStyle::GetDefaultFontStyle("Regular", 8),
	                            ESlateDrawEffect::None, FLinearColor(1.f, 1.f, 1.f, 0.5f));
	// The zones strip: each zone's share of the ride, in order.
	float Total = 0.f;
	for (const float Z : Zones)
	{
		Total += Z;
	}
	float X = 0.f;
	for (int32 Z = 0; Z < 7 && Total > 0.f; ++Z)
	{
		const float Wide = Zones[Z] / Total * W;
		Box(X, H + Gap, Wide, StripH, Hex(ZoneHex[Z]));
		X += Wide;
	}
	return Layer;
}
