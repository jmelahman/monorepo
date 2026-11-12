// Points and levels: a ride earns XP for its distance and climbing, its segments and PRs, and a
// finished workout; XP adds up to levels, and levels unlock HUD colours. Plain C++, tested in
// tests/progress_test.cpp.
#pragma once

#include <algorithm>
#include <cmath>
#include <string>
#include <vector>

namespace Veloverse
{
// What one ride earned, by where it came from.
struct FRideXp
{
	int Distance = 0; // 20 a km
	int Climbing = 0; // 1 a 5 m climbed
	int Segments = 0; // 25 a segment ridden, 50 more for a PR
	int Workout = 0;  // 100 for a workout ridden to the end
	int Total() const { return Distance + Climbing + Segments + Workout; }
};

inline FRideXp RideXp(double Metres, double Climbed, int SegmentsRidden, int Prs, bool bWorkoutDone)
{
	FRideXp Xp;
	Xp.Distance = static_cast<int>(std::floor(std::max(Metres, 0.0) / 1000.0 * 20.0));
	Xp.Climbing = static_cast<int>(std::floor(std::max(Climbed, 0.0) / 5.0));
	Xp.Segments = 25 * std::max(SegmentsRidden, 0) + 50 * std::max(Prs, 0);
	Xp.Workout = bWorkoutDone ? 100 : 0;
	return Xp;
}

constexpr int MaxLevel = 50;

// XP to go from Level to the next: 500 at first, 100 more each level.
inline int XpToNext(int Level) { return 500 + 100 * (std::clamp(Level, 1, MaxLevel) - 1); }

struct FLevel
{
	int Level = 1;
	int Into = 0;   // XP into this level
	int Needed = 0; // XP this level takes (0 at the top)
	double Fraction() const { return Needed > 0 ? static_cast<double>(Into) / Needed : 1.0; }
};

inline FLevel LevelFor(long long Xp)
{
	FLevel L;
	long long Left = std::max(Xp, 0LL);
	while (L.Level < MaxLevel && Left >= XpToNext(L.Level))
	{
		Left -= XpToNext(L.Level);
		++L.Level;
	}
	L.Into = static_cast<int>(L.Level < MaxLevel ? Left : 0);
	L.Needed = L.Level < MaxLevel ? XpToNext(L.Level) : 0;
	return L;
}

// ---- Unlocks -------------------------------------------------------------------------------------

// A HUD colour, unlocked at a level: the accent the HUD and menus draw in.
struct FTheme
{
	const char* Name;
	int Level;
	unsigned Rgb; // 0xRRGGBB, sRGB
};

inline const std::vector<FTheme>& Themes()
{
	static const std::vector<FTheme> All = {
		{"Ember", 1, 0xFFAD50},  {"Surf", 3, 0x2EC4D6},  {"Moss", 5, 0x7BC043},  {"Lilac", 8, 0xB28DFF},
		{"Coral", 12, 0xFF6F7D}, {"Gold", 16, 0xF2C14E}, {"Ice", 20, 0xBFE6FF}, {"Night", 25, 0x5468FF},
		{"Polka", 30, 0xE8262B}, {"Chalk", 40, 0xF4F1EA},
	};
	return All;
}

// The themes a ride took the rider to, from Before XP to After.
inline std::vector<FTheme> Unlocked(long long Before, long long After)
{
	const int From = LevelFor(Before).Level, To = LevelFor(After).Level;
	std::vector<FTheme> Out;
	for (const FTheme& T : Themes())
	{
		if (T.Level > From && T.Level <= To)
		{
			Out.push_back(T);
		}
	}
	return Out;
}

inline bool IsUnlocked(const FTheme& T, long long Xp) { return LevelFor(Xp).Level >= T.Level; }
} // namespace Veloverse
