// Rides as Garmin FIT activity files, the format Strava, Garmin Connect, TrainingPeaks and
// intervals.icu take: a file_id, timer events, a record a second (position, altitude, distance,
// speed, power, cadence, heart rate), a lap, a session and the activity. Plain C++ with no engine
// types, tested by tests/fit_file_test.cpp.
//
// The island isn't anywhere, so it's put somewhere: its origin at Point Nemo in the South Pacific,
// the ocean's farthest point from land, so a map draws the route over open water. The game's +X is
// east and +Y south, as a map seen from above in the editor.
#pragma once

#include "RideStats.h"

#include <cmath>
#include <cstdint>
#include <initializer_list>
#include <string>
#include <vector>

namespace Veloverse
{
namespace Geo
{
constexpr double OriginLat = -48.876667; // Point Nemo
constexpr double OriginLon = -123.393333;
constexpr double EarthRadius = 6371008.8; // m, the mean

// Degrees for a point East and South metres from the origin, on a plane tangent there: across the
// island's 50 km the error is well under a metre.
inline void ToLatLon(double EastM, double SouthM, double& Lat, double& Lon)
{
	constexpr double Deg = 180.0 / 3.14159265358979323846;
	Lat = OriginLat - SouthM / EarthRadius * Deg;
	Lon = OriginLon + EastM / (EarthRadius * std::cos(OriginLat / Deg)) * Deg;
}
} // namespace Geo

namespace Fit
{
// Seconds from the Unix epoch to FIT's, 31 Dec 1989 00:00 UTC.
constexpr int64_t EpochOffset = 631065600;

inline uint16_t Crc(uint16_t Crc, uint8_t Byte)
{
	static constexpr uint16_t Table[16] = {0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401,
	                                       0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400};
	uint16_t T = Table[Crc & 0xF];
	Crc = (Crc >> 4) & 0x0FFF;
	Crc = Crc ^ T ^ Table[Byte & 0xF];
	T = Table[Crc & 0xF];
	Crc = (Crc >> 4) & 0x0FFF;
	return Crc ^ T ^ Table[(Byte >> 4) & 0xF];
}

inline uint16_t Crc(const uint8_t* Data, std::size_t Size, uint16_t Start = 0)
{
	for (std::size_t I = 0; I < Size; ++I)
	{
		Start = Crc(Start, Data[I]);
	}
	return Start;
}

// Base types: the byte in a field definition, and the value meaning "no value".
enum class EType : uint8_t
{
	Enum = 0x00,
	UInt8 = 0x02,
	SInt16 = 0x83,
	UInt16 = 0x84,
	SInt32 = 0x85,
	UInt32 = 0x86,
	UInt32z = 0x8C,
};

inline int Size(EType T)
{
	switch (T)
	{
	case EType::Enum:
	case EType::UInt8: return 1;
	case EType::SInt16:
	case EType::UInt16: return 2;
	default: return 4;
	}
}

inline int64_t Invalid(EType T)
{
	switch (T)
	{
	case EType::Enum:
	case EType::UInt8: return 0xFF;
	case EType::SInt16: return 0x7FFF;
	case EType::UInt16: return 0xFFFF;
	case EType::SInt32: return 0x7FFFFFFF;
	case EType::UInt32z: return 0;
	default: return 0xFFFFFFFF;
	}
}

struct FField
{
	uint8_t Number;
	EType Type;
	int64_t Value;
};

// Writes definition and data messages, each message type on its own local number, defined the
// first time it's written (a message's fields must be the same every time).
class FWriter
{
public:
	void Message(uint16_t Global, std::initializer_list<FField> Fields)
	{
		int Local = -1;
		for (std::size_t I = 0; I < Defined.size(); ++I)
		{
			Local = Defined[I] == Global ? static_cast<int>(I) : Local;
		}
		if (Local < 0)
		{
			Local = static_cast<int>(Defined.size());
			Defined.push_back(Global);
			Data.push_back(static_cast<uint8_t>(0x40 | Local));
			Data.push_back(0);    // reserved
			Data.push_back(0);    // little-endian
			Put(Global, 2);
			Data.push_back(static_cast<uint8_t>(Fields.size()));
			for (const FField& F : Fields)
			{
				Data.push_back(F.Number);
				Data.push_back(static_cast<uint8_t>(Size(F.Type)));
				Data.push_back(static_cast<uint8_t>(F.Type));
			}
		}
		Data.push_back(static_cast<uint8_t>(Local));
		for (const FField& F : Fields)
		{
			Put(F.Value, Size(F.Type));
		}
	}

	// The whole file: the 14-byte header with its CRC, the messages, and the file's CRC.
	std::vector<uint8_t> File() const
	{
		std::vector<uint8_t> Out;
		auto Put32 = [&Out](uint32_t V)
		{
			for (int I = 0; I < 4; ++I)
			{
				Out.push_back(static_cast<uint8_t>(V >> (8 * I)));
			}
		};
		Out.push_back(14);
		Out.push_back(0x20); // protocol 2.0
		Out.push_back(static_cast<uint8_t>(2195 & 0xFF)); // profile 21.95
		Out.push_back(static_cast<uint8_t>(2195 >> 8));
		Put32(static_cast<uint32_t>(Data.size()));
		for (const char C : {'.', 'F', 'I', 'T'})
		{
			Out.push_back(static_cast<uint8_t>(C));
		}
		const uint16_t HeaderCrc = Crc(Out.data(), Out.size());
		Out.push_back(static_cast<uint8_t>(HeaderCrc));
		Out.push_back(static_cast<uint8_t>(HeaderCrc >> 8));
		Out.insert(Out.end(), Data.begin(), Data.end());
		const uint16_t FileCrc = Crc(Out.data(), Out.size());
		Out.push_back(static_cast<uint8_t>(FileCrc));
		Out.push_back(static_cast<uint8_t>(FileCrc >> 8));
		return Out;
	}

private:
	std::vector<uint8_t> Data;
	std::vector<uint16_t> Defined;

	void Put(int64_t V, int Bytes)
	{
		const uint64_t U = static_cast<uint64_t>(V);
		for (int I = 0; I < Bytes; ++I)
		{
			Data.push_back(static_cast<uint8_t>(U >> (8 * I)));
		}
	}
};

// A scaled value, rounded, or the type's "no value" when it's missing (Have false) or won't fit.
inline int64_t Scaled(double V, double Scale, EType T, bool Have = true, double Offset = 0.0)
{
	if (!Have || !std::isfinite(V))
	{
		return Invalid(T);
	}
	const int64_t R = std::llround((V + Offset) * Scale);
	const int64_t Max = Invalid(T);
	const bool bSigned = T == EType::SInt16 || T == EType::SInt32;
	return (R < (bSigned ? -Max : 0) || R >= Max) ? Invalid(T) : R;
}

inline int64_t Semicircles(double Degrees) { return std::llround(Degrees * (2147483648.0 / 180.0)); }

// A ride as a FIT activity: Samples a second apart from StartUnix (seconds, UTC), with positions
// where bPositions. Sport is cycling, sub-sport virtual activity, as Strava files a virtual ride.
inline std::vector<uint8_t> Activity(const std::vector<FSample>& Samples, const FSummary& S, int64_t StartUnix,
                                     double Ftp, bool bPositions = true)
{
	using E = EType;
	FWriter W;
	const int64_t Start = StartUnix - EpochOffset;
	const int64_t End = Start + (Samples.empty() ? 0 : static_cast<int64_t>(Samples.back().Seconds));
	const int64_t Ms = static_cast<int64_t>(S.Seconds * 1000.0);
	// file_id: an activity, made by a "development" manufacturer.
	W.Message(0, {{0, E::Enum, 4}, {1, E::UInt16, 255}, {2, E::UInt16, 1}, {3, E::UInt32z, 1}, {4, E::UInt32, Start}});
	// event: the timer starts.
	W.Message(21, {{253, E::UInt32, Start}, {0, E::Enum, 0}, {1, E::Enum, 0}});
	for (const FSample& P : Samples)
	{
		double Lat = 0.0, Lon = 0.0;
		Geo::ToLatLon(P.East, P.South, Lat, Lon);
		W.Message(20, {
			{253, E::UInt32, Start + static_cast<int64_t>(P.Seconds)},
			{0, E::SInt32, bPositions ? Semicircles(Lat) : Invalid(E::SInt32)},
			{1, E::SInt32, bPositions ? Semicircles(Lon) : Invalid(E::SInt32)},
			{2, E::UInt16, Scaled(P.Elevation, 5.0, E::UInt16, true, 500.0)},
			{3, E::UInt8, Scaled(P.HeartRate, 1.0, E::UInt8, P.HeartRate > 0.0)},
			{4, E::UInt8, Scaled(P.Cadence, 1.0, E::UInt8)},
			{5, E::UInt32, Scaled(P.Metres, 100.0, E::UInt32)},
			{6, E::UInt16, Scaled(P.Speed, 1000.0, E::UInt16)},
			{7, E::UInt16, Scaled(P.Watts, 1.0, E::UInt16)},
		});
	}
	// event: the timer stops, everything.
	W.Message(21, {{253, E::UInt32, End}, {0, E::Enum, 0}, {1, E::Enum, 4}});
	const bool bHeart = S.AverageHeartRate > 0.0;
	W.Message(19, {
		{253, E::UInt32, End}, {0, E::Enum, 9}, {1, E::Enum, 1}, {2, E::UInt32, Start},
		{7, E::UInt32, Ms}, {8, E::UInt32, Ms}, {9, E::UInt32, Scaled(S.Metres, 100.0, E::UInt32)},
		{13, E::UInt16, Scaled(S.AverageSpeed, 1000.0, E::UInt16)},
		{15, E::UInt8, Scaled(S.AverageHeartRate, 1.0, E::UInt8, bHeart)},
		{16, E::UInt8, Scaled(S.MaxHeartRate, 1.0, E::UInt8, bHeart)},
		{17, E::UInt8, Scaled(S.AverageCadence, 1.0, E::UInt8)},
		{19, E::UInt16, Scaled(S.AverageWatts, 1.0, E::UInt16)}, {20, E::UInt16, Scaled(S.MaxWatts, 1.0, E::UInt16)},
		{21, E::UInt16, Scaled(S.Climbed, 1.0, E::UInt16)}, {25, E::Enum, 2}, {39, E::Enum, 58},
	});
	W.Message(18, {
		{253, E::UInt32, End}, {0, E::Enum, 8}, {1, E::Enum, 1}, {2, E::UInt32, Start},
		{5, E::Enum, 2}, {6, E::Enum, 58},
		{7, E::UInt32, Ms}, {8, E::UInt32, Ms}, {9, E::UInt32, Scaled(S.Metres, 100.0, E::UInt32)},
		{11, E::UInt16, Scaled(S.Kilojoules, 1.0, E::UInt16)}, // kcal burnt is about the kJ done
		{14, E::UInt16, Scaled(S.AverageSpeed, 1000.0, E::UInt16)},
		{16, E::UInt8, Scaled(S.AverageHeartRate, 1.0, E::UInt8, bHeart)},
		{17, E::UInt8, Scaled(S.MaxHeartRate, 1.0, E::UInt8, bHeart)},
		{18, E::UInt8, Scaled(S.AverageCadence, 1.0, E::UInt8)},
		{20, E::UInt16, Scaled(S.AverageWatts, 1.0, E::UInt16)}, {21, E::UInt16, Scaled(S.MaxWatts, 1.0, E::UInt16)},
		{22, E::UInt16, Scaled(S.Climbed, 1.0, E::UInt16)},
		{25, E::UInt16, 0}, {26, E::UInt16, 1},
		{34, E::UInt16, Scaled(S.NormalizedWatts, 1.0, E::UInt16)},
		{35, E::UInt16, Scaled(S.Tss, 10.0, E::UInt16)},
		{36, E::UInt16, Scaled(S.Intensity, 1000.0, E::UInt16)},
		{45, E::UInt16, Scaled(Ftp, 1.0, E::UInt16)},
	});
	W.Message(34, {{253, E::UInt32, End}, {0, E::UInt32, Ms}, {1, E::UInt16, 1}, {2, E::Enum, 0}, {3, E::Enum, 26},
	               {4, E::Enum, 1}});
	return W.File();
}
} // namespace Fit
} // namespace Veloverse
