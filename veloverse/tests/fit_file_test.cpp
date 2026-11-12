// Checks FitFile.h without the engine. Run with tests/run.sh. With a path argument, also writes a
// sample ride there, for checking with another FIT reader.
#include "FitFile.h"

#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got)
{
	std::printf("%s %s (%.6f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

int main(int Argc, char** Argv)
{
	// The CRC of "123456789" under FIT's CRC-16 (ARC) is 0xBB3D.
	const char* Digits = "123456789";
	Check(Fit::Crc(reinterpret_cast<const uint8_t*>(Digits), 9) == 0xBB3D, "FIT's CRC", Fit::Crc(reinterpret_cast<const uint8_t*>(Digits), 9));

	// The origin is Point Nemo; 1 km south is about 0.009 degrees, and east widens with latitude.
	double Lat = 0.0, Lon = 0.0;
	Geo::ToLatLon(0.0, 0.0, Lat, Lon);
	Check(Lat == Geo::OriginLat && Lon == Geo::OriginLon, "the origin is Point Nemo", Lat);
	Geo::ToLatLon(0.0, 1000.0, Lat, Lon);
	Check(std::abs(Lat - Geo::OriginLat + 0.008993) < 1e-5, "1 km south", Lat - Geo::OriginLat);
	Geo::ToLatLon(1000.0, 0.0, Lat, Lon);
	Check(std::abs(Lon - Geo::OriginLon - 0.013680) < 1e-4, "1 km east at 49 S", Lon - Geo::OriginLon);

	// Ten minutes round a circle.
	std::vector<FSample> Ride;
	for (int I = 1; I <= 600; ++I)
	{
		FSample S;
		S.Seconds = I;
		S.Metres = I * 8.0;
		S.Speed = 8.0;
		S.Watts = I % 120 < 60 ? 300.0 : 150.0;
		S.Cadence = 90.0;
		S.Elevation = 20.0 + 10.0 * std::sin(I / 100.0);
		S.East = 800.0 * std::cos(I / 100.0);
		S.South = 800.0 * std::sin(I / 100.0);
		Ride.push_back(S);
	}
	const FSummary Sum = Summarize(Ride, 250.0);
	const std::vector<uint8_t> File = Fit::Activity(Ride, Sum, 1794460000, 250.0);
	Check(File.size() > 600 * 20, "a record a second", double(File.size()));
	Check(File[0] == 14 && File[8] == '.' && File[9] == 'F' && File[10] == 'I' && File[11] == 'T', "the header", File[0]);
	const uint32_t DataSize = File[4] | File[5] << 8 | File[6] << 16 | uint32_t(File[7]) << 24;
	Check(DataSize + 16 == File.size(), "the data size", DataSize);
	Check(Fit::Crc(File.data(), 14) == 0, "the header's CRC checks", 0.0);
	Check(Fit::Crc(File.data(), File.size()) == 0, "the file's CRC checks", 0.0);
	Check(Fit::Scaled(-1.0, 1.0, Fit::EType::UInt16) == 0xFFFF && Fit::Scaled(70000, 1.0, Fit::EType::UInt16) == 0xFFFF,
	      "out of range is no value", 0.0);
	Check(Fit::Scaled(0.0, 1.0, Fit::EType::UInt8, false) == 0xFF, "a missing heart rate is no value", 0.0);
	if (Argc > 1)
	{
		if (FILE* Out = std::fopen(Argv[1], "wb"))
		{
			std::fwrite(File.data(), 1, File.size(), Out);
			std::fclose(Out);
		}
	}
	std::printf("%s\n", Failures ? "FAILED" : "passed");
	return Failures ? 1 : 0;
}
