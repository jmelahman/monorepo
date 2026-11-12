// Structured workouts: a list of steps, each a steady or ramping target as a share of FTP, read
// from the built-in library or imported from the formats other apps use (.zwo, .erg, .mrc). A
// FWorkoutPlayer walks one as the ride goes and says what power the trainer should hold. Plain C++
// with no engine types, tested by tests/workout_test.cpp.
#pragma once

#include <algorithm>
#include <cctype>
#include <cmath>
#include <cstdlib>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

namespace Veloverse
{
struct FWorkoutStep
{
	double Seconds = 0.0;
	double Start = 0.0; // share of FTP at the step's start
	double End = 0.0;   // and at its end: the same for a steady step, else a ramp
	double Cadence = 0.0; // a target rpm, 0 for none
	bool bFree = false;   // free riding: no target, the rider rides the road
	std::string Label;    // "Warm up", "On 3/5", ...

	double At(double T) const
	{
		const double F = Seconds > 0.0 ? std::clamp(T / Seconds, 0.0, 1.0) : 0.0;
		return Start + (End - Start) * F;
	}
};

struct FWorkout
{
	std::string Name;
	std::string Description;
	std::vector<FWorkoutStep> Steps;

	double Seconds() const
	{
		double Sum = 0.0;
		for (const FWorkoutStep& S : Steps)
		{
			Sum += S.Seconds;
		}
		return Sum;
	}

	// The training load the workout would be ridden exactly: TSS from its normalized power,
	// sampled once a second as the ride log does. Free steps count as 50% of FTP.
	double Tss() const
	{
		std::vector<double> Shares;
		for (const FWorkoutStep& S : Steps)
		{
			for (double T = 0.5; T < S.Seconds; T += 1.0)
			{
				Shares.push_back(S.bFree ? 0.5 : S.At(T));
			}
		}
		if (Shares.empty())
		{
			return 0.0;
		}
		const std::size_t Window = std::min<std::size_t>(30, Shares.size());
		double Rolling = 0.0, Fourth = 0.0;
		std::size_t Count = 0;
		for (std::size_t I = 0; I < Shares.size(); ++I)
		{
			Rolling += Shares[I] - (I >= Window ? Shares[I - Window] : 0.0);
			if (I + 1 >= Window)
			{
				Fourth += std::pow(Rolling / Window, 4.0);
				++Count;
			}
		}
		const double If = std::pow(Fourth / Count, 0.25);
		return Shares.size() * If * If / 3600.0 * 100.0;
	}
};

// ---- Building ------------------------------------------------------------------------------

namespace WorkoutBuild
{
inline FWorkoutStep Steady(double Seconds, double Share, std::string Label = {}, double Cadence = 0.0)
{
	return {Seconds, Share, Share, Cadence, false, std::move(Label)};
}
inline FWorkoutStep Ramp(double Seconds, double From, double To, std::string Label = {})
{
	return {Seconds, From, To, 0.0, false, std::move(Label)};
}
inline FWorkoutStep Free(double Seconds, std::string Label = "Free ride")
{
	return {Seconds, 0.0, 0.0, 0.0, true, std::move(Label)};
}
// Repeats of on and off, labelled "On 1/5", "Off 1/5"...; the last rest is left out when
// bLastRest is false.
inline void Repeat(std::vector<FWorkoutStep>& Steps, int Count, double OnSeconds, double On, double OffSeconds,
                   double Off, double Cadence = 0.0, bool bLastRest = true)
{
	for (int I = 1; I <= Count; ++I)
	{
		const std::string Of = " " + std::to_string(I) + "/" + std::to_string(Count);
		Steps.push_back(Steady(OnSeconds, On, "On" + Of, Cadence));
		if (OffSeconds > 0.0 && (bLastRest || I < Count))
		{
			Steps.push_back(Steady(OffSeconds, Off, "Rest" + Of));
		}
	}
}
} // namespace WorkoutBuild

// The workouts that come with the game.
inline std::vector<FWorkout> BuiltInWorkouts()
{
	using namespace WorkoutBuild;
	std::vector<FWorkout> Out;
	auto Add = [&](std::string Name, std::string Description) -> std::vector<FWorkoutStep>&
	{
		Out.push_back({std::move(Name), std::move(Description), {}});
		return Out.back().Steps;
	};
	auto WarmUp = [](std::vector<FWorkoutStep>& S, double Minutes = 10.0)
	{ S.push_back(Ramp(Minutes * 60.0, 0.45, 0.75, "Warm up")); };
	auto CoolDown = [](std::vector<FWorkoutStep>& S, double Minutes = 5.0)
	{ S.push_back(Ramp(Minutes * 60.0, 0.65, 0.45, "Cool down")); };

	{
		auto& S = Add("Sweet spot 3x10", "Three 10-minute blocks just under threshold: the most fitness for the time.");
		WarmUp(S);
		Repeat(S, 3, 600, 0.90, 300, 0.55, 90, false);
		S.push_back(Steady(300, 0.55, "Easy"));
		CoolDown(S);
	}
	{
		auto& S = Add("VO2 max 5x3", "Five hard 3-minute efforts with equal rest, to lift the ceiling.");
		WarmUp(S);
		S.push_back(Steady(60, 1.05, "Opener"));
		S.push_back(Steady(180, 0.55, "Easy"));
		Repeat(S, 5, 180, 1.18, 180, 0.50, 100);
		CoolDown(S);
	}
	{
		auto& S = Add("Threshold 2x20", "Two 20-minute efforts at FTP. Hard, steady, and the classic test of form.");
		WarmUp(S, 12);
		Repeat(S, 2, 1200, 0.97, 300, 0.55, 90, false);
		S.push_back(Steady(300, 0.55, "Easy"));
		CoolDown(S);
	}
	{
		auto& S = Add("Over-unders", "Four 8-minute blocks swinging over and under threshold every 2 minutes.");
		WarmUp(S);
		for (int I = 1; I <= 4; ++I)
		{
			const std::string Of = " " + std::to_string(I) + "/4";
			for (int J = 0; J < 2; ++J)
			{
				S.push_back(Steady(120, 0.92, "Under" + Of));
				S.push_back(Steady(120, 1.07, "Over" + Of));
			}
			if (I < 4)
			{
				S.push_back(Steady(240, 0.55, "Rest" + Of));
			}
		}
		CoolDown(S);
	}
	{
		auto& S = Add("Endurance hour", "An hour in zone 2 with a few cadence drills. Easy enough to talk.");
		WarmUp(S, 8);
		for (int I = 0; I < 4; ++I)
		{
			S.push_back(Steady(600, 0.68, "Endurance"));
			S.push_back(Steady(60, 0.68, "Spin up", 105));
		}
		S.push_back(Steady(480, 0.65, "Endurance"));
		CoolDown(S);
	}
	{
		auto& S = Add("Sprints 6x15s", "Short all-out sprints with long rests, then a steady tempo block.");
		WarmUp(S);
		Repeat(S, 6, 15, 2.0, 165, 0.50, 110);
		S.push_back(Steady(600, 0.80, "Tempo"));
		CoolDown(S);
	}
	{
		auto& S = Add("Ramp test", "One-minute steps up until you can't hold the power. Your FTP is 75% of the best minute.");
		S.push_back(Steady(300, 0.40, "Warm up"));
		for (int I = 0; I < 30; ++I)
		{
			S.push_back(Steady(60, 0.40 + 0.06 * I, "Step " + std::to_string(I + 1)));
		}
		CoolDown(S);
	}
	{
		auto& S = Add("Recovery spin", "Thirty easy minutes to loosen the legs.");
		S.push_back(Steady(1800, 0.50, "Easy", 90));
	}
	return Out;
}

// ---- Importing -----------------------------------------------------------------------------

namespace WorkoutImport
{
inline std::string Lower(std::string S)
{
	std::transform(S.begin(), S.end(), S.begin(), [](unsigned char C) { return char(std::tolower(C)); });
	return S;
}

inline std::string Trim(const std::string& S)
{
	const auto B = S.find_first_not_of(" \t\r\n");
	const auto E = S.find_last_not_of(" \t\r\n");
	return B == std::string::npos ? std::string() : S.substr(B, E - B + 1);
}

// An XML element as the .zwo reader needs it: its name, its attributes, and whether it closes.
struct FTag
{
	std::string Name; // lower case; "/name" for a closing tag
	std::vector<std::pair<std::string, std::string>> Attributes;
	std::string Text; // the text after the tag, up to the next one

	std::optional<double> Number(const char* Key) const
	{
		for (const auto& [K, V] : Attributes)
		{
			if (K == Key)
			{
				char* End = nullptr;
				const double D = std::strtod(V.c_str(), &End);
				if (End != V.c_str())
				{
					return D;
				}
			}
		}
		return std::nullopt;
	}
	double Or(const char* Key, double Default) const { return Number(Key).value_or(Default); }
};

inline std::vector<FTag> Tags(const std::string& Xml)
{
	std::vector<FTag> Out;
	std::size_t I = 0;
	while ((I = Xml.find('<', I)) != std::string::npos)
	{
		if (Xml.compare(I, 4, "<!--") == 0)
		{
			const std::size_t E = Xml.find("-->", I);
			I = E == std::string::npos ? Xml.size() : E + 3;
			continue;
		}
		const std::size_t E = Xml.find('>', I);
		if (E == std::string::npos)
		{
			break;
		}
		const std::string Body = Xml.substr(I + 1, E - I - 1);
		I = E + 1;
		if (Body.empty() || Body[0] == '?' || Body[0] == '!')
		{
			continue;
		}
		FTag Tag;
		std::size_t P = 0;
		while (P < Body.size() && !std::isspace(static_cast<unsigned char>(Body[P])) && Body[P] != '/')
		{
			++P;
		}
		Tag.Name = Lower(Body.substr(0, P ? P : 1 + Body.find_first_of(" \t\r\n/", 1)));
		if (Body[0] == '/')
		{
			Tag.Name = "/" + Lower(Trim(Body.substr(1)));
		}
		// Attributes: key="value" or key='value'.
		while (P < Body.size())
		{
			while (P < Body.size() && (std::isspace(static_cast<unsigned char>(Body[P])) || Body[P] == '/'))
			{
				++P;
			}
			const std::size_t Eq = Body.find('=', P);
			if (Eq == std::string::npos)
			{
				break;
			}
			const std::string Key = Lower(Trim(Body.substr(P, Eq - P)));
			std::size_t Q = Eq + 1;
			while (Q < Body.size() && std::isspace(static_cast<unsigned char>(Body[Q])))
			{
				++Q;
			}
			if (Q >= Body.size() || (Body[Q] != '"' && Body[Q] != '\''))
			{
				break;
			}
			const std::size_t Close = Body.find(Body[Q], Q + 1);
			if (Close == std::string::npos)
			{
				break;
			}
			Tag.Attributes.push_back({Key, Body.substr(Q + 1, Close - Q - 1)});
			P = Close + 1;
		}
		const std::size_t Next = Xml.find('<', I);
		Tag.Text = Trim(Xml.substr(I, (Next == std::string::npos ? Xml.size() : Next) - I));
		Out.push_back(std::move(Tag));
	}
	return Out;
}

// Zwift's .zwo: <workout_file><name/><description/><workout> and its steps as elements.
inline std::optional<FWorkout> Zwo(const std::string& Xml)
{
	using namespace WorkoutBuild;
	FWorkout W;
	bool bInWorkout = false;
	for (const FTag& T : Tags(Xml))
	{
		const double Seconds = T.Or("duration", 0.0);
		const double Cadence = T.Or("cadence", 0.0);
		if (T.Name == "name" && W.Name.empty())
		{
			W.Name = T.Text;
		}
		else if (T.Name == "description" && W.Description.empty())
		{
			W.Description = T.Text;
		}
		else if (T.Name == "workout")
		{
			bInWorkout = true;
		}
		else if (T.Name == "/workout")
		{
			bInWorkout = false;
		}
		else if (!bInWorkout)
		{
			continue;
		}
		else if (T.Name == "warmup" || T.Name == "cooldown" || T.Name == "ramp")
		{
			const double Low = T.Or("powerlow", T.Or("power", 0.5)), High = T.Or("powerhigh", Low);
			const char* Label = T.Name == "warmup" ? "Warm up" : T.Name == "cooldown" ? "Cool down" : "Ramp";
			W.Steps.push_back({Seconds, Low, High, Cadence, false, Label});
		}
		else if (T.Name == "steadystate")
		{
			W.Steps.push_back(Steady(Seconds, T.Or("power", T.Or("powerlow", 0.5)), "Steady", Cadence));
		}
		else if (T.Name == "intervalst")
		{
			const int Count = std::max(1, static_cast<int>(T.Or("repeat", 1.0)));
			Repeat(W.Steps, Count, T.Or("onduration", 0.0), T.Or("onpower", 1.0), T.Or("offduration", 0.0),
			       T.Or("offpower", 0.5), T.Or("cadence", 0.0));
		}
		else if (T.Name == "freeride" || T.Name == "maxeffort")
		{
			W.Steps.push_back(Free(Seconds, T.Name == "maxeffort" ? "All out" : "Free ride"));
		}
	}
	W.Steps.erase(std::remove_if(W.Steps.begin(), W.Steps.end(), [](const FWorkoutStep& S) { return S.Seconds <= 0.0; }),
	              W.Steps.end());
	if (W.Steps.empty())
	{
		return std::nullopt;
	}
	return W;
}

// .erg and .mrc: a [COURSE HEADER] then [COURSE DATA] of "minutes value" points joined by straight
// lines, two points at the same minute making a jump. .erg values are watts (divided by the file's
// FTP, or RiderFtp without one); .mrc values are percent of FTP.
inline std::optional<FWorkout> ErgMrc(const std::string& Text, double RiderFtp)
{
	FWorkout W;
	std::istringstream In(Text);
	std::string Line;
	bool bData = false, bPercent = false;
	double FileFtp = 0.0;
	std::vector<std::pair<double, double>> Points; // seconds, share
	std::vector<std::pair<double, double>> Raw;
	while (std::getline(In, Line))
	{
		const std::string L = Trim(Line);
		const std::string Up = Lower(L);
		if (Up.empty() || Up[0] == ';' || Up[0] == '#')
		{
			continue;
		}
		if (Up.find("[course data]") == 0)
		{
			bData = true;
			continue;
		}
		if (Up.find("[end course data]") == 0)
		{
			bData = false;
			continue;
		}
		if (!bData)
		{
			const std::size_t Eq = L.find('=');
			const std::string Key = Lower(Trim(L.substr(0, Eq)));
			const std::string Value = Eq == std::string::npos ? std::string() : Trim(L.substr(Eq + 1));
			if (Key == "description" || Key == "file name")
			{
				(Key == "description" ? W.Description : W.Name) = Value;
			}
			else if (Key == "ftp")
			{
				FileFtp = std::atof(Value.c_str());
			}
			else if (Up.find("minutes") == 0 && Up.find("percent") != std::string::npos)
			{
				bPercent = true;
			}
			continue;
		}
		std::istringstream Pair(L);
		double Minutes = 0.0, Value = 0.0;
		if (Pair >> Minutes >> Value)
		{
			Raw.push_back({Minutes * 60.0, Value});
		}
	}
	const double Ftp = FileFtp > 0.0 ? FileFtp : std::max(RiderFtp, 1.0);
	for (const auto& [T, V] : Raw)
	{
		Points.push_back({T, bPercent ? V / 100.0 : V / Ftp});
	}
	for (std::size_t I = 1; I < Points.size(); ++I)
	{
		const double Seconds = Points[I].first - Points[I - 1].first;
		if (Seconds > 0.0)
		{
			W.Steps.push_back({Seconds, Points[I - 1].second, Points[I].second, 0.0, false, {}});
		}
	}
	if (W.Steps.empty())
	{
		return std::nullopt;
	}
	// Steps labelled by their shape; a steady run split over several points joins up.
	std::vector<FWorkoutStep> Joined;
	for (FWorkoutStep& S : W.Steps)
	{
		if (!Joined.empty() && Joined.back().Start == Joined.back().End && S.Start == S.End &&
		    std::abs(Joined.back().End - S.Start) < 1e-9)
		{
			Joined.back().Seconds += S.Seconds;
			continue;
		}
		Joined.push_back(S);
	}
	for (FWorkoutStep& S : Joined)
	{
		S.Label = std::abs(S.Start - S.End) < 1e-9 ? "Steady" : "Ramp";
	}
	W.Steps = std::move(Joined);
	return W;
}

// By the file's extension: .zwo, .erg or .mrc.
inline std::optional<FWorkout> Read(const std::string& FileName, const std::string& Text, double RiderFtp)
{
	const std::string Name = Lower(FileName);
	auto Ends = [&](const char* Ext) { return Name.size() >= 4 && Name.compare(Name.size() - 4, 4, Ext) == 0; };
	std::optional<FWorkout> W = Ends(".zwo")                      ? Zwo(Text)
	                            : Ends(".erg") || Ends(".mrc")    ? ErgMrc(Text, RiderFtp)
	                                                              : std::nullopt;
	if (W && W->Name.empty())
	{
		const std::size_t Slash = FileName.find_last_of("/\\");
		const std::string Base = FileName.substr(Slash == std::string::npos ? 0 : Slash + 1);
		W->Name = Base.substr(0, Base.size() - 4);
	}
	return W;
}
} // namespace WorkoutImport

// ---- Riding --------------------------------------------------------------------------------

// Where a ride is in its workout, by riding seconds.
class FWorkoutPlayer
{
public:
	FWorkoutPlayer() = default;
	explicit FWorkoutPlayer(FWorkout In) : W(std::move(In)) {}

	const FWorkout& Workout() const { return W; }
	bool Active() const { return !W.Steps.empty(); }
	bool Done() const { return Active() && Index >= W.Steps.size(); }
	std::size_t StepIndex() const { return Index; }
	const FWorkoutStep* Step() const { return Index < W.Steps.size() ? &W.Steps[Index] : nullptr; }
	const FWorkoutStep* NextStep() const { return Index + 1 < W.Steps.size() ? &W.Steps[Index + 1] : nullptr; }
	double InStep() const { return InStepSeconds; }
	double StepLeft() const { return Step() ? Step()->Seconds - InStepSeconds : 0.0; }
	double Elapsed() const { return Clock; }

	void Tick(double Dt)
	{
		if (!Active() || Done())
		{
			return;
		}
		Clock += Dt;
		InStepSeconds += Dt;
		while (Index < W.Steps.size() && InStepSeconds >= W.Steps[Index].Seconds)
		{
			InStepSeconds -= W.Steps[Index].Seconds;
			++Index;
		}
		if (Done())
		{
			InStepSeconds = 0.0;
		}
	}

	// On to the next step now.
	void Skip()
	{
		if (Index < W.Steps.size())
		{
			Clock += StepLeft();
			++Index;
			InStepSeconds = 0.0;
		}
	}

	// The watts to hold now, with the rider's own adjustment (Bias, a share: 0.05 is 5% harder);
	// none on a free step or after the end.
	std::optional<double> Target(double Ftp, double Bias = 0.0) const
	{
		const FWorkoutStep* S = Step();
		if (!S || S->bFree)
		{
			return std::nullopt;
		}
		return std::max(0.0, S->At(InStepSeconds) * (1.0 + Bias) * Ftp);
	}

private:
	FWorkout W;
	std::size_t Index = 0;
	double InStepSeconds = 0.0;
	double Clock = 0.0;
};
} // namespace Veloverse
