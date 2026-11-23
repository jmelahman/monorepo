// The rider's profile and their rides, kept between sessions as JSON in the game's Saved folder:
// ~/.config/Veloverse/profile.json, and a file per ride in ~/.config/Veloverse/Rides. The numbers are
// RideStats.h's; this is where they're sampled, stored and read back.
#pragma once

#include "CoreMinimal.h"
#include "RideReview.h"
#include "RideStats.h"
#include "RiderKit.h"

#include <vector>

struct FRideProfile
{
	float Ftp = 250.f;     // W
	float WeightKg = 80.f; // the rider
	float BikeKg = 10.f;
	int64 Xp = 0;                // points from every ride, toward levels (Progress.h)
	FString Theme = TEXT("Ember"); // the HUD colour, one of the unlocked Progress.h themes
	int32 MaxHeartRate = 190;      // bpm, for heart-rate zones (RideReview.h)

	// The settings that ride with the rider (the screen's own are in GameUserSettings.ini).
	bool bImperial = false;    // miles, mph and feet (Units.h)
	float Fov = 61.f;          // the view's width, degrees: a 20 mm lens
	float Difficulty = 100.f;  // the share of the road's gradient the trainer gives, %
	bool bErg = true;          // a workout's targets go to the trainer
	int32 Weather = 0;         // 0 the route's own, 1 clear, 2 overcast, 3 rain
	int32 TimeOfDay = 0;       // 0 the route's own, 1 morning, 2 midday, 3 evening, 4 sunset
	float Grass = 100.f;       // how far the grass reaches, % of GrassStandard (RidePawn.cpp)
	float LookSpeed = 100.f;   // how fast the mouse turns the view, % of 0.12 degrees per count
	float HeadSway = 65.f;     // how much the head moves with the pedalling and the road, %
	float BikeSway = 100.f;    // how much the bike rolls with the pedalling and into bends, %
	// 4-step curated motion level: 0 Off, 1 Low, 2 Medium (default), 3 High.
	int32 MotionLevel() const
	{
		if (BikeSway < 5.f && HeadSway < 5.f) return 0;
		if (BikeSway < 75.f) return 1;
		if (BikeSway < 125.f) return 2;
		return 3;
	}
	void SetMotionLevel(int32 Level)
	{
		static constexpr float BikeVals[4] = {0.f, 45.f, 100.f, 150.f};
		static constexpr float HeadVals[4] = {0.f, 25.f, 65.f, 110.f};
		const int32 I = FMath::Clamp(Level, 0, 3);
		BikeSway = BikeVals[I];
		HeadSway = HeadVals[I];
	}
	int32 View = 0;            // ERideView: the eyes, over the bars, behind, or steady
	bool bHud = true;          // the ride's panels shown (the dock's H)
	bool bGraphs = false;      // power and heart rate across the foot of the screen (G)
	bool bFps = false;         // the frame rate in the ride's bottom-right corner (Graphics)
	float MasterVolume = 85.f;   // 0..100% overall audio volume
	float BikeVolume = 100.f;    // 0..100% wind, tires, freehub & drivetrain
	float AmbienceVolume = 100.f;// 0..100% coastal surf & park/forest birdsong
	int32 GraphicsQuality = 2;   // 0 Low, 1 Medium, 2 High (default), 3 Epic; -1 custom
	int32 Upscaling = 1;         // 0 Off (100%), 1 Quality (75%), 2 Balanced (67%), 3 Performance (50%)
	// The rider's look (RiderKit.h): a swatch per part, -1 for the rider's own colour (Settings > Kit).
	int32 Kit[Veloverse::Kit::Parts] = {-1, -1, -1, -1, -1, -1, -1, -1};

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
