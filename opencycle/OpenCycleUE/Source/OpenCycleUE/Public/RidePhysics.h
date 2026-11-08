// How fast a rider goes for the power they put down. Plain C++ with no engine types, so it can be
// tested on its own (tests/ride_physics_test.cpp).
#pragma once

#include <algorithm>
#include <cmath>

namespace OpenCycle
{
struct FRider
{
	// A touring rider: sat up on a steel bike with wider tyres, about 28 km/h for 220 W on the flat.
	double MassKg = 90.0;     // rider and bike
	double Crr = 0.008;       // rolling resistance, wider tyres on a country lane
	double CdA = 0.55;        // drag area sat up, m^2
	double AirDensity = 1.225;
	double Drivetrain = 0.97; // share of pedal power that reaches the wheel
	double WheelMassKg = 1.0; // the wheels' spin, added to the mass to be accelerated
};

constexpr double Gravity = 9.81;

// The resisting force at a speed (m/s) on a grade (rise over run), in newtons.
inline double Resistance(const FRider& Rider, double Speed, double Grade)
{
	const double Theta = std::atan(Grade);
	const double Climb = Rider.MassKg * Gravity * std::sin(Theta);
	const double Rolling = Rider.MassKg * Gravity * Rider.Crr * std::cos(Theta);
	const double Drag = 0.5 * Rider.AirDensity * Rider.CdA * Speed * std::abs(Speed);
	return Climb + Rolling + Drag;
}

// The speed after Dt seconds of Watts at the pedals. Below 1 m/s the push is held at what it would
// be at 1 m/s, so a standing start doesn't divide by zero. The rider never rolls backwards: on a
// climb too steep for their power they come to a stop.
inline double Step(const FRider& Rider, double Speed, double Watts, double Grade, double Dt)
{
	const double Push = std::max(Watts, 0.0) * Rider.Drivetrain / std::max(Speed, 1.0);
	const double Accel = (Push - Resistance(Rider, Speed, Grade)) / (Rider.MassKg + Rider.WheelMassKg);
	return std::max(Speed + Accel * Dt, 0.0);
}

// The speed that Watts holds on a grade, found by bisection: where push meets resistance.
inline double SteadySpeed(const FRider& Rider, double Watts, double Grade)
{
	double Lo = 0.0, Hi = 40.0;
	for (int I = 0; I < 60; ++I)
	{
		const double Mid = 0.5 * (Lo + Hi);
		const double Push = Watts * Rider.Drivetrain / std::max(Mid, 1e-3);
		(Push > Resistance(Rider, Mid, Grade) ? Lo : Hi) = Mid;
	}
	return 0.5 * (Lo + Hi);
}
} // namespace OpenCycle
