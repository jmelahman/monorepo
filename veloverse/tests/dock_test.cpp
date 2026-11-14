// Checks Dock.h without the engine. Run with tests/run.sh.
#include "Dock.h"

#include <cstdio>

using namespace Veloverse;

static int Failures = 0;

static void Check(bool Ok, const char* What, double Got = 0.0)
{
	std::printf("%s %s (%.3f)\n", Ok ? "ok  " : "FAIL", What, Got);
	Failures += Ok ? 0 : 1;
}

int main()
{
	const FDockLayout Up = LayoutDock(1280.f, 720.f, false, 1.f);
	Check(Up.Panel.X + Up.Panel.W * 0.5f == 640.f, "centred", Up.Panel.X);
	Check(Up.Panel.Y + Up.Panel.H == 720.f - FDockLayout::Margin, "a margin off the foot", Up.Panel.Y);
	bool Inside = true, Apart = true;
	for (int I = 0; I < DockCount; ++I)
	{
		const FDockBox& B = Up.Buttons[I];
		Inside = Inside && B.X >= Up.Panel.X && B.X + B.W <= Up.Panel.X + Up.Panel.W && B.Y >= Up.Panel.Y &&
		         B.Y + B.H <= Up.Panel.Y + Up.Panel.H;
		Apart = Apart && (I == 0 || B.X >= Up.Buttons[I - 1].X + Up.Buttons[I - 1].W);
		const int Hit = Up.Hit(B.X + B.W * 0.5f, B.Y + B.H * 0.5f);
		Check(Hit == I, "each button's middle hits it", Hit);
	}
	Check(Inside, "the buttons sit in the panel");
	Check(Apart, "and don't overlap");
	Check(Up.Hit(Up.Buttons[0].X + Up.Buttons[0].W + 1.f, Up.Buttons[0].Y + 10.f) == -1, "the gap hits nothing");
	Check(Up.Hit(10.f, 10.f) == -1, "the sky hits nothing");

	const FDockLayout Away = LayoutDock(1280.f, 720.f, false, 0.f);
	Check(Away.Hit(Away.Buttons[2].X + 5.f, Away.Buttons[2].Y + 5.f) == -1, "a hidden dock can't be clicked");
	Check(Away.Panel.Y > Up.Panel.Y, "it sinks as it goes", Away.Panel.Y - Up.Panel.Y);
	Check(Away.Clear() == 720.f && Up.Clear() <= Up.Panel.Y, "panels over the foot clear it only while it's up",
	      Up.Clear());

	const FDockLayout OverGraphs = LayoutDock(1280.f, 720.f, true, 1.f);
	Check(OverGraphs.Panel.Y + OverGraphs.Panel.H <= 720.f - FDockLayout::GraphsTall, "over the graphs",
	      OverGraphs.Panel.Y);
	Check(LayoutDock(2240.f, 720.f, false, 1.f).Panel.X > Up.Panel.X, "centred on a wide screen too");

	FDockReveal R;
	R.Tick(0.1f, false);
	Check(!R.Visible(), "away until the mouse moves");
	R.Moved();
	for (int I = 0; I < 10; ++I)
	{
		R.Tick(0.05f, false);
	}
	Check(R.Shown == 1.f, "up within a fifth of a second of moving", R.Shown);
	for (int I = 0; I < 60; ++I)
	{
		R.Tick(0.1f, true);
	}
	Check(R.Shown == 1.f, "stays up while the pointer rests on it", R.Shown);
	for (int I = 0; I < 40; ++I)
	{
		R.Tick(0.1f, false);
	}
	Check(!R.Visible(), "away a few seconds after it's left", R.Shown);
	R.Moved();
	R.Tick(0.05f, false);
	R.Hide();
	for (int I = 0; I < 5; ++I)
	{
		R.Tick(0.1f, false);
	}
	Check(!R.Visible(), "Hide sends it away at once", R.Shown);

	std::printf(Failures ? "%d failed\n" : "all passed\n", Failures);
	return Failures ? 1 : 0;
}
