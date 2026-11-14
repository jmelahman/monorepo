#include "Modules/ModuleManager.h"

#include "Containers/Ticker.h"
#include "MoviePlayer.h"
#include "Misc/Paths.h"
#include "Styling/CoreStyle.h"
#include "UObject/UObjectGlobals.h"
#include "Widgets/Images/SThrobber.h"
#include "Widgets/Layout/SBorder.h"
#include "Widgets/SBoxPanel.h"
#include "Widgets/Text/STextBlock.h"

namespace
{
// The menu's own colours: its near-black and the off-white ink.
const FLinearColor Ground = FLinearColor(FColor(0x0E, 0x11, 0x10));
const FLinearColor Ink = FLinearColor(FColor(0xF3, 0xEE, 0xE4));

FSlateFontInfo MenuFont(const TCHAR* File, float Px)
{
	// Px is the CSS size; Slate's are points at 96 DPI. The engine's Roboto if the file isn't there.
	const FString Path = FPaths::ConvertRelativePathToFull(FPaths::ProjectContentDir() / TEXT("Menu") / File);
	return FPaths::FileExists(Path) ? FSlateFontInfo(Path, Px * 0.75f) : FCoreStyle::GetDefaultFontStyle("Bold", Px * 0.75f);
}

void PrepareLoadingScreen()
{
	IGameMoviePlayer* Movies = GetMoviePlayer();
	if (!Movies || !IsMoviePlayerEnabled())
	{
		return;
	}
	FLoadingScreenAttributes Screen;
	Screen.bAutoCompleteWhenLoadingCompletes = true;
	Screen.MinimumLoadingScreenDisplayTime = 0.f;
	// clang-format off
	Screen.WidgetLoadingScreen =
		SNew(SBorder)
		.BorderImage(FCoreStyle::Get().GetBrush("WhiteBrush"))
		.BorderBackgroundColor(Ground)
		.HAlign(HAlign_Center)
		.VAlign(VAlign_Center)
		[
			SNew(SVerticalBox)
			+ SVerticalBox::Slot().AutoHeight().HAlign(HAlign_Center)
			[
				SNew(STextBlock)
				.Text(FText::FromString(TEXT("Veloverse")))
				.Font(MenuFont(TEXT("SourceSerif4-Semibold.ttf"), 88.f))
				.ColorAndOpacity(Ink)
			]
			+ SVerticalBox::Slot().AutoHeight().HAlign(HAlign_Center).Padding(0.f, 28.f, 0.f, 0.f)
			[
				SNew(SBorder)
				.BorderImage(FCoreStyle::Get().GetBrush("NoBorder"))
				.ColorAndOpacity(FLinearColor(Ink.R, Ink.G, Ink.B, 0.6f))
				[
					SNew(SThrobber)
					.NumPieces(3)
					.Animate(SThrobber::Opacity)
				]
			]
		];
	// clang-format on
	Movies->SetupLoadingScreen(Screen);
}
}

// While the world loads, at launch and on a restart: the name in the menu's serif on its dark ground
// and a quiet throbber, instead of a black window that looks hung. The engine's movie player draws it
// on its own thread, so it keeps moving while the map loads; -NoLoadingScreen turns it off.
class FVeloverseModule : public FDefaultGameModuleImpl
{
public:
	virtual void StartupModule() override
	{
		if (IsRunningDedicatedServer() || GIsEditor)
		{
			return;
		}
		PrepareLoadingScreen();
		// The movie player shows a setup once, so set up again for the next load (a restart): a
		// tick after the load, once the player has finished with this one.
		FCoreUObjectDelegates::PostLoadMapWithWorld.AddLambda([](UWorld*) {
			FTSTicker::GetCoreTicker().AddTicker(FTickerDelegate::CreateLambda([](float) {
				PrepareLoadingScreen();
				return false;
			}));
		});
	}
};

IMPLEMENT_PRIMARY_GAME_MODULE(FVeloverseModule, VeloverseUE, "VeloverseUE");
