// The ride's dock: a row of buttons along the foot of the screen that rises when the mouse moves
// and sinks away after a few seconds still. Behind it, when on, the graphs: power and heart rate
// across the whole width; the dock stays where it is either way. In the HUD's 1280x720 frame (wider on wider screens); no engine, so
// tests/dock_test.cpp checks it.
#pragma once

#include <algorithm>
#include <array>

namespace Veloverse
{
enum class EDock : int
{
	View,   // V: through the rider's eyes, over the bars, or behind (the ones the bike's assets allow)
	Motion, // M: camera & bike motion level (Off, Low, Medium, High)
	Graphs,  // G: power and heart rate across the foot of the screen
	Hud,     // H: the panels on or off
	Time,    // T: daytime (Auto, Morning, Midday, Evening)
	Weather, // C: weather (Auto, Clear, Overcast, Rain)
	Route,   // R: ride on along another route
	Look,   // Tab: the mouse turns the head freely, the cursor and dock away; Tab again for them
	Pause,  // Esc
	Count,
};
constexpr int DockCount = static_cast<int>(EDock::Count);

struct FDockBox
{
	float X = 0.f, Y = 0.f, W = 0.f, H = 0.f;
	bool Contains(float Px, float Py) const { return Px >= X && Px < X + W && Py >= Y && Py < Y + H; }
};

struct FDockLayout
{
	static constexpr float Button = 44.f, Gap = 6.f, Pad = 8.f, Tall = Button + 2.f * Pad;
	static constexpr float Margin = 16.f, Slide = 18.f;
	static constexpr float GraphsTall = 128.f;

	float Floor = 720.f; // the frame's foot
	float Shown = 0.f;   // 0 away, 1 up
	FDockBox Panel;
	std::array<FDockBox, DockCount> Buttons{};

	// The lowest a panel over the foot of the screen should sit, clear of the dock as it rises.
	float Clear() const { return Floor - (Margin + Tall) * Shown; }

	// The button at a point in the frame, or -1. Only once the dock is mostly up.
	int Hit(float X, float Y) const
	{
		if (Shown < 0.6f)
		{
			return -1;
		}
		for (int I = 0; I < DockCount; ++I)
		{
			if (Buttons[I].Contains(X, Y))
			{
				return I;
			}
		}
		return -1;
	}
};

// The dock for a frame W wide and H tall, raised by Shown.
inline FDockLayout LayoutDock(float W, float H, float Shown)
{
	FDockLayout L;
	L.Shown = std::clamp(Shown, 0.f, 1.f);
	L.Floor = H;
	const float Wide = DockCount * FDockLayout::Button + (DockCount - 1) * FDockLayout::Gap + 2.f * FDockLayout::Pad;
	// Eased out: it slides the last few units up as it fades in.
	const float Rise = 1.f - (1.f - L.Shown) * (1.f - L.Shown);
	L.Panel = {(W - Wide) * 0.5f, L.Floor - FDockLayout::Margin - FDockLayout::Tall + (1.f - Rise) * FDockLayout::Slide, Wide,
	           FDockLayout::Tall};
	for (int I = 0; I < DockCount; ++I)
	{
		L.Buttons[I] = {L.Panel.X + FDockLayout::Pad + I * (FDockLayout::Button + FDockLayout::Gap), L.Panel.Y + FDockLayout::Pad,
		                FDockLayout::Button, FDockLayout::Button};
	}
	return L;
}

// When the dock shows: up while the mouse moves or rests on it, away after Linger seconds still.
struct FDockReveal
{
	static constexpr float Linger = 3.f, Rise = 0.18f, Fall = 0.3f;
	float Still = 1e9f; // seconds since the mouse moved
	float Shown = 0.f;

	void Moved() { Still = 0.f; }
	void Hide() { Still = 1e9f; }
	// bHeld: the pointer is over the dock, or a button is down on it.
	void Tick(float Dt, bool bHeld)
	{
		Still = bHeld ? 0.f : Still + Dt;
		const bool bUp = Still < Linger;
		Shown = bUp ? std::min(1.f, Shown + Dt / Rise) : std::max(0.f, Shown - Dt / Fall);
	}
	bool Visible() const { return Shown > 0.f; }
};
} // namespace Veloverse
