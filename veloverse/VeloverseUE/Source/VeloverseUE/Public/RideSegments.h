// The route's segments in the ride: its climbs, timed as the rider passes through, each against
// the rider's best (its ghost), and kept between rides in Saved/Veloverse/segments.json. The
// timing and the ghost are Segments.h's; this finds the climbs on a track, names them after their
// stretch, and stores the history.
#pragma once

#include "CoreMinimal.h"
#include "Segments.h"

class URideTrack;

struct FSegmentResult
{
	FString Name;
	Veloverse::FSegmentEffort Effort;
	Veloverse::FRanking Ranking; // against the rides before this one
	double FinishedAt = 0.0;     // ride seconds
};

class FRideSegments
{
public:
	// Finds the track's climbs and reads their history; again whenever the track changes.
	void Use(const URideTrack& Track);
	bool UsesTrack(const URideTrack* Track) const { return Track && Track == Used.Get(); }

	// Every frame of riding: metres along the track, unwrapped, ride seconds and power.
	void Tick(double DistanceM, double Now, double Watts);
	// A new start (a jump, or a new ride): nothing under way counts, and the results start over.
	void Reset();

	struct FLive
	{
		FString Name;
		bool bOn = false;        // riding it now, or else coming up
		double ToGo = 0.0;       // metres to its end, or to its start when coming up
		double Length = 0.0, Grade = 0.0;
		double Elapsed = 0.0;    // seconds into it
		TOptional<double> Gap;   // seconds behind (+) or ahead (-) of the best, when there is one
		TOptional<double> Best;  // the best time
	};
	// The segment the rider is on, or the next within Reach metres.
	TOptional<FLive> Live(double DistanceM, double Reach = 600.0) const;

	const TArray<FSegmentResult>& Results() const { return Finished; }
	int32 Num() const { return Timers.Num(); }

	// Adds this ride's efforts to the history and writes it.
	void Save(const FString& When);

private:
	struct FHistory
	{
		FString Name;
		TArray<Veloverse::FSegmentRecord> Records;
		TOptional<Veloverse::FSegmentEffort> Best;
	};
	TWeakObjectPtr<const URideTrack> Used;
	double TrackLength = 0.0; // metres
	bool bClosed = false;
	TArray<Veloverse::FSegmentTimer> Timers;
	TArray<FString> Keys;    // the history's key for each: route title and where it starts
	TArray<FString> Names;
	TMap<FString, FHistory> History;
	TArray<FSegmentResult> Finished;

	static FString File();
	void Load();
};
