#include "RideSegments.h"

#include "RideRecord.h"
#include "RideTrack.h"

#include "Dom/JsonObject.h"
#include "HAL/FileManager.h"
#include "Misc/FileHelper.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"

FString FRideSegments::File() { return FRideProfile::Folder() / TEXT("segments.json"); }

void FRideSegments::Use(const URideTrack& Track)
{
	Used = &Track;
	TrackLength = Track.Length() / 100.0;
	bClosed = Track.bClosed;
	Timers.Reset();
	Keys.Reset();
	Names.Reset();
	Finished.Reset();
	// The height every 10 m, in metres.
	constexpr double Step = 10.0;
	std::vector<double> Heights;
	for (double M = 0.0; M < TrackLength; M += Step)
	{
		Heights.push_back(Track.Height(static_cast<float>(M * 100.0)) / 100.0);
	}
	const std::vector<Veloverse::FSegment> Climbs = Veloverse::FindClimbs(Heights, Step, bClosed);
	TMap<FString, int32> PerStretch;
	for (const Veloverse::FSegment& Climb : Climbs)
	{
		// Named after the stretch it starts in: "Coast climb", then "Coast climb 2".
		FString Stretch = TEXT("Road");
		for (const FRideStretch& S : Track.Stretches)
		{
			if (S.Metres <= Climb.Start)
			{
				Stretch = S.Name;
			}
		}
		const int32 Count = ++PerStretch.FindOrAdd(Stretch);
		FString Name = FString::Printf(TEXT("%s climb"), *Stretch);
		if (Count > 1)
		{
			Name += FString::Printf(TEXT(" %d"), Count);
		}
		Veloverse::FSegment Named = Climb;
		Named.Name = TCHAR_TO_UTF8(*Name);
		Timers.Emplace(Named, TrackLength, bClosed);
		Names.Add(Name);
		Keys.Add(FString::Printf(TEXT("%s@%.0f"), *Track.Title, Climb.Start));
	}
	Load();
	UE_LOG(LogTemp, Display, TEXT("RIDE segments: %d climbs on %s"), Timers.Num(), *Track.Title);
}

void FRideSegments::Reset()
{
	for (Veloverse::FSegmentTimer& T : Timers)
	{
		T.Reset();
	}
	Finished.Reset(); // a new ride from a new start
}

void FRideSegments::Tick(double DistanceM, double Now, double Watts)
{
	for (int32 I = 0; I < Timers.Num(); ++I)
	{
		if (std::optional<Veloverse::FSegmentEffort> Effort = Timers[I].Update(DistanceM, Now, Watts))
		{
			FSegmentResult R;
			R.Name = Names[I];
			R.Effort = MoveTemp(*Effort);
			const FHistory* Past = History.Find(Keys[I]);
			std::vector<Veloverse::FSegmentRecord> Before;
			if (Past)
			{
				Before.assign(Past->Records.GetData(), Past->Records.GetData() + Past->Records.Num());
			}
			// This ride's earlier passes count too, a lap ago.
			for (const FSegmentResult& Earlier : Finished)
			{
				if (Earlier.Name == R.Name)
				{
					Before.push_back({"", Earlier.Effort.Seconds, Earlier.Effort.AverageWatts});
				}
			}
			R.Ranking = Veloverse::Rank(Before, R.Effort.Seconds);
			R.FinishedAt = Now;
			UE_LOG(LogTemp, Display, TEXT("RIDE segment %s: %.1f s at %.0f W, %d of %d"), *R.Name, R.Effort.Seconds,
			       R.Effort.AverageWatts, R.Ranking.Rank, R.Ranking.Of);
			Finished.Add(MoveTemp(R));
		}
	}
}

TOptional<FRideSegments::FLive> FRideSegments::Live(double DistanceM, double Reach) const
{
	TOptional<FLive> Out;
	double Nearest = Reach;
	for (int32 I = 0; I < Timers.Num(); ++I)
	{
		const Veloverse::FSegmentTimer& T = Timers[I];
		const Veloverse::FSegment& S = T.Seg();
		FLive L;
		L.Name = Names[I];
		L.Length = S.Length(TrackLength);
		L.Grade = S.Grade(TrackLength);
		// The best this ride or before.
		if (const FHistory* Past = History.Find(Keys[I]); Past && Past->Best)
		{
			L.Best = Past->Best->Seconds;
		}
		const Veloverse::FSegmentEffort* Ghost = nullptr;
		if (const FHistory* Past = History.Find(Keys[I]); Past && Past->Best)
		{
			Ghost = &*Past->Best;
		}
		for (const FSegmentResult& R : Finished)
		{
			if (R.Name == L.Name && (!L.Best || R.Effort.Seconds < *L.Best))
			{
				L.Best = R.Effort.Seconds;
				Ghost = &R.Effort;
			}
		}
		if (T.Running())
		{
			L.bOn = true;
			L.ToGo = L.Length - T.Into();
			L.Elapsed = T.Elapsed();
			if (Ghost)
			{
				L.Gap = Veloverse::GhostGap(*Ghost, T.Into(), T.Elapsed());
			}
			return L; // on one: that's the one
		}
		// Coming up: metres to its next start.
		double Ahead = S.Start - FMath::Fmod(DistanceM, FMath::Max(TrackLength, 1.0));
		if (bClosed && Ahead < 0.0)
		{
			Ahead += TrackLength;
		}
		if (Ahead >= 0.0 && Ahead < Nearest)
		{
			Nearest = Ahead;
			L.ToGo = Ahead;
			Out = L;
		}
	}
	return Out;
}

// ---- History ------------------------------------------------------------------------------------
// {"<route>@<start m>": {"name": ..., "efforts": [{"when", "s", "w"}...], "best": {"s", "w", "step",
// "splits": [...]}}}

void FRideSegments::Load()
{
	History.Reset();
	FString Text;
	TSharedPtr<FJsonObject> Root;
	if (!FFileHelper::LoadFileToString(Text, *File()) ||
	    !FJsonSerializer::Deserialize(TJsonReaderFactory<>::Create(Text), Root) || !Root)
	{
		return;
	}
	for (const auto& [Key, Value] : Root->Values)
	{
		const TSharedPtr<FJsonObject>* O = nullptr;
		if (!Value->TryGetObject(O))
		{
			continue;
		}
		FHistory H;
		(*O)->TryGetStringField(TEXT("name"), H.Name);
		const TArray<TSharedPtr<FJsonValue>>* Efforts = nullptr;
		if ((*O)->TryGetArrayField(TEXT("efforts"), Efforts))
		{
			for (const TSharedPtr<FJsonValue>& E : *Efforts)
			{
				const TSharedPtr<FJsonObject>* EO = nullptr;
				if (E->TryGetObject(EO))
				{
					H.Records.Add({TCHAR_TO_UTF8(*(*EO)->GetStringField(TEXT("when"))),
					               (*EO)->GetNumberField(TEXT("s")), (*EO)->GetNumberField(TEXT("w"))});
				}
			}
		}
		const TSharedPtr<FJsonObject>* Best = nullptr;
		if ((*O)->TryGetObjectField(TEXT("best"), Best))
		{
			Veloverse::FSegmentEffort B;
			B.Seconds = (*Best)->GetNumberField(TEXT("s"));
			B.AverageWatts = (*Best)->GetNumberField(TEXT("w"));
			B.Step = (*Best)->GetNumberField(TEXT("step"));
			const TArray<TSharedPtr<FJsonValue>>* Splits = nullptr;
			if ((*Best)->TryGetArrayField(TEXT("splits"), Splits))
			{
				for (const TSharedPtr<FJsonValue>& S : *Splits)
				{
					B.Splits.push_back(S->AsNumber());
				}
			}
			if (B.Step > 0.0 && B.Splits.size() >= 2)
			{
				H.Best = MoveTemp(B);
			}
		}
		History.Add(Key, MoveTemp(H));
	}
}

void FRideSegments::Save(const FString& When)
{
	if (Finished.IsEmpty())
	{
		return;
	}
	Load(); // what's on disk now, in case another ride wrote since
	for (const FSegmentResult& R : Finished)
	{
		const int32 I = Names.IndexOfByKey(R.Name);
		if (I == INDEX_NONE)
		{
			continue;
		}
		FHistory& H = History.FindOrAdd(Keys[I]);
		H.Name = R.Name;
		H.Records.Add({TCHAR_TO_UTF8(*When), R.Effort.Seconds, R.Effort.AverageWatts});
		if (!H.Best || R.Effort.Seconds < H.Best->Seconds)
		{
			H.Best = R.Effort;
		}
	}
	TSharedRef<FJsonObject> Root = MakeShared<FJsonObject>();
	for (const auto& [Key, H] : History)
	{
		TSharedRef<FJsonObject> O = MakeShared<FJsonObject>();
		O->SetStringField(TEXT("name"), H.Name);
		TArray<TSharedPtr<FJsonValue>> Efforts;
		for (const Veloverse::FSegmentRecord& R : H.Records)
		{
			TSharedRef<FJsonObject> E = MakeShared<FJsonObject>();
			E->SetStringField(TEXT("when"), UTF8_TO_TCHAR(R.When.c_str()));
			E->SetNumberField(TEXT("s"), FMath::RoundToDouble(R.Seconds * 10.0) / 10.0);
			E->SetNumberField(TEXT("w"), FMath::RoundToDouble(R.AverageWatts));
			Efforts.Add(MakeShared<FJsonValueObject>(E));
		}
		O->SetArrayField(TEXT("efforts"), Efforts);
		if (H.Best)
		{
			TSharedRef<FJsonObject> B = MakeShared<FJsonObject>();
			B->SetNumberField(TEXT("s"), H.Best->Seconds);
			B->SetNumberField(TEXT("w"), H.Best->AverageWatts);
			B->SetNumberField(TEXT("step"), H.Best->Step);
			TArray<TSharedPtr<FJsonValue>> Splits;
			for (const double S : H.Best->Splits)
			{
				Splits.Add(MakeShared<FJsonValueNumber>(FMath::RoundToDouble(S * 100.0) / 100.0));
			}
			B->SetArrayField(TEXT("splits"), Splits);
			O->SetObjectField(TEXT("best"), B);
		}
		Root->SetObjectField(Key, O);
	}
	IFileManager::Get().MakeDirectory(*FRideProfile::Folder(), true);
	FString Text;
	FJsonSerializer::Serialize(Root, TJsonWriterFactory<>::Create(&Text));
	FFileHelper::SaveStringToFile(Text, *File(), FFileHelper::EEncodingOptions::ForceUTF8WithoutBOM);
}
