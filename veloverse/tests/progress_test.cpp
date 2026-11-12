// Checks Progress.h without the engine. Run with tests/run.sh.
#include "Progress.h"

#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got = 0.0)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

int main()
{
	// 30 km with 400 m up, two segments (one a PR) and a workout done.
	const FRideXp Xp = RideXp(30000.0, 400.0, 2, 1, true);
	Check(Xp.Distance == 600, "20 XP a km", Xp.Distance);
	Check(Xp.Climbing == 80, "1 XP a 5 m climbed", Xp.Climbing);
	Check(Xp.Segments == 100, "25 a segment and 50 a PR", Xp.Segments);
	Check(Xp.Total() == 880, "adding up", Xp.Total());
	Check(RideXp(-5.0, -1.0, 0, 0, false).Total() == 0, "nothing for nothing");

	Check(LevelFor(0).Level == 1 && LevelFor(0).Needed == 500, "a new rider is level 1 of 500 XP");
	Check(LevelFor(499).Level == 1 && LevelFor(500).Level == 2, "level 2 at 500", LevelFor(500).Level);
	const FLevel Three = LevelFor(500 + 600 + 350);
	Check(Three.Level == 3 && Three.Into == 350 && Three.Needed == 700, "350 into level 3's 700", Three.Into);
	Check(LevelFor(100000000).Level == MaxLevel && LevelFor(100000000).Fraction() == 1.0, "the top level is full");

	const std::vector<FTheme> New = Unlocked(400, 500 + 600 + 50);
	Check(New.size() == 1 && std::string(New[0].Name) == "Surf", "level 3 unlocks Surf",
	      static_cast<double>(New.size()));
	Check(Unlocked(0, 100).empty(), "no new level, no unlock");
	Check(IsUnlocked(Themes()[0], 0), "Ember from the start");
	bool Ordered = true;
	for (std::size_t I = 1; I < Themes().size(); ++I)
	{
		Ordered = Ordered && Themes()[I].Level > Themes()[I - 1].Level && Themes()[I].Level <= MaxLevel;
	}
	Check(Ordered, "themes in level order, all reachable");

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
