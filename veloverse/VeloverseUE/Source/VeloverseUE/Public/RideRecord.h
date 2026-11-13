// The rider's profile and their rides, kept between sessions as JSON in the game's Saved folder:
// ~/.config/Veloverse/profile.json, and a file per ride in ~/.config/Veloverse/Rides. The numbers are
// RideStats.h's; this is where they're sampled, stored and read back.
#pragma once

#include "CoreMinimal.h"
#include "RideReview.h"
#include "RideStats.h"

#include <vector>

struct FRideProfile
{
	float Ftp = 250.f;     // W
	float WeightKg = 80.f; // the rider
	float BikeKg = 10.f;
	int64 Xp = 0;                // points from every ride, toward levels (Progress.h)
	FString Theme = TEXT("Ember"); // the HUD colour, one of the unlocked Progress.h themes
	int32 MaxHeartRate = 190;      // bpm, for heart-rate zones (RideReview.h)

	// The theme's colour, or Ember's when the theme isn't known or isn't unlocked.
	FLinearColor Accent() const;

	static FRideProfile Load();
	static bool Saved(); // false until the rider's first profile is written: a first run
	void Save() const;
	static FString Folder(); // ~/.config/Veloverse
};

// A climb timed on a ride, as the ride's file keeps it.
struct FRideClimb
{
	FString Name;
	double Seconds = 0.0, Watts = 0.0;
	int32 Rank = 0, Of = 0; // among every try, this one included
	bool bPr = false;
};

// One saved ride as the history lists it: its summary without the samples.
struct FRideEntry
{
	FString File;
	FDateTime Started;
	FString Route;
	float Ftp = 0.f;
	int32 MaxHeartRate = 190;
	int32 Xp = 0;
	Veloverse::FSummary Summary;
	Veloverse::FCurve Curve = {}; // over RideReview.h's CurveDurations; rides before it fill what Best has
	TArray<FRideClimb> Climbs;
};

// Samples a ride once a second of riding and writes it out when it ends.
class FRideRecorder
{
public:
	// Called every frame the rider moves on, with that frame's riding time.
	// East and South are metres from the world origin, for the map.
	void Tick(float Dt, double Metres, double Watts, double Cadence, double HeartRate, double Speed, double Elevation,
	          double East, double South);
	void Reset();

	const std::vector<Veloverse::FSample>& Samples() const { return Recorded; }
	Veloverse::FSummary Summary(float Ftp) const { return Veloverse::Summarize(Recorded, Ftp); }

	// Writes the ride to ~/.config/Veloverse/Rides/<when>.json, and beside it <when>.fit for Strava and
	// the like; returns the .json, or "" if it was too short to keep (under MinSeconds) or the write
	// failed.
	// Climbs and Xp are what the ride earned, kept for its review.
	FString Save(const FString& Route, float StartMetres, const FRideProfile& Profile,
	             const TArray<FRideClimb>& Climbs = {}, int32 Xp = 0) const;
	static constexpr double MinSeconds = 30.0;

	// The saved rides, newest first.
	static TArray<FRideEntry> History();
	// One saved ride's samples, a second apart; empty if it can't be read.
	static std::vector<Veloverse::FSample> LoadSamples(const FString& File);
	static FString RidesFolder(); // ~/.config/Veloverse/Rides

private:
	std::vector<Veloverse::FSample> Recorded;
	FDateTime Started;
	double Clock = 0.0; // riding seconds
	// Sums over the current second, averaged into its sample.
	double Watts = 0.0, Cadence = 0.0, Heart = 0.0, Span = 0.0;
};
