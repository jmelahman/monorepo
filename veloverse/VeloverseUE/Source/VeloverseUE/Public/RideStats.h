// A ride's numbers from its once-a-second samples: averages, normalized power, the best efforts
// over set durations, time in each power zone, and the training load. Plain C++ with no engine
// types, so it can be tested on its own (tests/ride_stats_test.cpp).
#pragma once

#include <algorithm>
#include <array>
#include <cmath>
#include <cstddef>
#include <vector>

namespace Veloverse
{
// One second of riding.
struct FSample
{
	double Seconds = 0.0;   // riding time at this sample (pauses don't count)
	double Metres = 0.0;    // ridden since the start
	double Watts = 0.0;
	double Cadence = 0.0;   // rpm, 0 when coasting
	double HeartRate = 0.0; // bpm, 0 without a strap
	double Speed = 0.0;     // m/s
	double Elevation = 0.0; // m
	double East = 0.0;      // m from the world's origin, for a map (FitFile.h's Geo)
	double South = 0.0;
};

// Coggan's seven power zones as shares of FTP: the top of zones 1 to 6 (zone 7 has no top).
constexpr std::array<double, 6> ZoneTops = {0.55, 0.75, 0.90, 1.05, 1.20, 1.50};

inline int PowerZone(double Watts, double Ftp)
{
	int Z = 0;
	while (Z < 6 && Watts >= ZoneTops[Z] * Ftp)
	{
		++Z;
	}
	return Z;
}

// The best efforts the summary lists, in seconds.
constexpr std::array<int, 5> BestDurations = {5, 60, 300, 1200, 3600};

struct FSummary
{
	double Seconds = 0.0;
	double Metres = 0.0;
	double Climbed = 0.0; // m of ascent
	double AverageWatts = 0.0;
	double NormalizedWatts = 0.0;
	double MaxWatts = 0.0;
	double Kilojoules = 0.0;
	double AverageCadence = 0.0; // while pedalling
	double AverageHeartRate = 0.0;
	double MaxHeartRate = 0.0;
	double AverageSpeed = 0.0; // m/s
	double Intensity = 0.0;    // NP over FTP
	double Tss = 0.0;          // training stress: an hour at FTP is 100
	std::array<double, 7> ZoneSeconds = {};
	// The best average power over each of BestDurations, 0 where the ride was shorter.
	std::array<double, BestDurations.size()> Best = {};
};

// The best average power over Window consecutive samples, 0 if there are fewer.
inline double BestAverage(const std::vector<FSample>& Samples, std::size_t Window)
{
	if (Window == 0 || Samples.size() < Window)
	{
		return 0.0;
	}
	double Sum = 0.0;
	for (std::size_t I = 0; I < Window; ++I)
	{
		Sum += Samples[I].Watts;
	}
	double Best = Sum;
	for (std::size_t I = Window; I < Samples.size(); ++I)
	{
		Sum += Samples[I].Watts - Samples[I - Window].Watts;
		Best = std::max(Best, Sum);
	}
	return Best / static_cast<double>(Window);
}

// Normalized power: the 30-second rolling average, raised to the fourth power, averaged, and the
// fourth root taken. Rides under 30 s get their plain average.
inline double NormalizedPower(const std::vector<FSample>& Samples)
{
	constexpr std::size_t Window = 30;
	if (Samples.size() < Window)
	{
		double Sum = 0.0;
		for (const FSample& S : Samples)
		{
			Sum += S.Watts;
		}
		return Samples.empty() ? 0.0 : Sum / static_cast<double>(Samples.size());
	}
	double Rolling = 0.0, Fourth = 0.0;
	std::size_t Count = 0;
	for (std::size_t I = 0; I < Samples.size(); ++I)
	{
		Rolling += Samples[I].Watts;
		if (I >= Window)
		{
			Rolling -= Samples[I - Window].Watts;
		}
		if (I + 1 >= Window)
		{
			Fourth += std::pow(Rolling / Window, 4.0);
			++Count;
		}
	}
	return std::pow(Fourth / static_cast<double>(Count), 0.25);
}

// The summary of a ride sampled once a second against an FTP in watts.
inline FSummary Summarize(const std::vector<FSample>& Samples, double Ftp)
{
	FSummary Out;
	if (Samples.empty())
	{
		return Out;
	}
	Ftp = std::max(Ftp, 1.0);
	double Watts = 0.0, Cadence = 0.0, Heart = 0.0;
	int Pedalling = 0, Beating = 0;
	for (std::size_t I = 0; I < Samples.size(); ++I)
	{
		const FSample& S = Samples[I];
		Watts += S.Watts;
		Out.MaxWatts = std::max(Out.MaxWatts, S.Watts);
		if (S.Cadence > 0.0)
		{
			Cadence += S.Cadence;
			++Pedalling;
		}
		if (S.HeartRate > 0.0)
		{
			Heart += S.HeartRate;
			++Beating;
			Out.MaxHeartRate = std::max(Out.MaxHeartRate, S.HeartRate);
		}
		Out.ZoneSeconds[PowerZone(S.Watts, Ftp)] += 1.0;
		if (I > 0)
		{
			Out.Climbed += std::max(0.0, S.Elevation - Samples[I - 1].Elevation);
		}
	}
	const double N = static_cast<double>(Samples.size());
	Out.Seconds = Samples.back().Seconds;
	Out.Metres = Samples.back().Metres;
	Out.AverageWatts = Watts / N;
	Out.Kilojoules = Watts / 1000.0; // a sample a second: watts summed are joules
	Out.AverageCadence = Pedalling ? Cadence / Pedalling : 0.0;
	Out.AverageHeartRate = Beating ? Heart / Beating : 0.0;
	Out.AverageSpeed = Out.Seconds > 0.0 ? Out.Metres / Out.Seconds : 0.0;
	Out.NormalizedWatts = NormalizedPower(Samples);
	Out.Intensity = Out.NormalizedWatts / Ftp;
	Out.Tss = N * Out.NormalizedWatts * Out.Intensity / (Ftp * 3600.0) * 100.0;
	for (std::size_t I = 0; I < BestDurations.size(); ++I)
	{
		Out.Best[I] = BestAverage(Samples, static_cast<std::size_t>(BestDurations[I]));
	}
	return Out;
}
} // namespace Veloverse
