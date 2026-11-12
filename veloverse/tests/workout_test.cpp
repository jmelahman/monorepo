// Checks Workout.h without the engine. Run with tests/run.sh.
#include "Workout.h"

#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

static bool Near(double A, double B, double Tol = 1e-6) { return std::abs(A - B) <= Tol; }

int main()
{
	// The library: every workout has steps, and an hour at FTP would be 100 TSS.
	const std::vector<FWorkout> Library = BuiltInWorkouts();
	Check(Library.size() >= 6, "the library has workouts", double(Library.size()));
	for (const FWorkout& W : Library)
	{
		Check(!W.Steps.empty() && W.Seconds() > 600.0 && !W.Name.empty(), W.Name.c_str(), W.Seconds() / 60.0);
	}
	FWorkout Hour{"Hour", "", {WorkoutBuild::Steady(3600, 1.0)}};
	Check(Near(Hour.Tss(), 100.0, 1e-6), "an hour at FTP is 100 TSS", Hour.Tss());
	const FWorkout& Sweet = Library[0];
	Check(Sweet.Steps[1].Label == "On 1/3" && Sweet.Steps[2].Label == "Rest 1/3", "repeats are labelled", 0.0);
	Check(Sweet.Tss() > 40.0 && Sweet.Tss() < 80.0, "sweet spot 3x10 is a middling load", Sweet.Tss());

	// The player: targets, ramps, step changes, skipping and the end.
	FWorkout Two{"Two", "", {WorkoutBuild::Ramp(100, 0.5, 1.0), WorkoutBuild::Steady(60, 1.2), WorkoutBuild::Free(30)}};
	FWorkoutPlayer P(Two);
	Check(Near(*P.Target(200.0), 100.0), "a ramp starts at its start", *P.Target(200.0));
	P.Tick(50.0);
	Check(Near(*P.Target(200.0), 150.0), "half way up the ramp", *P.Target(200.0));
	Check(Near(*P.Target(200.0, 0.1), 165.0), "the rider's bias makes it 10% harder", *P.Target(200.0, 0.1));
	P.Tick(60.0);
	Check(P.StepIndex() == 1 && Near(P.InStep(), 10.0), "on into the next step", P.InStep());
	Check(Near(*P.Target(200.0), 240.0), "the steady step's target", *P.Target(200.0));
	P.Skip();
	Check(P.StepIndex() == 2 && !P.Target(200.0), "a free step has no target", P.Elapsed());
	P.Tick(31.0);
	Check(P.Done() && !P.Target(200.0), "done after the last step", P.Elapsed());

	// .zwo
	const std::string Zwo = R"(<?xml version="1.0"?>
<workout_file>
  <author>Someone</author>
  <name>Test &amp; more</name>
  <description>Short one</description>
  <sportType>bike</sportType>
  <workout>
    <Warmup Duration="300" PowerLow="0.40" PowerHigh="0.70"/>
    <!-- a comment <SteadyState Duration="999" Power="9"/> -->
    <SteadyState Duration="600" Power="0.88" Cadence="90">
      <textevent timeoffset="10" message="Settle in"/>
    </SteadyState>
    <IntervalsT Repeat="3" OnDuration="60" OffDuration="120" OnPower="1.20" OffPower="0.50"/>
    <FreeRide Duration="120"/>
    <Cooldown Duration='300' PowerLow='0.65' PowerHigh='0.40'/>
  </workout>
</workout_file>)";
	const auto Z = WorkoutImport::Read("test.zwo", Zwo, 250.0);
	Check(Z.has_value(), ".zwo reads", 0.0);
	if (Z)
	{
		Check(Z->Name == "Test &amp; more" && Z->Description == "Short one", ".zwo name and description", 0.0);
		Check(Z->Steps.size() == 10, ".zwo: warm up, steady, 3 x on and off, free, cool down", double(Z->Steps.size()));
		Check(Near(Z->Steps[0].Start, 0.40) && Near(Z->Steps[0].End, 0.70), ".zwo warm up ramps", Z->Steps[0].End);
		Check(Near(Z->Steps[1].Start, 0.88) && Near(Z->Steps[1].Cadence, 90.0), ".zwo steady with cadence", 0.88);
		Check(Near(Z->Steps[2].Start, 1.20) && Near(Z->Steps[2].Seconds, 60.0), ".zwo on step", Z->Steps[2].Start);
		Check(Z->Steps[8].bFree, ".zwo free ride", 0.0);
		Check(Near(Z->Seconds(), 300 + 600 + 540 + 120 + 300), ".zwo length", Z->Seconds());
	}

	// .erg in watts against the file's FTP, and .mrc in percent.
	const std::string Erg = "[COURSE HEADER]\nVERSION = 2\nUNITS = ENGLISH\nDESCRIPTION = Erg test\nFILE NAME = ergy\n"
	                        "FTP = 200\nMINUTES WATTS\n[END COURSE HEADER]\n[COURSE DATA]\n"
	                        "0.00\t100\n5.00\t200\n5.00\t300\n7.00\t300\n9.00\t300\n9.00\t100\n10.00\t100\n[END COURSE DATA]\n";
	const auto E = WorkoutImport::Read("x.erg", Erg, 250.0);
	Check(E.has_value(), ".erg reads", 0.0);
	if (E)
	{
		Check(E->Name == "ergy" && E->Description == "Erg test", ".erg header", 0.0);
		Check(E->Steps.size() == 3, ".erg: a ramp, a joined steady, the easy minute", double(E->Steps.size()));
		Check(Near(E->Steps[0].Start, 0.5) && Near(E->Steps[0].End, 1.0), ".erg watts over the file's FTP", E->Steps[0].End);
		Check(Near(E->Steps[1].Seconds, 240.0) && Near(E->Steps[1].Start, 1.5), ".erg steady points join", E->Steps[1].Seconds);
	}
	const std::string Mrc = "[COURSE HEADER]\nMINUTES PERCENT\n[END COURSE HEADER]\n[COURSE DATA]\n0 50\n10 50\n10 95\n30 95\n[END COURSE DATA]\n";
	const auto M = WorkoutImport::Read("dir/Tempo Thing.mrc", Mrc, 250.0);
	Check(M && M->Steps.size() == 2 && Near(M->Steps[1].Start, 0.95), ".mrc percent", M ? M->Steps[1].Start : 0.0);
	Check(M && M->Name == "Tempo Thing", ".mrc named after its file", 0.0);
	const auto Bare = WorkoutImport::Read("w.erg", "[COURSE DATA]\n0 125\n10 125\n[END COURSE DATA]\n", 250.0);
	Check(Bare && Near(Bare->Steps[0].Start, 0.5), ".erg without an FTP uses the rider's", Bare ? Bare->Steps[0].Start : 0.0);
	Check(!WorkoutImport::Read("w.txt", Erg, 250.0) && !WorkoutImport::Read("w.zwo", "<x/>", 250.0),
	      "other files and empty workouts don't read", 0.0);

	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
