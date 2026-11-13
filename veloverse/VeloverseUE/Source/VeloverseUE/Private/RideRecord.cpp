#include "RideRecord.h"

#include "FitFile.h"
#include "Progress.h"

#include "Dom/JsonObject.h"
#include "HAL/FileManager.h"
#include "HAL/PlatformFileManager.h"
#include "HAL/PlatformMisc.h"
#include "HAL/PlatformProcess.h"
#include "Misc/FileHelper.h"
#include "Misc/Paths.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"

namespace
{
FString ProfileFile() { return FRideProfile::Folder() / TEXT("profile.json"); }

TSharedPtr<FJsonObject> ReadJson(const FString& File)
{
	FString Text;
	TSharedPtr<FJsonObject> Object;
	if (FFileHelper::LoadFileToString(Text, *File))
	{
		FJsonSerializer::Deserialize(TJsonReaderFactory<>::Create(Text), Object);
	}
	return Object;
}

bool WriteJson(const FString& File, const TSharedRef<FJsonObject>& Object)
{
	FString Text;
	FJsonSerializer::Serialize(Object, TJsonWriterFactory<>::Create(&Text));
	return FFileHelper::SaveStringToFile(Text, *File, FFileHelper::EEncodingOptions::ForceUTF8WithoutBOM);
}

TSharedRef<FJsonObject> SummaryJson(const Veloverse::FSummary& S)
{
	TSharedRef<FJsonObject> O = MakeShared<FJsonObject>();
	O->SetNumberField(TEXT("seconds"), S.Seconds);
	O->SetNumberField(TEXT("metres"), S.Metres);
	O->SetNumberField(TEXT("climbed_m"), S.Climbed);
	O->SetNumberField(TEXT("average_w"), S.AverageWatts);
	O->SetNumberField(TEXT("normalized_w"), S.NormalizedWatts);
	O->SetNumberField(TEXT("max_w"), S.MaxWatts);
	O->SetNumberField(TEXT("kj"), S.Kilojoules);
	O->SetNumberField(TEXT("average_rpm"), S.AverageCadence);
	O->SetNumberField(TEXT("average_bpm"), S.AverageHeartRate);
	O->SetNumberField(TEXT("max_bpm"), S.MaxHeartRate);
	O->SetNumberField(TEXT("average_ms"), S.AverageSpeed);
	O->SetNumberField(TEXT("intensity"), S.Intensity);
	O->SetNumberField(TEXT("tss"), S.Tss);
	TArray<TSharedPtr<FJsonValue>> Zones, Best;
	for (const double Z : S.ZoneSeconds)
	{
		Zones.Add(MakeShared<FJsonValueNumber>(Z));
	}
	for (const double B : S.Best)
	{
		Best.Add(MakeShared<FJsonValueNumber>(B));
	}
	O->SetArrayField(TEXT("zone_seconds"), Zones);
	O->SetArrayField(TEXT("best_w"), Best); // over RideStats.h's BestDurations
	return O;
}

Veloverse::FSummary SummaryOf(const FJsonObject& O)
{
	Veloverse::FSummary S;
	S.Seconds = O.GetNumberField(TEXT("seconds"));
	S.Metres = O.GetNumberField(TEXT("metres"));
	S.Climbed = O.GetNumberField(TEXT("climbed_m"));
	S.AverageWatts = O.GetNumberField(TEXT("average_w"));
	S.NormalizedWatts = O.GetNumberField(TEXT("normalized_w"));
	S.MaxWatts = O.GetNumberField(TEXT("max_w"));
	S.Kilojoules = O.GetNumberField(TEXT("kj"));
	S.AverageCadence = O.GetNumberField(TEXT("average_rpm"));
	S.AverageHeartRate = O.GetNumberField(TEXT("average_bpm"));
	S.MaxHeartRate = O.GetNumberField(TEXT("max_bpm"));
	S.AverageSpeed = O.GetNumberField(TEXT("average_ms"));
	S.Intensity = O.GetNumberField(TEXT("intensity"));
	S.Tss = O.GetNumberField(TEXT("tss"));
	const TArray<TSharedPtr<FJsonValue>>* Values = nullptr;
	if (O.TryGetArrayField(TEXT("zone_seconds"), Values))
	{
		for (int32 I = 0; I < FMath::Min(Values->Num(), 7); ++I)
		{
			S.ZoneSeconds[I] = (*Values)[I]->AsNumber();
		}
	}
	if (O.TryGetArrayField(TEXT("best_w"), Values))
	{
		for (int32 I = 0; I < FMath::Min(Values->Num(), static_cast<int32>(S.Best.size())); ++I)
		{
			S.Best[I] = (*Values)[I]->AsNumber();
		}
	}
	return S;
}
} // namespace

FString FRideRecorder::RidesFolder() { return FRideProfile::Folder() / TEXT("Rides"); }

// The rider's own folder (~/.config/Veloverse on Linux), outside the game's install, so a new build
// unpacked over or beside the old one keeps the profile, rides and segment history. Anything a build
// before this kept in the game's Saved folder moves across the first time.
FString FRideProfile::Folder()
{
	static const FString Path = []
	{
#if PLATFORM_LINUX
		// UserSettingsDir is ~/.config/Epic on Linux, so go by the XDG rule ourselves.
		FString Config = FPlatformMisc::GetEnvironmentVariable(TEXT("XDG_CONFIG_HOME"));
		if (Config.IsEmpty()) Config = FPaths::Combine(FPlatformProcess::UserHomeDir(), TEXT(".config"));
		const FString Mine = FPaths::Combine(Config, TEXT("Veloverse"));
#else
		const FString Mine = FPaths::Combine(FPlatformProcess::UserSettingsDir(), TEXT("Veloverse"));
#endif
		const FString Old = FPaths::ProjectSavedDir() / TEXT("Veloverse");
		IFileManager& Files = IFileManager::Get();
		// An empty file in the folder's place is a stub from a failed move in a 0.2.0 test build.
		if (Files.FileExists(*Mine) && Files.FileSize(*Mine) == 0) Files.Delete(*Mine);
		if (!Files.DirectoryExists(*Mine) && Files.DirectoryExists(*Old))
		{
			Files.MakeDirectory(*FPaths::GetPath(Mine), true);
			// Copied, not moved: a move across filesystems leaves an empty file behind. The old folder
			// stays as a backup.
			if (!FPlatformFileManager::Get().GetPlatformFile().CopyDirectoryTree(*Mine, *Old, false))
			{
				UE_LOG(LogTemp, Warning, TEXT("RIDE couldn't move %s to %s"), *Old, *Mine);
				return Old;
			}
		}
		UE_LOG(LogTemp, Display, TEXT("RIDE profile folder %s"), *Mine);
		return Mine;
	}();
	return Path;
}

bool FRideProfile::Saved() { return IFileManager::Get().FileExists(*ProfileFile()); }

FRideProfile FRideProfile::Load()
{
	FRideProfile Profile;
	if (const TSharedPtr<FJsonObject> O = ReadJson(ProfileFile()))
	{
		double Value = 0.0;
		if (O->TryGetNumberField(TEXT("ftp_w"), Value))
		{
			Profile.Ftp = FMath::Clamp(static_cast<float>(Value), 50.f, 600.f);
		}
		if (O->TryGetNumberField(TEXT("weight_kg"), Value))
		{
			Profile.WeightKg = FMath::Clamp(static_cast<float>(Value), 30.f, 200.f);
		}
		if (O->TryGetNumberField(TEXT("bike_kg"), Value))
		{
			Profile.BikeKg = FMath::Clamp(static_cast<float>(Value), 5.f, 30.f);
		}
		if (O->TryGetNumberField(TEXT("xp"), Value))
		{
			Profile.Xp = FMath::Max(static_cast<int64>(Value), int64{0});
		}
		O->TryGetStringField(TEXT("theme"), Profile.Theme);
		if (O->TryGetNumberField(TEXT("max_bpm"), Value))
		{
			Profile.MaxHeartRate = FMath::Clamp(static_cast<int32>(Value), 120, 230);
		}
		O->TryGetBoolField(TEXT("imperial"), Profile.bImperial);
		O->TryGetBoolField(TEXT("erg"), Profile.bErg);
		if (O->TryGetNumberField(TEXT("fov"), Value))
		{
			Profile.Fov = FMath::Clamp(static_cast<float>(Value), 45.f, 100.f);
		}
		if (O->TryGetNumberField(TEXT("difficulty"), Value))
		{
			Profile.Difficulty = FMath::Clamp(static_cast<float>(Value), 0.f, 100.f);
		}
		if (O->TryGetNumberField(TEXT("weather"), Value))
		{
			Profile.Weather = FMath::Clamp(static_cast<int32>(Value), 0, 3);
		}
		if (O->TryGetNumberField(TEXT("time_of_day"), Value))
		{
			Profile.TimeOfDay = FMath::Clamp(static_cast<int32>(Value), 0, 3);
		}
		if (O->TryGetNumberField(TEXT("grass"), Value))
		{
			Profile.Grass = FMath::Clamp(static_cast<float>(Value), 25.f, 200.f);
		}
	}
	return Profile;
}

void FRideProfile::Save() const
{
	IFileManager::Get().MakeDirectory(*Folder(), true);
	TSharedRef<FJsonObject> O = MakeShared<FJsonObject>();
	O->SetNumberField(TEXT("ftp_w"), Ftp);
	O->SetNumberField(TEXT("weight_kg"), WeightKg);
	O->SetNumberField(TEXT("bike_kg"), BikeKg);
	O->SetNumberField(TEXT("xp"), static_cast<double>(Xp));
	O->SetStringField(TEXT("theme"), Theme);
	O->SetNumberField(TEXT("max_bpm"), MaxHeartRate);
	O->SetBoolField(TEXT("imperial"), bImperial);
	O->SetBoolField(TEXT("erg"), bErg);
	O->SetNumberField(TEXT("fov"), Fov);
	O->SetNumberField(TEXT("difficulty"), Difficulty);
	O->SetNumberField(TEXT("weather"), Weather);
	O->SetNumberField(TEXT("time_of_day"), TimeOfDay);
	O->SetNumberField(TEXT("grass"), Grass);
	WriteJson(ProfileFile(), O);
}

FLinearColor FRideProfile::Accent() const
{
	unsigned Rgb = Veloverse::Themes()[0].Rgb;
	for (const Veloverse::FTheme& T : Veloverse::Themes())
	{
		if (Theme == UTF8_TO_TCHAR(T.Name) && Veloverse::IsUnlocked(T, Xp))
		{
			Rgb = T.Rgb;
		}
	}
	return FLinearColor(FColor((Rgb >> 16) & 0xFF, (Rgb >> 8) & 0xFF, Rgb & 0xFF));
}

void FRideRecorder::Reset()
{
	Recorded.clear();
	Clock = Watts = Cadence = Heart = Span = 0.0;
}

void FRideRecorder::Tick(float Dt, double Metres, double InWatts, double InCadence, double HeartRate, double Speed,
                         double Elevation, double East, double South)
{
	if (Recorded.empty() && Span == 0.0)
	{
		Started = FDateTime::Now();
	}
	Clock += Dt;
	Span += Dt;
	Watts += InWatts * Dt;
	Cadence += InCadence * Dt;
	Heart += HeartRate * Dt;
	// A sample for each whole second ridden, carrying that second's averages.
	while (Clock >= static_cast<double>(Recorded.size() + 1))
	{
		Veloverse::FSample S;
		S.Seconds = static_cast<double>(Recorded.size() + 1);
		S.Metres = Metres;
		S.Watts = Watts / Span;
		S.Cadence = Cadence / Span;
		S.HeartRate = Heart / Span;
		S.Speed = Speed;
		S.Elevation = Elevation;
		S.East = East;
		S.South = South;
		Recorded.push_back(S);
		Watts = Cadence = Heart = Span = 0.0;
	}
}

FString FRideRecorder::Save(const FString& Route, float StartMetres, const FRideProfile& Profile,
                           const TArray<FRideClimb>& Climbs, int32 Xp) const
{
	if (Recorded.empty() || Recorded.back().Seconds < MinSeconds)
	{
		return FString();
	}
	IFileManager::Get().MakeDirectory(*RidesFolder(), true);
	TSharedRef<FJsonObject> O = MakeShared<FJsonObject>();
	O->SetNumberField(TEXT("version"), 1);
	O->SetStringField(TEXT("started"), Started.ToIso8601());
	O->SetStringField(TEXT("route"), Route);
	O->SetNumberField(TEXT("start_m"), StartMetres);
	O->SetNumberField(TEXT("ftp_w"), Profile.Ftp);
	O->SetNumberField(TEXT("weight_kg"), Profile.WeightKg);
	O->SetNumberField(TEXT("max_bpm"), Profile.MaxHeartRate);
	O->SetNumberField(TEXT("xp"), Xp);
	O->SetObjectField(TEXT("summary"), SummaryJson(Summary(Profile.Ftp)));
	TArray<TSharedPtr<FJsonValue>> Curve, ClimbValues;
	for (const double W : Veloverse::PowerCurve(Recorded))
	{
		Curve.Add(MakeShared<FJsonValueNumber>(FMath::RoundToDouble(W)));
	}
	O->SetArrayField(TEXT("curve_w"), Curve); // over RideReview.h's CurveDurations
	for (const FRideClimb& C : Climbs)
	{
		TSharedRef<FJsonObject> Climb = MakeShared<FJsonObject>();
		Climb->SetStringField(TEXT("name"), C.Name);
		Climb->SetNumberField(TEXT("seconds"), FMath::RoundToDouble(C.Seconds * 10.0) / 10.0);
		Climb->SetNumberField(TEXT("w"), FMath::RoundToDouble(C.Watts));
		Climb->SetNumberField(TEXT("rank"), C.Rank);
		Climb->SetNumberField(TEXT("of"), C.Of);
		Climb->SetBoolField(TEXT("pr"), C.bPr);
		ClimbValues.Add(MakeShared<FJsonValueObject>(Climb));
	}
	O->SetArrayField(TEXT("climbs"), ClimbValues);
	// The samples as columns, one value a second.
	TSharedRef<FJsonObject> Columns = MakeShared<FJsonObject>();
	auto Column = [&](const TCHAR* Name, double Veloverse::FSample::*Field, double Scale)
	{
		TArray<TSharedPtr<FJsonValue>> Values;
		Values.Reserve(Recorded.size());
		for (const Veloverse::FSample& S : Recorded)
		{
			Values.Add(MakeShared<FJsonValueNumber>(FMath::RoundToDouble(S.*Field * Scale) / Scale));
		}
		Columns->SetArrayField(Name, Values);
	};
	Column(TEXT("m"), &Veloverse::FSample::Metres, 10.0);
	Column(TEXT("w"), &Veloverse::FSample::Watts, 1.0);
	Column(TEXT("rpm"), &Veloverse::FSample::Cadence, 1.0);
	Column(TEXT("bpm"), &Veloverse::FSample::HeartRate, 1.0);
	Column(TEXT("ms"), &Veloverse::FSample::Speed, 100.0);
	Column(TEXT("ele_m"), &Veloverse::FSample::Elevation, 10.0);
	Column(TEXT("east_m"), &Veloverse::FSample::East, 10.0);
	Column(TEXT("south_m"), &Veloverse::FSample::South, 10.0);
	O->SetObjectField(TEXT("samples"), Columns);
	const FString Base = RidesFolder() / Started.ToString(TEXT("%Y%m%d-%H%M%S"));
	if (!WriteJson(Base + TEXT(".json"), O))
	{
		return FString();
	}
	// FIT wants UTC: Started is local time, so take the offset from now.
	const FTimespan ToUtc = FDateTime::UtcNow() - FDateTime::Now();
	const std::vector<uint8_t> Fit = Veloverse::Fit::Activity(Recorded, Summary(Profile.Ftp),
	                                                          (Started + ToUtc).ToUnixTimestamp(), Profile.Ftp);
	FFileHelper::SaveArrayToFile(TArrayView<const uint8>(Fit.data(), static_cast<int32>(Fit.size())),
	                             *(Base + TEXT(".fit")));
	return Base + TEXT(".json");
}

TArray<FRideEntry> FRideRecorder::History()
{
	TArray<FString> Files;
	IFileManager::Get().FindFiles(Files, *(RidesFolder() / TEXT("*.json")), true, false);
	Files.Sort([](const FString& A, const FString& B) { return A > B; }); // named by time: newest first
	TArray<FRideEntry> Out;
	for (const FString& Name : Files)
	{
		const TSharedPtr<FJsonObject> O = ReadJson(RidesFolder() / Name);
		const TSharedPtr<FJsonObject>* Summary = nullptr;
		if (!O || !O->TryGetObjectField(TEXT("summary"), Summary))
		{
			continue;
		}
		FRideEntry Entry;
		Entry.File = RidesFolder() / Name;
		FDateTime::ParseIso8601(*O->GetStringField(TEXT("started")), Entry.Started);
		Entry.Route = O->GetStringField(TEXT("route"));
		Entry.Ftp = O->GetNumberField(TEXT("ftp_w"));
		Entry.Summary = SummaryOf(**Summary);
		double Number = 0.0;
		if (O->TryGetNumberField(TEXT("max_bpm"), Number))
		{
			Entry.MaxHeartRate = static_cast<int32>(Number);
		}
		if (O->TryGetNumberField(TEXT("xp"), Number))
		{
			Entry.Xp = static_cast<int32>(Number);
		}
		const TArray<TSharedPtr<FJsonValue>>* Values = nullptr;
		if (O->TryGetArrayField(TEXT("curve_w"), Values))
		{
			for (int32 I = 0; I < FMath::Min(Values->Num(), static_cast<int32>(Entry.Curve.size())); ++I)
			{
				Entry.Curve[I] = (*Values)[I]->AsNumber();
			}
		}
		else
		{
			// Before the curve was kept: the summary's bests sit at some of its durations.
			for (std::size_t B = 0; B < Veloverse::BestDurations.size(); ++B)
			{
				for (std::size_t I = 0; I < Entry.Curve.size(); ++I)
				{
					if (Veloverse::CurveDurations[I] == Veloverse::BestDurations[B])
					{
						Entry.Curve[I] = Entry.Summary.Best[B];
					}
				}
			}
		}
		if (O->TryGetArrayField(TEXT("climbs"), Values))
		{
			for (const TSharedPtr<FJsonValue>& V : *Values)
			{
				const TSharedPtr<FJsonObject>* C = nullptr;
				if (V->TryGetObject(C))
				{
					FRideClimb Climb;
					Climb.Name = (*C)->GetStringField(TEXT("name"));
					Climb.Seconds = (*C)->GetNumberField(TEXT("seconds"));
					Climb.Watts = (*C)->GetNumberField(TEXT("w"));
					Climb.Rank = static_cast<int32>((*C)->GetNumberField(TEXT("rank")));
					Climb.Of = static_cast<int32>((*C)->GetNumberField(TEXT("of")));
					Climb.bPr = (*C)->GetBoolField(TEXT("pr"));
					Entry.Climbs.Add(MoveTemp(Climb));
				}
			}
		}
		Out.Add(MoveTemp(Entry));
	}
	return Out;
}

std::vector<Veloverse::FSample> FRideRecorder::LoadSamples(const FString& File)
{
	std::vector<Veloverse::FSample> Out;
	const TSharedPtr<FJsonObject> O = ReadJson(File);
	const TSharedPtr<FJsonObject>* Columns = nullptr;
	if (!O || !O->TryGetObjectField(TEXT("samples"), Columns))
	{
		return Out;
	}
	const TArray<TSharedPtr<FJsonValue>>* Metres = nullptr;
	if (!(*Columns)->TryGetArrayField(TEXT("m"), Metres))
	{
		return Out;
	}
	Out.resize(Metres->Num());
	for (int32 I = 0; I < Metres->Num(); ++I)
	{
		Out[I].Seconds = I + 1.0;
	}
	auto Column = [&](const TCHAR* Name, double Veloverse::FSample::*Field)
	{
		const TArray<TSharedPtr<FJsonValue>>* Values = nullptr;
		if ((*Columns)->TryGetArrayField(Name, Values))
		{
			for (int32 I = 0; I < FMath::Min(Values->Num(), static_cast<int32>(Out.size())); ++I)
			{
				Out[I].*Field = (*Values)[I]->AsNumber();
			}
		}
	};
	Column(TEXT("m"), &Veloverse::FSample::Metres);
	Column(TEXT("w"), &Veloverse::FSample::Watts);
	Column(TEXT("rpm"), &Veloverse::FSample::Cadence);
	Column(TEXT("bpm"), &Veloverse::FSample::HeartRate);
	Column(TEXT("ms"), &Veloverse::FSample::Speed);
	Column(TEXT("ele_m"), &Veloverse::FSample::Elevation);
	Column(TEXT("east_m"), &Veloverse::FSample::East);
	Column(TEXT("south_m"), &Veloverse::FSample::South);
	return Out;
}
