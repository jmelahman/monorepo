// The rider's look: skin tone, hair and kit colours, picked in Settings > Kit and kept in the profile.
// Each part is a palette of linear colours. The first listed default is the export's own colour
// (rider/README.md's tint masks, stats.json "masks"), so an untouched rider looks as built.
// RidePawn sets them on the rider's tint materials (import_rider.py's M_RiderTint: C1..C6).
#pragma once

#include "CoreMinimal.h"

namespace Veloverse::Kit
{
enum EPart : int32
{
	Skin,
	Hair,
	Jersey,
	Accent,
	Shorts,
	Socks,
	Helmet,
	Shoes,
	Gloves,
	Parts,
};

struct FSwatch
{
	const TCHAR* Name;
	FLinearColor Colour; // linear
};

struct FPart
{
	const TCHAR* Key;  // the profile's "kit" field
	const TCHAR* Name;
	const TCHAR* Hint;
	TArrayView<const FSwatch> Swatches;
	int32 Default; // the export's own colour
};

inline const FSwatch SkinTones[] = {
	{TEXT("Fair"), {0.62f, 0.43f, 0.34f}},  {TEXT("Light"), {0.55f, 0.36f, 0.27f}},
	{TEXT("Medium"), {0.48f, 0.30f, 0.22f}}, {TEXT("Tan"), {0.36f, 0.21f, 0.13f}},
	{TEXT("Brown"), {0.22f, 0.12f, 0.07f}},  {TEXT("Deep"), {0.11f, 0.06f, 0.035f}},
	// Darker still, at the user's request (15 Nov). Added at the end so saved picks keep their tone.
	{TEXT("Umber"), {0.068f, 0.037f, 0.022f}}, {TEXT("Ebony"), {0.040f, 0.023f, 0.015f}},
};
inline const FSwatch Hairs[] = {
	{TEXT("Black"), {0.012f, 0.010f, 0.009f}}, {TEXT("Dark brown"), {0.030f, 0.019f, 0.012f}},
	{TEXT("Brown"), {0.09f, 0.05f, 0.025f}},   {TEXT("Auburn"), {0.18f, 0.06f, 0.025f}},
	{TEXT("Blond"), {0.40f, 0.28f, 0.14f}},    {TEXT("Grey"), {0.30f, 0.29f, 0.28f}},
};
inline const FSwatch Jerseys[] = {
	{TEXT("Navy"), {0.010f, 0.024f, 0.075f}}, {TEXT("Black"), {0.012f, 0.012f, 0.013f}},
	{TEXT("White"), {0.80f, 0.80f, 0.78f}},   {TEXT("Red"), {0.45f, 0.02f, 0.02f}},
	{TEXT("Green"), {0.01f, 0.10f, 0.04f}},   {TEXT("Sky"), {0.10f, 0.35f, 0.70f}},
	{TEXT("Yellow"), {0.85f, 0.62f, 0.02f}},  {TEXT("Purple"), {0.12f, 0.03f, 0.25f}},
};
inline const FSwatch Accents[] = {
	{TEXT("Orange"), {0.80f, 0.22f, 0.05f}}, {TEXT("White"), {0.80f, 0.80f, 0.78f}},
	{TEXT("Black"), {0.012f, 0.012f, 0.013f}}, {TEXT("Red"), {0.60f, 0.02f, 0.02f}},
	{TEXT("Yellow"), {0.85f, 0.62f, 0.02f}}, {TEXT("Teal"), {0.02f, 0.40f, 0.35f}},
	{TEXT("Pink"), {0.80f, 0.15f, 0.35f}},   {TEXT("Lime"), {0.35f, 0.75f, 0.05f}},
};
inline const FSwatch ShortsSwatches[] = {
	{TEXT("Black"), {0.012f, 0.012f, 0.013f}}, {TEXT("Navy"), {0.010f, 0.024f, 0.075f}},
	{TEXT("Grey"), {0.08f, 0.08f, 0.085f}},    {TEXT("White"), {0.80f, 0.80f, 0.78f}},
};
inline const FSwatch SockSwatches[] = {
	{TEXT("White"), {0.80f, 0.80f, 0.78f}}, {TEXT("Black"), {0.012f, 0.012f, 0.013f}},
	{TEXT("Grey"), {0.25f, 0.25f, 0.26f}},  {TEXT("Navy"), {0.010f, 0.024f, 0.075f}},
};
inline const FSwatch Helmets[] = {
	{TEXT("Navy"), {0.010f, 0.024f, 0.075f}}, {TEXT("Black"), {0.012f, 0.012f, 0.013f}},
	{TEXT("White"), {0.80f, 0.80f, 0.78f}},   {TEXT("Red"), {0.45f, 0.02f, 0.02f}},
	{TEXT("Yellow"), {0.85f, 0.62f, 0.02f}},  {TEXT("Sky"), {0.10f, 0.35f, 0.70f}},
};
inline const FSwatch ShoeSwatches[] = {
	{TEXT("White"), {0.80f, 0.80f, 0.78f}}, {TEXT("Black"), {0.012f, 0.012f, 0.013f}},
	{TEXT("Grey"), {0.25f, 0.25f, 0.26f}},  {TEXT("Red"), {0.45f, 0.02f, 0.02f}},
};
// Index 0 = Bare hands (RidePawn maps Bare to the rider's current Skin tone).
inline const FSwatch GloveSwatches[] = {
	{TEXT("Bare"), {0.52f, 0.36f, 0.28f}},     {TEXT("Charcoal"), {0.018f, 0.020f, 0.024f}},
	{TEXT("Midnight"), {0.010f, 0.024f, 0.075f}}, {TEXT("White"), {0.80f, 0.80f, 0.78f}},
	{TEXT("Crimson"), {0.45f, 0.02f, 0.02f}},  {TEXT("Forest"), {0.01f, 0.10f, 0.04f}},
	{TEXT("Amber"), {0.80f, 0.22f, 0.05f}},    {TEXT("Teal"), {0.02f, 0.40f, 0.35f}},
};

inline const FPart& Part(int32 I)
{
	static const FPart All[Parts] = {
		{TEXT("skin"), TEXT("Skin tone"), TEXT("Face, arms and legs"), SkinTones, 2},
		{TEXT("hair"), TEXT("Hair"), TEXT("Brows and hair"), Hairs, 1},
		{TEXT("jersey"), TEXT("Jersey"), TEXT("Body and sleeves"), Jerseys, 0},
		{TEXT("accent"), TEXT("Accent"), TEXT("Side panels and cuffs"), Accents, 0},
		{TEXT("shorts"), TEXT("Bib shorts"), TEXT("To above the knee"), ShortsSwatches, 0},
		{TEXT("socks"), TEXT("Socks"), TEXT("Mid-calf"), SockSwatches, 0},
		{TEXT("helmet"), TEXT("Helmet"), TEXT("Shell; rim stays black"), Helmets, 0},
		{TEXT("shoes"), TEXT("Shoes"), TEXT("Uppers; soles stay black"), ShoeSwatches, 0},
		{TEXT("gloves"), TEXT("Gloves"), TEXT("Fingerless road mitts"), GloveSwatches, 1},
	};
	return All[FMath::Clamp(I, 0, Parts - 1)];
}

// A profile's pick for a part: its swatch index, or -1 for the export's own colour.
inline const FSwatch& Pick(int32 PartIndex, int32 Chosen)
{
	const FPart& P = Part(PartIndex);
	return P.Swatches[P.Swatches.IsValidIndex(Chosen) ? Chosen : P.Default];
}
} // namespace Veloverse::Kit
