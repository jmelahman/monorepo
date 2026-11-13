// The menu's pages, painted from the design canvas's boards in a 1920x1080 frame (wider screens
// widen it: C.W). RideMenu.cpp keeps the state they show and runs what their hits do.
#include <algorithm>

#include "GameFramework/GameUserSettings.h"
#include "HAL/PlatformProcess.h"
#include "Progress.h"
#include "RideMenu.h"
#include "RideMenuStyle.h"
#include "RidePawn.h"
#include "RidePlayerController.h"
#include "RideTrack.h"
#include "Trainer.h"

using namespace VeloverseMenu;

namespace
{
using FStops = TArray<TPair<float, FLinearColor>>;

FString Utf8(const std::string& S) { return FString(UTF8_TO_TCHAR(S.c_str())); }

// A small capital label, as the boards letter their names.
float Label(const FPainter& P, float X, float Y, const FString& Text, const FLinearColor& Colour = Muted,
            EAlign Align = EAlign::Left, float Px = 13.f)
{
	return P.Text(X, Y, Text.ToUpper(), P.Font("Bold", Px, 0.14f), Colour, Align);
}

// A number, its unit after it smaller, and its name under it.
void Stat(const FPainter& P, float X, float Y, const FString& Value, const FString& Unit, const FString& Name,
          float Px = 40.f)
{
	const float Wd = P.Text(X, Y, Value, P.Font("Bold", Px), Ink);
	if (!Unit.IsEmpty())
	{
		P.Text(X + Wd + 6.f, Y, Unit, P.Font("Medium", Px * 0.42f), Muted);
	}
	Label(P, X, Y + 28.f, Name);
}

// Values evenly across X..X+Wd, Lo at the bottom of the box and Hi at its top.
template <typename T>
TArray<FVector2f> Plot(const T& Values, int32 N, float X, float Y, float Wd, float H, double Lo, double Hi,
                       int32 From = 0, int32 To = -1)
{
	TArray<FVector2f> Out;
	To = To < 0 ? N - 1 : FMath::Min(To, N - 1);
	const double Span = FMath::Max(Hi - Lo, 1e-6);
	for (int32 I = FMath::Max(From, 0); I <= To; ++I)
	{
		const float Share = N > 1 ? static_cast<float>(I) / (N - 1) : 0.f;
		const float V = static_cast<float>((FMath::Clamp(static_cast<double>(Values[I]), Lo, Hi) - Lo) / Span);
		Out.Add(FVector2f(X + Wd * Share, Y + H * (1.f - V)));
	}
	return Out;
}

// The span of a trace's non-zero values.
void Range(const std::vector<double>& V, double& Lo, double& Hi)
{
	Lo = TNumericLimits<double>::Max();
	Hi = 0.0;
	for (const double X : V)
	{
		if (X > 0.0)
		{
			Lo = FMath::Min(Lo, X);
			Hi = FMath::Max(Hi, X);
		}
	}
	if (Hi <= 0.0)
	{
		Lo = 0.0;
	}
}

FString WorkoutLabel(const Veloverse::FWorkout& W)
{
	return FString::Printf(TEXT("%s, %.0f min, TSS %.0f"), *Utf8(W.Name), W.Seconds() / 60.0, W.Tss());
}

// The trainer as the start page and pause dialog name it, and whether power is coming from it.
FString TrainerStatus(const ARidePawn* Pawn, bool& bLive)
{
	bLive = false;
	const ITrainer* T = Pawn ? Pawn->CurrentTrainer() : nullptr;
	if (!T)
	{
		return TEXT("No trainer");
	}
	if (Pawn->TrainerSource == TEXT("keyboard"))
	{
		bLive = true;
		return TEXT("Keyboard");
	}
	for (const FSensor& S : T->Sensors())
	{
		if (S.bInUse && S.bConnected && S.Kind != FSensor::EKind::Heart)
		{
			bLive = true;
			return S.Name;
		}
	}
	return TEXT("Looking for a trainer");
}

FString Hours(double Seconds)
{
	const int64 Minutes = FMath::FloorToInt64(Seconds / 60.0);
	return FString::Printf(TEXT("%lldh %02lldm"), Minutes / 60, Minutes % 60);
}

// The world dimmed under a page.
void Shade(const FPainter& P, float W, float Alpha) { P.Rect(0.f, 0.f, W, 1080.f, WithAlpha(Night, Alpha)); }

const TCHAR* const QualityNames[5] = {TEXT("Low"), TEXT("Medium"), TEXT("High"), TEXT("Epic"), TEXT("Cinematic")};

TArray<FString> Qualities()
{
	TArray<FString> Out;
	for (const TCHAR* Q : QualityNames)
	{
		Out.Add(Q);
	}
	return Out;
}

UGameUserSettings* Video() { return UGameUserSettings::GetGameUserSettings(); }
void Apply() { Video()->ApplySettings(false); }
} // namespace

// ---- Shared --------------------------------------------------------------------------------------

void URideMenu::PaintHeader(FPaintContext& C, const FString& Title) const
{
	URideMenu* M = &C.Menu;
	C.BackButton([M] { M->GoBack(); });
	C.P.Text(140.f, Mid(74.f, 44.f), Title, C.P.Serif(44.f), Ink);
}

// ---- Start ---------------------------------------------------------------------------------------

void URideMenu::PaintStart(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	// The world stays bright on the right; the menu's side is shaded to read over it.
	FStops Across;
	Across.Emplace(0.f, WithAlpha(Night, 0.94f));
	Across.Emplace(0.3f, WithAlpha(Night, 0.78f));
	Across.Emplace(0.58f, WithAlpha(Night, 0.15f));
	Across.Emplace(0.7f, WithAlpha(Night, 0.f));
	Across.Emplace(1.f, WithAlpha(Night, 0.f));
	P.Gradient(0.f, 0.f, C.W, 1080.f, Across, true);
	FStops Down;
	Down.Emplace(0.f, WithAlpha(Night, 0.f));
	Down.Emplace(1.f, WithAlpha(Night, 0.7f));
	P.Gradient(0.f, 820.f, C.W, 260.f, Down, false);

	// Top right: the trainer and the rider's level, each a way in to its page.
	float Right = C.W - 64.f;
	if (Pawn)
	{
		const Veloverse::FLevel Level = Veloverse::LevelFor(Pawn->Profile.Xp);
		const FString Text = FString::Printf(TEXT("Level %d"), Level.Level);
		const float Wd = P.Width(Text, P.Font("Bold", 16.f)) + 40.f + 96.f;
		Right -= Wd;
		P.Round(Right, 48.f, Wd, 44.f, 22.f, WithAlpha(Night, C.Hot(TEXT("level")) ? 0.8f : 0.55f));
		P.Border(Right, 48.f, Wd, 44.f, 22.f, InkA(0.16f));
		const float Tw = P.Text(Right + 20.f, Mid(70.f, 16.f), Text, P.Font("Bold", 16.f), Ink);
		P.Round(Right + 32.f + Tw, 67.f, 80.f, 6.f, 3.f, InkA(0.16f));
		P.Round(Right + 32.f + Tw, 67.f, FMath::Max(80.f * static_cast<float>(Level.Fraction()), 6.f), 6.f, 3.f,
		        C.Accent);
		C.Ring(TEXT("level"), Right, 48.f, Wd, 44.f, 22.f);
		C.Hit(TEXT("level"), Right, 48.f, Wd, 44.f).OnClick = [M]
		{
			M->Tab = ERideSettingsTab::Rider;
			M->Show(ERideMenuPage::Settings);
		};
		Right -= 12.f;
	}
	{
		bool bLive = false;
		const FString Status = TrainerStatus(Pawn, bLive);
		const float Wd = P.Width(Status, P.Font("Bold", 16.f)) + 76.f;
		Right -= Wd;
		P.Round(Right, 48.f, Wd, 44.f, 22.f, WithAlpha(Night, C.Hot(TEXT("trainer")) ? 0.8f : 0.55f));
		P.Border(Right, 48.f, Wd, 44.f, 22.f, InkA(0.16f));
		P.Icon(Icons::Bluetooth, Right + 16.f, 60.f, 20.f, bLive ? Ok : Muted);
		P.Text(Right + 44.f, Mid(70.f, 16.f), Status, P.Font("Bold", 16.f), Ink);
		C.Ring(TEXT("trainer"), Right, 48.f, Wd, 44.f, 22.f);
		C.Hit(TEXT("trainer"), Right, 48.f, Wd, 44.f).OnClick = [M] { M->Show(ERideMenuPage::Sensors); };
	}

	// The title.
	P.Icon(Icons::Rider, 120.f, 104.f, 24.f, C.Accent);
	P.Text(156.f, Mid(116.f, 15.f), TEXT("RIDE THE ISLES"), P.Font("Bold", 15.f, 0.24f), C.Accent);
	P.Text(114.f, 238.f, TEXT("Veloverse"), P.Serif(104.f), Ink);
	P.Text(120.f, 286.f, FString::Printf(TEXT("%d routes across five islands, from the harbour"), Surveys.Num()),
	       P.Font("Medium", 20.f), Soft);

	// Ride: the route as it's set, from where it starts.
	const FRouteSurvey* Route = Pawn ? SurveyOf(Pawn->Track) : nullptr;
	{
		const FString Key = TEXT("ride");
		const bool bHot = C.Hot(Key);
		P.Round(120.f, 332.f, 560.f, 84.f, 14.f, bHot ? C.Accent * 1.12f : C.Accent);
		P.Text(150.f, 370.f, TEXT("Ride"), P.Serif(30.f), C.OnAccent);
		if (Route)
		{
			P.Text(150.f, 398.f,
			       P.Fit(FString::Printf(TEXT("Continue: %s · %.1f km"), *Route->Title, Route->Km),
			             P.Font("Medium", 16.f), 430.f),
			       P.Font("Medium", 16.f), WithAlpha(C.OnAccent, 0.8f));
		}
		P.Icon(Icons::Arrow, 618.f, 358.f, 32.f, C.OnAccent, 2.4f);
		C.Ring(Key, 120.f, 332.f, 560.f, 84.f, 14.f);
		C.Hit(Key, 120.f, 332.f, 560.f, 84.f).OnClick = [M] { M->Controller()->Ride(); };
	}

	// The list under it.
	FString WorkoutText = TEXT("None, just ride");
	if (Pawn && Pawn->Workout.Active())
	{
		const FString Name = Utf8(Pawn->Workout.Workout().Name);
		WorkoutText = Name;
		for (const Veloverse::FWorkout& W : Workouts)
		{
			if (Utf8(W.Name) == Name)
			{
				WorkoutText = WorkoutLabel(W);
			}
		}
	}
	bool bLive = false;
	struct FItem
	{
		const TCHAR* Key;
		const TCHAR* Icon;
		FString Label, Value;
		TFunction<void()> Click;
		bool bCycle = false;
	};
	const TArray<FItem> Items = {
		{TEXT("routes"), Icons::Map, TEXT("Choose a route"), Route ? Route->Title : FString(),
		 [M] { M->Show(ERideMenuPage::Routes); }},
		{TEXT("stats"), Icons::Stats, TEXT("Stats"), FString::Printf(TEXT("%.0f km this week"), Totals[0].Km),
		 [M] { M->Show(ERideMenuPage::Stats); }},
		{TEXT("workout"), Icons::Workout, TEXT("Workout"), WorkoutText, [M] { M->StepWorkout(1); }, true},
		{TEXT("sensors"), Icons::Signal, TEXT("Trainer and sensors"), TrainerStatus(Pawn, bLive),
		 [M] { M->Show(ERideMenuPage::Sensors); }},
		{TEXT("settings"), Icons::Gear, TEXT("Settings"), FString(), [M] { M->Show(ERideMenuPage::Settings); }},
		{TEXT("quit"), Icons::Leave, TEXT("Quit"), FString(), [M] { M->Controller()->Quit(); }},
	};
	float Y = 440.f;
	for (const FItem& Item : Items)
	{
		const bool bHot = C.Hot(Item.Key);
		if (bHot)
		{
			P.Round(120.f, Y, 560.f, 64.f, 12.f, InkA(0.08f));
		}
		P.Icon(Item.Icon, 140.f, Y + 21.f, 22.f, bHot ? Ink : Soft);
		P.Text(180.f, Mid(Y + 32.f, 20.f), Item.Label, P.Font("Bold", 20.f), Ink);
		const float ValueRight = Item.bCycle ? 604.f : 656.f;
		if (!Item.Value.IsEmpty())
		{
			const FSlateFontInfo F = P.Font("Medium", 16.f);
			P.Text(ValueRight, Mid(Y + 32.f, 16.f), P.Fit(Item.Value, F, 250.f), F, Muted, EAlign::Right);
		}
		if (Item.bCycle)
		{
			P.Icon(Icons::ChevronLeft, 620.f, Y + 22.f, 20.f, Muted);
			P.Icon(Icons::Chevron, 646.f, Y + 22.f, 20.f, Muted);
		}
		C.Ring(Item.Key, 120.f, Y, 560.f, 64.f, 12.f);
		FHit& H = C.Hit(Item.Key, 120.f, Y, 560.f, 64.f);
		H.OnClick = Item.Click;
		if (Item.bCycle)
		{
			H.OnLeft = [M] { M->StepWorkout(-1); };
			H.OnRight = [M] { M->StepWorkout(1); };
		}
		Y += 70.f;
	}

	// The last ride, to look back at.
	if (History.Num() > 0)
	{
		const FRideEntry& E = History[0];
		const float X = C.W - 64.f - 420.f, Top = 1080.f - 64.f - 180.f;
		const FString Key = TEXT("last");
		C.Panel(X, Top, 420.f, 180.f, 16.f, C.Hot(Key) ? 0.9f : 0.72f);
		Label(P, X + 28.f, Top + 40.f, TEXT("Last ride · ") + RideDay(E.Started));
		P.Text(X + 28.f, Top + 84.f, P.Fit(E.Route.IsEmpty() ? FString(TEXT("Ride")) : E.Route, P.Serif(28.f), 364.f),
		       P.Serif(28.f), Ink);
		P.Text(X + 28.f, Top + 120.f,
		       FString::Printf(TEXT("%.1f km · %s · %s"), E.Summary.Metres / 1000.0, *Clock(E.Summary.Seconds, false),
		                       *Watts(E.Summary.AverageWatts)),
		       P.Font("Medium", 17.f), Soft);
		P.Text(X + 28.f, Top + 156.f, TEXT("Review the ride"), P.Font("Bold", 16.f), C.Accent);
		P.Icon(Icons::Arrow, X + 28.f + P.Width(TEXT("Review the ride"), P.Font("Bold", 16.f)) + 8.f, Top + 141.f, 20.f,
		       C.Accent);
		C.Ring(Key, X, Top, 420.f, 180.f, 16.f);
		C.Hit(Key, X, Top, 420.f, 180.f).OnClick = [M]
		{
			M->bFromStats = false;
			M->ReviewRide(0);
			M->Show(ERideMenuPage::Summary);
		};
	}
	P.Text(120.f, 1040.f, TEXT("Veloverse ") + Version, P.Font("Medium", 14.f), WithAlpha(Muted, 0.7f));
}

// ---- Choosing a route ----------------------------------------------------------------------------

void URideMenu::PaintRoutes(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.62f);
	PaintHeader(C, TEXT("Choose a route"));
	P.Text(140.f + P.Width(TEXT("Choose a route"), P.Serif(44.f)) + 20.f, Mid(76.f, 17.f),
	       TEXT("Every loop starts and ends at the harbour"), P.Font("Medium", 17.f), Muted);
	if (!Pawn)
	{
		return;
	}
	const int32 Chosen = Pawn->RouteIndex();

	// The list.
	float Y = 140.f;
	for (int32 I = 0; I < Pawn->Routes.Num() && Y + 88.f <= 1040.f; ++I)
	{
		const FRouteSurvey* S = SurveyOf(Pawn->Routes[I]);
		if (!S)
		{
			continue;
		}
		const FString Key = FString::Printf(TEXT("route%d"), I);
		const bool bOn = I == Chosen;
		const bool bHot = C.Hot(Key);
		P.Round(64.f, Y, 600.f, 88.f, 14.f,
		        bOn ? WithAlpha(C.Accent, 0.12f) : WithAlpha(Night, bHot ? 0.75f : 0.55f));
		P.Border(64.f, Y, 600.f, 88.f, 14.f, bOn ? WithAlpha(C.Accent, 0.7f) : InkA(bHot ? 0.24f : 0.1f));
		P.Text(88.f, Y + 38.f, P.Fit(S->Title, P.Font("Bold", 22.f), 340.f), P.Font("Bold", 22.f), Ink);
		P.Text(88.f, Y + 66.f,
		       FString::Printf(TEXT("%.1f km · %s m climbing%s"), S->Km, *Thousands(S->Climb),
		                       S->bClosed ? TEXT("") : TEXT(" · one way")),
		       P.Font("Medium", 16.f), Muted);
		if (S->Heights.Num() > 1)
		{
			float Lo = TNumericLimits<float>::Max(), Hi = 0.f;
			for (const float H : S->Heights)
			{
				Lo = FMath::Min(Lo, H);
				Hi = FMath::Max(Hi, H);
			}
			const TArray<FVector2f> Line =
				Plot(S->Heights, S->Heights.Num(), 470.f, Y + 22.f, 170.f, 44.f, Lo, FMath::Max(Hi, Lo + 20.f));
			P.Area(Line, Y + 66.f, bOn ? WithAlpha(C.Accent, 0.22f) : InkA(0.08f));
			P.Lines(Line, bOn ? C.Accent : InkA(0.5f), 1.6f);
		}
		C.Ring(Key, 64.f, Y, 600.f, 88.f, 14.f);
		C.Hit(Key, 64.f, Y, 600.f, 88.f).OnClick = [M, I]
		{
			if (ARidePawn* R = M->Rider(); R && !R->bStarted)
			{
				R->UseRoute(I);
			}
		};
		Y += 96.f;
	}

	const FRouteSurvey* S = Pawn->Routes.IsValidIndex(Chosen) ? SurveyOf(Pawn->Routes[Chosen]) : nullptr;
	if (!S)
	{
		return;
	}
	const float RightEdge = C.W - 64.f;

	// The route's islands, as shares of it.
	TArray<TPair<FString, float>> Shares;
	for (int32 I = 0; I < S->Islands.Num(); ++I)
	{
		const float End = I + 1 < S->Islands.Num() ? S->Islands[I + 1].Key : 1.f;
		const FString& Name = S->Islands[I].Value;
		TPair<FString, float>* Found = Shares.FindByPredicate([&](const TPair<FString, float>& X) { return X.Key == Name; });
		if (Found)
		{
			Found->Value += End - S->Islands[I].Key;
		}
		else
		{
			Shares.Emplace(Name, End - S->Islands[I].Key);
		}
	}
	TArray<FString> Inland;
	for (const TPair<FString, float>& Share : Shares)
	{
		if (Share.Key != TEXT("The Coast"))
		{
			Inland.Add(Share.Key);
		}
	}
	const FString Kicker = Inland.Num() > 0 ? FString::Join(Inland, TEXT(" · ")) : FString(TEXT("The Coast"));
	P.Text(720.f, 170.f, Kicker.ToUpper(), P.Font("Bold", 15.f, 0.24f), C.Accent);
	P.Text(716.f, 236.f, P.Fit(S->Title, P.Serif(64.f), RightEdge - 720.f), P.Serif(64.f), Ink);
	P.Wrap(720.f, 280.f, FMath::Min(1100.f, RightEdge - 720.f), 26.f, S->Blurb, P.Font("Medium", 19.f), Soft, 1);

	// The map: every route faint, this one in its islands' colours.
	const float MapX = 720.f, MapY = 320.f;
	const float K = FMath::Min(567.f / FMath::Max(MapWidth, 1.f), 520.f / FMath::Max(MapHeight, 1.f));
	const float Mw = MapWidth * K, Mh = MapHeight * K;
	if (MapTexture)
	{
		MapBrush.OutlineSettings = FSlateBrushOutlineSettings(16.f * P.S);
		MapBrush.OutlineSettings.RoundingType = ESlateBrushRoundingType::FixedRadius;
		P.Image(MapX, MapY, Mw, Mh, &MapBrush);
		P.Border(MapX, MapY, Mw, Mh, 16.f, InkA(0.14f));
	}
	else
	{
		C.Panel(MapX, MapY, 567.f, 520.f);
		P.Text(MapX + 283.f, MapY + 266.f, TEXT("No map yet"), P.Font("Medium", 16.f), Muted, EAlign::Centre);
	}
	auto ToMap = [&](const TArray<FVector2f>& Points)
	{
		TArray<FVector2f> Out;
		for (const FVector2f& Pt : Points)
		{
			Out.Add(FVector2f(MapX + Pt.X * K, MapY + Pt.Y * K));
		}
		return Out;
	};
	if (MapTexture && MapScale > 0.f)
	{
		for (const FRouteSurvey& Other : Surveys)
		{
			if (&Other != S)
			{
				P.Lines(ToMap(Other.Map), InkA(0.22f), 1.4f);
			}
		}
		const TArray<FVector2f> Road = ToMap(S->Map);
		P.Lines(Road, WithAlpha(Night, 0.85f), 6.9f);
		// The road a stretch at a time, each in its island's colour.
		int32 Island = 0;
		TArray<FVector2f> Run;
		for (int32 I = 0; I < Road.Num(); ++I)
		{
			const float Share = Road.Num() > 1 ? static_cast<float>(I) / (Road.Num() - 1) : 0.f;
			Run.Add(Road[I]);
			if (Island + 1 < S->Islands.Num() && S->Islands[Island + 1].Key <= Share)
			{
				P.Lines(Run, IslandColour(S->Islands[Island].Value), 3.7f);
				Run = {Road[I]};
				++Island;
			}
		}
		if (S->Islands.IsValidIndex(Island))
		{
			P.Lines(Run, IslandColour(S->Islands[Island].Value), 3.7f);
		}
		else
		{
			P.Lines(Run, C.Accent, 3.7f);
		}
		for (const TPair<FString, FVector2f>& L : MapLabels)
		{
			P.Text(MapX + L.Value.X * K, MapY + L.Value.Y * K, L.Key.ToUpper(), P.Font("Bold", 13.8f, 0.18f), InkA(0.72f),
			       EAlign::Centre);
		}
		// Where it starts.
		const float Length = Pawn->Track ? Pawn->Track->Length() : 0.f;
		if (Road.Num() > 0 && Length > 0.f)
		{
			const float Share = FMath::Clamp(Pawn->StartMetres * 100.f / Length, 0.f, 1.f);
			const FVector2f At = Road[FMath::RoundToInt32(Share * (Road.Num() - 1))];
			P.Disc(At.X, At.Y, 10.f, Ink);
			P.Disc(At.X, At.Y, 6.5f, C.Accent);
		}
		// A scale.
		float Km = 5.f;
		float Bar = Km * 100000.f * MapScale * K;
		if (Bar > 200.f)
		{
			Km = 1.f;
			Bar /= 5.f;
		}
		const float Bx = MapX + 24.f, By = MapY + Mh - 28.f;
		P.Lines({FVector2f(Bx, By - 6.f), FVector2f(Bx, By), FVector2f(Bx + Bar, By), FVector2f(Bx + Bar, By - 6.f)}, Ink,
		        1.6f);
		P.Text(Bx + Bar + 10.f, Mid(By - 3.f, 13.f), FString::Printf(TEXT("%.0f km"), Km), P.Font("Bold", 13.f), Ink);
	}

	// Beside the map: the numbers, the islands, where to start and the way in.
	const float Dx = 1327.f, Dw = FMath::Max(RightEdge - Dx, 300.f);
	Stat(P, Dx, 372.f, FString::Printf(TEXT("%.1f"), S->Km), TEXT("km"), TEXT("Distance"), 44.f);
	Stat(P, Dx + Dw / 3.f, 372.f, Thousands(S->Climb), TEXT("m"), TEXT("Climbing"), 44.f);
	Stat(P, Dx + 2.f * Dw / 3.f, 372.f, Thousands(S->Highest), TEXT("m"), TEXT("Highest"), 44.f);
	Label(P, Dx, 452.f, TEXT("Islands"));
	float Bx = Dx;
	for (const TPair<FString, float>& Share : Shares)
	{
		const float Wd = Dw * Share.Value;
		P.Rect(Bx, 466.f, FMath::Max(Wd - 2.f, 1.f), 12.f, IslandColour(Share.Key));
		Bx += Wd;
	}
	for (int32 I = 0; I < Shares.Num(); ++I)
	{
		const float Lx = Dx + (I % 2) * Dw * 0.5f, Ly = 512.f + (I / 2) * 30.f;
		P.Disc(Lx + 6.f, Ly - 5.f, 6.f, IslandColour(Shares[I].Key));
		const float Nw = P.Text(Lx + 20.f, Ly, Shares[I].Key, P.Font("Medium", 16.f), Soft);
		P.Text(Lx + 26.f + Nw, Ly, FString::Printf(TEXT("%.0f%%"), Shares[I].Value * 100.f), P.Font("Bold", 16.f),
		       Muted);
	}

	const TArray<ARidePawn::FStart> Starts = Pawn->Starts();
	const int32 StartAt = Pawn->StartIndex();
	if (Starts.Num() > 0)
	{
		Label(P, Dx, 628.f, TEXT("Start at"));
		const FString Key = TEXT("startat");
		const bool bHot = C.Hot(Key);
		P.Round(Dx, 640.f, Dw, 56.f, 12.f, InkA(bHot ? 0.1f : 0.06f));
		P.Icon(Icons::ChevronLeft, Dx + 14.f, 656.f, 24.f, Ink);
		P.Icon(Icons::Chevron, Dx + Dw - 38.f, 656.f, 24.f, Ink);
		const ARidePawn::FStart& Start = Starts[FMath::Clamp(StartAt, 0, Starts.Num() - 1)];
		P.Text(Dx + Dw * 0.5f, Mid(668.f, 18.f),
		       P.Fit(FString::Printf(TEXT("%s · %.1f km"), *Start.Name, Start.Metres / 1000.f), P.Font("Bold", 18.f),
		             Dw - 100.f),
		       P.Font("Bold", 18.f), Ink, EAlign::Centre);
		auto Step = [M](int32 By)
		{
			ARidePawn* R = M->Rider();
			if (R && !R->bStarted && R->Starts().Num() > 0)
			{
				const int32 N = R->Starts().Num();
				R->UseStart(((R->StartIndex() + By) % N + N) % N);
			}
		};
		C.Ring(Key, Dx, 640.f, Dw, 56.f, 12.f);
		FHit& H = C.Hit(Key, Dx, 640.f, Dw, 56.f);
		H.OnClick = [Step] { Step(1); };
		H.OnLeft = [Step] { Step(-1); };
		H.OnRight = [Step] { Step(1); };
		C.Hit(TEXT("startprev"), Dx, 640.f, 56.f, 56.f).OnClick = [Step] { Step(-1); };
	}
	C.Button(TEXT("start"), Dx, 718.f, Dw, 72.f, TEXT("Start ride"), EButton::Primary,
	         [M] { M->Controller()->Ride(); }, nullptr, 28.f, true, 14.f);
	C.Button(TEXT("check"), Dx, 802.f, Dw, 44.f, TEXT("Check trainer"), EButton::Quiet,
	         [M] { M->Show(ERideMenuPage::Sensors); }, Icons::Signal, 16.f);

	// The climb, along the route, under it in the islands' colours.
	const float Ex = 796.f, Ew = RightEdge - Ex, Ey = 868.f, Eh = 134.f;
	if (S->Heights.Num() > 1)
	{
		float Lo = TNumericLimits<float>::Max(), Hi = 0.f;
		for (const float H : S->Heights)
		{
			Lo = FMath::Min(Lo, H);
			Hi = FMath::Max(Hi, H);
		}
		Lo = FMath::FloorToFloat(Lo / 50.f) * 50.f;
		Hi = FMath::Max(FMath::CeilToFloat(Hi / 50.f) * 50.f, Lo + 50.f);
		const int32 N = S->Heights.Num();
		for (int32 I = 0; I < S->Islands.Num(); ++I)
		{
			const float From = S->Islands[I].Key, To = I + 1 < S->Islands.Num() ? S->Islands[I + 1].Key : 1.f;
			const TArray<FVector2f> Span = Plot(S->Heights, N, Ex, Ey, Ew, Eh, Lo, Hi,
			                                    FMath::FloorToInt32(From * (N - 1)), FMath::CeilToInt32(To * (N - 1)));
			P.Area(Span, Ey + Eh, WithAlpha(IslandColour(S->Islands[I].Value), 0.38f));
		}
		P.Lines(Plot(S->Heights, N, Ex, Ey, Ew, Eh, Lo, Hi), Ink, 2.f);
		P.Lines({FVector2f(Ex, Ey + Eh), FVector2f(Ex + Ew, Ey + Eh)}, InkA(0.2f), 1.f);
		P.Text(Ex - 14.f, Mid(Ey, 13.f), FString::Printf(TEXT("%.0f m"), Hi), P.Font("Medium", 13.f), Muted,
		       EAlign::Right);
		P.Text(Ex - 14.f, Mid(Ey + Eh, 13.f), FString::Printf(TEXT("%.0f m"), Lo), P.Font("Medium", 13.f), Muted,
		       EAlign::Right);
		for (int32 Q = 0; Q <= 4; ++Q)
		{
			P.Text(Ex + Ew * Q / 4.f, Ey + Eh + 28.f, FString::Printf(TEXT("%.0f km"), S->Km * Q / 4.f),
			       P.Font("Medium", 13.f), Muted, Q == 0 ? EAlign::Left : Q == 4 ? EAlign::Right : EAlign::Centre);
		}
		const float Length = Pawn->Track ? Pawn->Track->Length() : 0.f;
		if (Length > 0.f)
		{
			const float Sx = Ex + Ew * FMath::Clamp(Pawn->StartMetres * 100.f / Length, 0.f, 1.f);
			P.Dashed({FVector2f(Sx, Ey - 8.f), FVector2f(Sx, Ey + Eh)}, C.Accent, 1.6f, 4.f, 4.f);
			P.Disc(Sx, Ey - 8.f, 5.f, C.Accent);
		}
	}
	Label(P, 720.f, Ey - 16.f, TEXT("Elevation"));
}

// ---- Trainer and sensors -------------------------------------------------------------------------

void URideMenu::PaintSensors(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.6f);
	PaintHeader(C, TEXT("Trainer and sensors"));
	const ITrainer* T = Pawn ? Pawn->CurrentTrainer() : nullptr;

#if !UE_BUILD_SHIPPING
	// Where power comes from, for testing without a trainer: only Bluetooth ships.
	if (Pawn)
	{
		const TArray<FString> Options = {TEXT("Keyboard"), TEXT("Simulated"), TEXT("Bluetooth")};
		const float Wd = C.SegmentedWidth(Options, 15.f, 96.f);
		Label(P, C.W - 64.f - Wd, 40.f, TEXT("Source (development builds)"));
		C.Segmented(TEXT("source"), C.W - 64.f - Wd, 50.f, Options,
		            ARidePawn::TrainerSources.IndexOfByKey(Pawn->TrainerSource),
		            [M](int32 I)
		            {
			            if (ARidePawn* R = M->Rider(); R && ARidePawn::TrainerSources.IsValidIndex(I))
			            {
				            R->UseTrainer(ARidePawn::TrainerSources[I]);
			            }
		            },
		            40.f, 15.f, 96.f);
	}
#endif

	const float Wd = 880.f, X = (C.W - Wd) * 0.5f;
	float Y = 200.f;
	Label(P, X, Y, TEXT("Bluetooth devices"));
	P.Icon(Icons::Bluetooth, X + Wd - 22.f, Y - 16.f, 20.f, Sky);
	Y += 20.f;
	const TArray<FSensor> Sensors = T ? T->Sensors() : TArray<FSensor>();
	if (Pawn && Pawn->TrainerSource == TEXT("keyboard"))
	{
		C.Panel(X, Y, Wd, 120.f, 14.f);
		P.Icon(Icons::Trainer, X + 28.f, Y + 36.f, 44.f, Soft);
		P.Text(X + 96.f, Y + 54.f, TEXT("Riding on the keyboard"), P.Font("Bold", 20.f), Ink);
		P.Text(X + 96.f, Y + 84.f, TEXT("Hold W or Up to push harder, S or Down to ease off."), P.Font("Medium", 17.f),
		       Muted);
		Y += 136.f;
	}
	else if (Sensors.Num() == 0)
	{
		C.Panel(X, Y, Wd, 120.f, 14.f);
		const float Pulse = 0.5f + 0.5f * FMath::Sin(static_cast<float>(C.Time) * 3.f);
		P.Icon(Icons::Bluetooth, X + 32.f, Y + 38.f, 40.f, WithAlpha(Sky, 0.4f + 0.6f * Pulse));
		P.Text(X + 96.f, Y + 54.f, TEXT("Looking for devices"), P.Font("Bold", 20.f), Ink);
		P.Text(X + 96.f, Y + 84.f, TEXT("Wake your trainer by pedalling, and put your heart-rate strap on."),
		       P.Font("Medium", 17.f), Muted);
		Y += 136.f;
	}
	for (int32 I = 0; I < Sensors.Num() && Y < 640.f; ++I)
	{
		const FSensor& S = Sensors[I];
		const TCHAR* Icon = S.Kind == FSensor::EKind::Heart ? Icons::HeartIcon
			: S.Kind == FSensor::EKind::Power             ? Icons::Bolt
			                                              : Icons::Trainer;
		const TCHAR* Kind = S.Kind == FSensor::EKind::Heart ? TEXT("Heart-rate strap")
			: S.Kind == FSensor::EKind::Power             ? TEXT("Power meter")
			                                              : TEXT("Smart trainer");
		C.Panel(X, Y, Wd, 76.f, 14.f, S.bInUse ? 0.9f : 0.7f);
		if (S.bInUse)
		{
			P.Border(X, Y, Wd, 76.f, 14.f, WithAlpha(C.Accent, 0.5f));
		}
		P.Icon(Icon, X + 24.f, Y + 24.f, 28.f, S.Kind == FSensor::EKind::Heart ? Heart : Soft);
		P.Text(X + 72.f, Y + 34.f, P.Fit(S.Name, P.Font("Bold", 19.f), Wd - 300.f), P.Font("Bold", 19.f), Ink);
		const FString Status = S.bConnected ? TEXT("Connected") : S.bInUse ? TEXT("Connecting") : TEXT("Found");
		const float Kw = P.Text(X + 72.f, Y + 58.f, Kind, P.Font("Medium", 15.f), Muted);
		P.Disc(X + 84.f + Kw, Y + 53.f, 4.f, S.bConnected ? Ok : Muted);
		P.Text(X + 94.f + Kw, Y + 58.f, Status, P.Font("Medium", 15.f), S.bConnected ? Ok : Muted);
		const FString Key = FString::Printf(TEXT("use%d"), I);
		if (S.bInUse)
		{
			P.Icon(Icons::Check, X + Wd - 130.f, Y + 27.f, 22.f, Ok);
			P.Text(X + Wd - 24.f, Mid(Y + 38.f, 16.f), TEXT("In use"), P.Font("Bold", 16.f), Ok, EAlign::Right);
		}
		else
		{
			const FString Name = S.Name;
			const bool bStrap = S.Kind == FSensor::EKind::Heart;
			C.Button(Key, X + Wd - 124.f, Y + 16.f, 104.f, 44.f, TEXT("Use"), EButton::Outline,
			         [M, Name, bStrap]
			         {
				         ARidePawn* R = M->Rider();
				         if (ITrainer* Tr = R ? R->CurrentTrainer() : nullptr)
				         {
					         bStrap ? Tr->PickStrap(Name) : Tr->Pick(Name);
				         }
			         },
			         nullptr, 16.f);
		}
		Y += 84.f;
	}

	// What's coming in now.
	Y += 32.f;
	Label(P, X, Y, TEXT("Live"));
	Y += 16.f;
	const FTrainerReading Reading = Pawn ? Pawn->Reading : FTrainerReading();
	struct FCard
	{
		const TCHAR* Icon;
		const TCHAR* Name;
		double Value;
		const TCHAR* Unit;
		FLinearColor Colour;
	};
	const FCard Cards[3] = {{Icons::Bolt, TEXT("Power"), Reading.Watts, TEXT("W"), C.Accent},
	                        {Icons::Cadence, TEXT("Cadence"), Reading.CadenceRpm, TEXT("rpm"), Sky},
	                        {Icons::HeartIcon, TEXT("Heart rate"), Reading.HeartRateBpm, TEXT("bpm"), Heart}};
	const float Cw = (Wd - 32.f) / 3.f;
	for (int32 I = 0; I < 3; ++I)
	{
		const float Cx = X + I * (Cw + 16.f);
		C.Panel(Cx, Y, Cw, 150.f, 14.f);
		P.Icon(Cards[I].Icon, Cx + 24.f, Y + 22.f, 22.f, Cards[I].Colour);
		Label(P, Cx + 56.f, Y + 39.f, Cards[I].Name);
		const FString Value = Cards[I].Value > 0.0 ? FString::Printf(TEXT("%.0f"), Cards[I].Value) : FString(TEXT("–"));
		const float Vw = P.Text(Cx + 24.f, Y + 116.f, Value, P.Font("Bold", 52.f), Ink);
		P.Text(Cx + 32.f + Vw, Y + 116.f, Cards[I].Unit, P.Font("Medium", 20.f), Muted);
	}
	Y += 190.f;
	P.Wrap(X, Y, Wd - 300.f, 24.f,
	       TEXT("A smart trainer sets the resistance to the road's grade. Wake it by pedalling; it connects on its own "
	            "next time."),
	       P.Font("Medium", 16.f), Muted, 2);
	if (Pawn && !Pawn->bStarted)
	{
		C.Button(TEXT("routes"), X + Wd - 260.f, Y - 30.f, 260.f, 60.f, TEXT("Choose a route"), EButton::Primary,
		         [M] { M->Show(ERideMenuPage::Routes); }, Icons::Arrow, 19.f);
	}
	else
	{
		C.Button(TEXT("routes"), X + Wd - 260.f, Y - 30.f, 260.f, 60.f, TEXT("Back"), EButton::Primary,
		         [M] { M->GoBack(); }, nullptr, 19.f);
	}
}

// ---- Settings ------------------------------------------------------------------------------------

void URideMenu::PaintSettings(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.6f);
	PaintHeader(C, TEXT("Settings"));

	// The tabs.
	C.Panel(64.f, 150.f, 320.f, 866.f);
	const TCHAR* Names[3] = {TEXT("Graphics"), TEXT("Detail"), TEXT("Rider")};
	const TCHAR* TabIcons[3] = {Icons::Screen, Icons::Layers, Icons::Rider};
	const FLinearColor TabInk = FMath::Lerp(C.Accent, FLinearColor::White, 0.45f);
	for (int32 I = 0; I < 3; ++I)
	{
		const FString Key = FString::Printf(TEXT("tab%d"), I);
		const float Y = 166.f + I * 64.f;
		const bool bOn = static_cast<int32>(Tab) == I;
		if (bOn || C.Hot(Key))
		{
			P.Round(80.f, Y, 288.f, 60.f, 12.f, bOn ? WithAlpha(C.Accent, 0.14f) : InkA(0.07f));
		}
		P.Icon(TabIcons[I], 100.f, Y + 19.f, 22.f, bOn ? TabInk : Soft);
		P.Text(140.f, Mid(Y + 30.f, 19.f), Names[I], P.Font("Bold", 19.f), bOn ? TabInk : Ink);
		C.Ring(Key, 80.f, Y, 288.f, 60.f, 12.f);
		FHit& H = C.Hit(Key, 80.f, Y, 288.f, 60.f);
		H.OnClick = [M, I] { M->SetTab(static_cast<ERideSettingsTab>(I)); };
	}

	// The section.
	const float Sx = 416.f, Sw = C.W - 64.f - Sx;
	C.Panel(Sx, 150.f, Sw, 866.f);
	const float Left = Sx + 48.f, Right = Sx + Sw - 48.f;
	const FString Titles[3] = {TEXT("Graphics"), TEXT("Detail"), TEXT("Rider")};
	P.Text(Left, 222.f, Titles[static_cast<int32>(Tab)], P.Serif(36.f), Ink);
	float Y = 250.f;
	auto Row = [&](const FString& Name, const FString& Hint)
	{
		const float Cy = Y + 46.f;
		P.Text(Left, Cy - 4.f, Name, P.Font("Bold", 20.f), Ink);
		P.Text(Left, Cy + 22.f, P.Fit(Hint, P.Font("Medium", 15.f), Sw * 0.4f), P.Font("Medium", 15.f), Muted);
		P.Rect(Left, Y + 92.f, Right - Left, 1.f, InkA(0.08f));
		Y += 92.f;
		return Cy;
	};
	auto Choice = [&](const FString& Key, float Cy, const TArray<FString>& Options, int32 Chosen,
	                  TFunction<void(int32)> Pick)
	{
		const float Wd = C.SegmentedWidth(Options, 15.f, 96.f);
		C.Segmented(Key, Right - Wd, Cy - 26.f, Options, Chosen, MoveTemp(Pick), 42.f, 15.f, 96.f);
	};
	UGameUserSettings* V = Video();

	if (Tab == ERideSettingsTab::Graphics && V)
	{
		const int32 Overall = V->GetOverallScalabilityLevel();
		Choice(TEXT("overall"), Row(TEXT("Quality"), Overall < 0 ? TEXT("Custom: set in Detail") : TEXT("Every detail at once")),
		       Qualities(), Overall,
		       [](int32 I)
		       {
			       Video()->SetOverallScalabilityLevel(I);
			       Apply();
		       });
		static const int32 Scales[5] = {50, 67, 75, 85, 100};
		float Normalized = 1.f, Percent = 100.f, Min = 0.f, Max = 100.f;
		V->GetResolutionScaleInformationEx(Normalized, Percent, Min, Max);
		int32 ScaleAt = 0;
		TArray<FString> ScaleNames;
		for (int32 I = 0; I < 5; ++I)
		{
			ScaleNames.Add(FString::Printf(TEXT("%d%%"), Scales[I]));
			if (FMath::Abs(Percent - Scales[I]) < FMath::Abs(Percent - Scales[ScaleAt]))
			{
				ScaleAt = I;
			}
		}
		Choice(TEXT("scale"), Row(TEXT("Resolution"), TEXT("Fewer pixels, scaled up: faster and softer")), ScaleNames,
		       ScaleAt,
		       [](int32 I)
		       {
			       Video()->SetResolutionScaleValueEx(Scales[I]);
			       Apply();
		       });
		static const float Caps[5] = {30.f, 60.f, 90.f, 120.f, 0.f};
		const float Cap = V->GetFrameRateLimit();
		int32 CapAt = 4;
		for (int32 I = 0; I < 5; ++I)
		{
			CapAt = FMath::IsNearlyEqual(Cap, Caps[I], 0.5f) ? I : CapAt;
		}
		Choice(TEXT("cap"), Row(TEXT("Frame rate cap"), TEXT("Holds the frame rate down to keep it steady")),
		       {TEXT("30"), TEXT("60"), TEXT("90"), TEXT("120"), TEXT("None")}, CapAt,
		       [](int32 I)
		       {
			       Video()->SetFrameRateLimit(Caps[I]);
			       Apply();
		       });
		Choice(TEXT("vsync"), Row(TEXT("VSync"), TEXT("No tearing, a little more latency")), {TEXT("Off"), TEXT("On")},
		       V->IsVSyncEnabled() ? 1 : 0,
		       [](int32 I)
		       {
			       Video()->SetVSyncEnabled(I == 1);
			       Apply();
		       });
		static const EWindowMode::Type Modes[3] = {EWindowMode::Fullscreen, EWindowMode::WindowedFullscreen,
		                                           EWindowMode::Windowed};
		const int32 ModeAt = FMath::Max(0, static_cast<int32>(
			std::find(std::begin(Modes), std::end(Modes), V->GetFullscreenMode()) - std::begin(Modes)) % 3);
		Choice(TEXT("window"), Row(TEXT("Display"), TEXT("Borderless switches away quickest")),
		       {TEXT("Fullscreen"), TEXT("Borderless"), TEXT("Windowed")}, ModeAt,
		       [](int32 I)
		       {
			       Video()->SetFullscreenMode(Modes[I]);
			       Apply();
		       });
	}
	else if (Tab == ERideSettingsTab::Detail && V)
	{
		struct FGroup
		{
			const TCHAR* Key;
			const TCHAR* Name;
			const TCHAR* Hint;
			int32 (UGameUserSettings::*Get)() const;
			void (UGameUserSettings::*Set)(int32);
		};
		static const FGroup Groups[6] = {
			{TEXT("view"), TEXT("View distance"), TEXT("How far the trees and rocks reach"),
			 &UGameUserSettings::GetViewDistanceQuality, &UGameUserSettings::SetViewDistanceQuality},
			{TEXT("shadows"), TEXT("Shadows"), TEXT("Their sharpness and reach"), &UGameUserSettings::GetShadowQuality,
			 &UGameUserSettings::SetShadowQuality},
			{TEXT("gi"), TEXT("Lighting"), TEXT("Light bounced from the ground and leaves"),
			 &UGameUserSettings::GetGlobalIlluminationQuality, &UGameUserSettings::SetGlobalIlluminationQuality},
			{TEXT("reflect"), TEXT("Reflections"), TEXT("Water and wet roads"), &UGameUserSettings::GetReflectionQuality,
			 &UGameUserSettings::SetReflectionQuality},
			{TEXT("foliage"), TEXT("Foliage"), TEXT("How thick the grass and plants grow"),
			 &UGameUserSettings::GetFoliageQuality, &UGameUserSettings::SetFoliageQuality},
			{TEXT("effects"), TEXT("Effects"), TEXT("Rain, mist and dust"), &UGameUserSettings::GetVisualEffectQuality,
			 &UGameUserSettings::SetVisualEffectQuality},
		};
		for (const FGroup& G : Groups)
		{
			const auto Set = G.Set;
			Choice(G.Key, Row(G.Name, G.Hint), Qualities(), (V->*G.Get)(),
			       [Set](int32 I)
			       {
				       (Video()->*Set)(I);
				       Apply();
			       });
		}
	}
	else if (Tab == ERideSettingsTab::Rider && Pawn)
	{
		if (bWelcome || !FRideProfile::Saved())
		{
			P.Wrap(Left, Y + 8.f, FMath::Min(Right - Left, 900.f), 24.f,
			       TEXT("Welcome to Veloverse. Set your FTP and weight first: your weight sets how hard the climbs are, "
			            "and FTP sets the power zones, the workouts and the training load."),
			       P.Font("Medium", 17.f), C.Accent, 2);
			Y += 56.f;
		}
		auto Edit = [M](auto Change)
		{
			if (ARidePawn* R = M->Rider())
			{
				FRideProfile Changed = R->Profile;
				Change(Changed);
				R->SetProfile(Changed);
			}
		};
		struct FSlide
		{
			const TCHAR* Key;
			const TCHAR* Name;
			const TCHAR* Hint;
			float Lo, Hi, Step;
			float FRideProfile::*Field; // or nullptr: the max heart rate, an int
			bool bTenths; // shown to 0.1, else whole
			const TCHAR* Unit;
		};
		static const FSlide Slides[4] = {
			{TEXT("ftp"), TEXT("FTP"), TEXT("The power you can hold for an hour"), 100.f, 450.f, 5.f, &FRideProfile::Ftp,
			 false, TEXT("W")},
			{TEXT("weight"), TEXT("Weight"), TEXT("Yours: it sets how hard the climbs are"), 40.f, 150.f, 0.5f,
			 &FRideProfile::WeightKg, true, TEXT("kg")},
			{TEXT("bike"), TEXT("Bike"), TEXT("The bike's weight"), 6.f, 20.f, 0.5f, &FRideProfile::BikeKg,
			 true, TEXT("kg")},
			{TEXT("maxhr"), TEXT("Max heart rate"), TEXT("Sets the heart-rate zones"), 150.f, 215.f, 1.f, nullptr,
			 false, TEXT("bpm")},
		};
		for (const FSlide& S : Slides)
		{
			const float Cy = Row(S.Name, S.Hint);
			const float Value = S.Field ? Pawn->Profile.*(S.Field) : static_cast<float>(Pawn->Profile.MaxHeartRate);
			const FSlide* Sp = &S;
			auto Write = [Edit, Sp](float NewValue)
			{
				NewValue = FMath::Clamp(FMath::RoundToFloat(NewValue / Sp->Step) * Sp->Step, Sp->Lo, Sp->Hi);
				Edit([Sp, NewValue](FRideProfile& Pr)
				     {
					     if (Sp->Field)
					     {
						     Pr.*(Sp->Field) = NewValue;
					     }
					     else
					     {
						     Pr.MaxHeartRate = FMath::RoundToInt32(NewValue);
					     }
				     });
			};
			C.Slider(S.Key, Right - 560.f, Cy, 560.f, (Value - S.Lo) / (S.Hi - S.Lo),
			         S.bTenths ? FString::Printf(TEXT("%.1f %s"), Value, S.Unit) : FString::Printf(TEXT("%.0f %s"), Value, S.Unit), [Write, Sp](float Share) { Write(FMath::Lerp(Sp->Lo, Sp->Hi, Share)); },
			         [Write, Sp, Value](int32 By) { Write(Value + By * Sp->Step); });
		}
		// The HUD's colour: the themes the rider's level has unlocked.
		const float Cy = Row(TEXT("HUD colour"), TEXT("More unlock as you level up"));
		const auto& Themes = Veloverse::Themes();
		float X = Right - Themes.size() * 52.f + 8.f;
		for (const Veloverse::FTheme& Theme : Themes)
		{
			const FString Name = UTF8_TO_TCHAR(Theme.Name);
			const FString Key = TEXT("theme") + Name;
			const bool bOpen = Veloverse::IsUnlocked(Theme, Pawn->Profile.Xp);
			const bool bOn = Pawn->Profile.Theme == Name;
			const FLinearColor Colour = Hex(Theme.Rgb, bOpen ? 1.f : 0.3f);
			if (bOn)
			{
				P.Border(X - 5.f, Cy - 27.f, 54.f, 54.f, 27.f, Ink, 2.f);
			}
			P.Disc(X + 22.f, Cy, 20.f, Colour);
			if (!bOpen)
			{
				P.Icon(Icons::Lock, X + 13.f, Cy - 9.f, 18.f, Ink);
			}
			else
			{
				C.Ring(Key, X - 2.f, Cy - 24.f, 48.f, 48.f, 24.f);
				C.Hit(Key, X - 2.f, Cy - 24.f, 48.f, 48.f).OnClick = [Edit, Name] { Edit([Name](FRideProfile& Pr) { Pr.Theme = Name; }); };
			}
			if (C.Hot(Key) || bOn)
			{
				P.Text(X + 22.f, Cy + 44.f, bOpen ? Name : FString::Printf(TEXT("Level %d"), Theme.Level),
				       P.Font("Medium", 13.f), Muted, EAlign::Centre);
			}
			X += 52.f;
		}
		if (!FRideProfile::Saved())
		{
			C.Button(TEXT("done"), Right - 220.f, 940.f, 220.f, 56.f, TEXT("Done"), EButton::Primary,
			         [M] { M->LeaveRider(); }, Icons::Check);
		}
	}
}

// ---- Stats ---------------------------------------------------------------------------------------

void URideMenu::PaintStats(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.66f);
	PaintHeader(C, TEXT("Your rides"));
	const float RightEdge = C.W - 64.f;

	// The period, every ride, and the rider's level.
	const TArray<FString> Periods = {TEXT("Week"), TEXT("Month"), TEXT("Year"), TEXT("All time")};
	const float Sw = C.SegmentedWidth(Periods, 15.f, 88.f);
	C.Segmented(TEXT("period"), RightEdge - Sw, 48.f, Periods, Period, [M](int32 I) { M->Period = I; }, 42.f, 15.f,
	            88.f);
	float X = RightEdge - Sw - 16.f;
	if (History.Num() > 0)
	{
		const float Bw = C.ButtonWidth(TEXT("Every ride"), 16.f, true, 20.f);
		X -= Bw;
		C.Button(TEXT("rides"), X, 52.f, Bw, 44.f, TEXT("Every ride"), EButton::Outline,
		         [M]
		         {
			         M->bFromStats = true;
			         M->ReviewRide(0);
			         M->Show(ERideMenuPage::Summary);
		         },
		         Icons::Stats, 16.f);
		X -= 32.f;
	}
	if (Pawn)
	{
		const Veloverse::FLevel Level = Veloverse::LevelFor(Pawn->Profile.Xp);
		X -= 220.f;
		P.Text(X, 70.f, FString::Printf(TEXT("Level %d"), Level.Level), P.Font("Bold", 17.f), Ink);
		P.Text(X + 220.f, 70.f,
		       Level.Needed > 0 ? FString::Printf(TEXT("%s / %s XP"), *Thousands(Level.Into), *Thousands(Level.Needed))
		                        : FString(TEXT("Top level")),
		       P.Font("Medium", 14.f), Muted, EAlign::Right);
		P.Round(X, 82.f, 220.f, 6.f, 3.f, InkA(0.14f));
		P.Round(X, 82.f, FMath::Max(220.f * static_cast<float>(Level.Fraction()), 6.f), 6.f, 3.f, C.Accent);
	}

	// The period's totals.
	const FTotals& T = Totals[FMath::Clamp(Period, 0, 3)];
	const float Cw = (RightEdge - 64.f) / 6.f;
	Stat(P, 64.f, 196.f, FString::FromInt(T.Rides), FString(), T.Rides == 1 ? TEXT("Ride") : TEXT("Rides"), 48.f);
	Stat(P, 64.f + Cw, 196.f, FString::Printf(TEXT("%.1f"), T.Km), TEXT("km"), TEXT("Distance"), 48.f);
	Stat(P, 64.f + 2.f * Cw, 196.f, Hours(T.Seconds), FString(), TEXT("Riding time"), 48.f);
	Stat(P, 64.f + 3.f * Cw, 196.f, Thousands(T.Climbed), TEXT("m"), TEXT("Climbing"), 48.f);
	Stat(P, 64.f + 4.f * Cw, 196.f, Thousands(T.Kj), TEXT("kJ"), TEXT("Energy"), 48.f);
	Stat(P, 64.f + 5.f * Cw, 196.f, Thousands(T.Tss), FString(), TEXT("Training stress"), 48.f);

	// Twelve weeks of distance.
	C.Panel(64.f, 268.f, 912.f, 372.f);
	P.Text(88.f, 306.f, TEXT("Distance by week"), P.Font("Bold", 18.f), Ink);
	if (WeekKm.Num() == 12)
	{
		P.Text(952.f, 306.f, FString::Printf(TEXT("%.0f km this week"), WeekKm.Last()), P.Font("Medium", 15.f), Muted,
		       EAlign::Right);
		double Most = 10.0;
		for (const double Km : WeekKm)
		{
			Most = FMath::Max(Most, Km);
		}
		const float Bw = 848.f / 12.f;
		for (int32 I = 0; I < 12; ++I)
		{
			const float H = static_cast<float>(WeekKm[I] / Most) * 220.f;
			const float Bx = 104.f + I * Bw;
			P.Round(Bx, 588.f - FMath::Max(H, 3.f), Bw - 16.f, FMath::Max(H, 3.f), 4.f,
			        I == 11 ? C.Accent : InkA(WeekKm[I] > 0.0 ? 0.32f : 0.1f));
			if (WeekKm[I] > 0.0)
			{
				P.Text(Bx + (Bw - 16.f) * 0.5f, 580.f - H, FString::Printf(TEXT("%.0f"), WeekKm[I]), P.Font("Bold", 13.f),
				       Soft, EAlign::Centre);
			}
			if (I % 2 == 1 && WeekStarts.IsValidIndex(I))
			{
				P.Text(Bx + (Bw - 16.f) * 0.5f, 616.f,
				       FString::Printf(TEXT("%d %s"), WeekStarts[I].GetDay(), MonthName(WeekStarts[I].GetMonth())),
				       P.Font("Medium", 13.f), Muted, EAlign::Centre);
			}
		}
	}

	// The power curve: the best of every ride, and of the last 90 days.
	const float Px0 = 1000.f, Pw = RightEdge - Px0;
	C.Panel(Px0, 268.f, Pw, 372.f);
	P.Text(Px0 + 24.f, 306.f, TEXT("Power curve"), P.Font("Bold", 18.f), Ink);
	{
		const float Lw = P.Text(Px0 + Pw - 24.f, 306.f, TEXT("All time"), P.Font("Medium", 15.f), Muted, EAlign::Right);
		P.Lines({FVector2f(Px0 + Pw - 56.f - Lw, 301.f), FVector2f(Px0 + Pw - 32.f - Lw, 301.f)}, InkA(0.5f), 2.f);
		const float L2 = P.Width(TEXT("Last 90 days"), P.Font("Medium", 15.f));
		const float L2x = Px0 + Pw - 80.f - Lw - L2;
		P.Text(L2x, 306.f, TEXT("Last 90 days"), P.Font("Medium", 15.f), Muted);
		P.Lines({FVector2f(L2x - 32.f, 301.f), FVector2f(L2x - 8.f, 301.f)}, C.Accent, 2.5f);
	}
	double Top = 100.0;
	for (int32 I = 1; I <= 10; ++I)
	{
		Top = FMath::Max(Top, CurveAll[I]);
	}
	Top = FMath::CeilToDouble(Top / 100.0) * 100.0;
	const float Cx0 = Px0 + 64.f, CwP = Pw - 144.f, Base = 584.f, Ch = 220.f;
	auto CurveX = [&](double Seconds)
	{ return Cx0 + static_cast<float>(FMath::Loge(Seconds / 5.0) / FMath::Loge(720.0)) * CwP; };
	for (int32 G = 1; G <= 2; ++G)
	{
		const float Gy = Base - Ch * G / 2.f;
		P.Rect(Cx0, Gy, CwP, 1.f, InkA(0.08f));
		P.Text(Cx0 - 10.f, Mid(Gy, 13.f), FString::Printf(TEXT("%.0f"), Top * G / 2.0), P.Font("Medium", 13.f), Muted,
		       EAlign::Right);
	}
	P.Rect(Cx0, Base, CwP, 1.f, InkA(0.2f));
	const int32 Ticks[5] = {5, 60, 300, 1200, 3600};
	const TCHAR* TickNames[5] = {TEXT("5 s"), TEXT("1 min"), TEXT("5 min"), TEXT("20 min"), TEXT("1 hr")};
	for (int32 I = 0; I < 5; ++I)
	{
		P.Text(CurveX(Ticks[I]), Base + 26.f, TickNames[I], P.Font("Medium", 13.f), Muted, EAlign::Centre);
	}
	auto CurveLine = [&](const Veloverse::FCurve& Curve)
	{
		TArray<FVector2f> Out;
		for (int32 I = 1; I <= 10; ++I)
		{
			if (Curve[I] > 0.0)
			{
				Out.Add(FVector2f(CurveX(Veloverse::CurveDurations[I]), Base - static_cast<float>(Curve[I] / Top) * Ch));
			}
		}
		return Out;
	};
	P.Lines(CurveLine(CurveAll), InkA(0.5f), 2.f);
	const TArray<FVector2f> Recent = CurveLine(Curve90);
	P.Area(Recent, Base, WithAlpha(C.Accent, 0.12f));
	P.Lines(Recent, C.Accent, 2.5f);
	for (const FVector2f& Pt : Recent)
	{
		P.Disc(Pt.X, Pt.Y, 3.5f, C.Accent);
	}
	if (Pawn && Pawn->Ftp > 0.f)
	{
		const float Fy = Base - static_cast<float>(Pawn ? Pawn->Ftp / Top : 0.0) * Ch;
		if (Pawn && Fy > Base - Ch)
		{
			P.Dashed({FVector2f(Cx0, Fy), FVector2f(Cx0 + CwP, Fy)}, InkA(0.35f), 1.f, 5.f, 5.f);
			P.Text(Cx0 + CwP + 8.f, Mid(Fy, 13.f), TEXT("FTP"), P.Font("Bold", 13.f), Muted);
		}
	}

	// Fitness and fatigue, a day at a time.
	C.Panel(64.f, 664.f, 700.f, 352.f);
	P.Text(88.f, 702.f, TEXT("Fitness and fatigue"), P.Font("Bold", 18.f), Ink);
	const double Form = Load.Form();
	P.Text(740.f, 702.f, FString::Printf(TEXT("Form %+.0f, %s"), Form, UTF8_TO_TCHAR(Veloverse::FormWord(Form))),
	       P.Font("Bold", 16.f), Form > 5.0 ? Ok : Form < -30.0 ? Heart : Soft, EAlign::Right);
	if (Load.Fitness.size() > 1)
	{
		double Most = 20.0;
		for (size_t I = 0; I < Load.Fitness.size(); ++I)
		{
			Most = FMath::Max(Most, FMath::Max(Load.Fitness[I], Load.Fatigue[I]));
		}
		for (const double Tss : DayTss)
		{
			Most = FMath::Max(Most, Tss * 0.6);
		}
		Most *= 1.1;
		const float Fx = 104.f, Fw = 620.f, Fy = 732.f, Fh = 220.f;
		const int32 N = DayTss.Num();
		for (int32 I = 0; I < N; ++I)
		{
			if (DayTss[I] > 0.0)
			{
				const float H = FMath::Min(static_cast<float>(DayTss[I] / Most), 1.f) * Fh;
				P.Rect(Fx + Fw * I / FMath::Max(N - 1, 1) - 2.f, Fy + Fh - H, 4.f, H, InkA(0.16f));
			}
		}
		const int32 Days = static_cast<int32>(Load.Fitness.size());
		P.Lines(Plot(Load.Fatigue, Days, Fx, Fy, Fw, Fh, 0.0, Most), Fatigue, 2.f);
		P.Lines(Plot(Load.Fitness, Days, Fx, Fy, Fw, Fh, 0.0, Most), Sky, 2.6f);
		P.Rect(Fx, Fy + Fh, Fw, 1.f, InkA(0.2f));
		P.Text(Fx, Fy + Fh + 26.f, TEXT("90 days ago"), P.Font("Medium", 13.f), Muted);
		P.Text(Fx + Fw, Fy + Fh + 26.f, TEXT("Today"), P.Font("Medium", 13.f), Muted, EAlign::Right);
		const float Lx = Fx + Fw * 0.5f - 130.f;
		P.Disc(Lx, Fy + Fh + 21.f, 5.f, Sky);
		P.Text(Lx + 12.f, Fy + Fh + 26.f, FString::Printf(TEXT("Fitness %.0f"), Load.Fitness.back()),
		       P.Font("Medium", 13.f), Soft);
		P.Disc(Lx + 130.f, Fy + Fh + 21.f, 5.f, Fatigue);
		P.Text(Lx + 142.f, Fy + Fh + 26.f, FString::Printf(TEXT("Fatigue %.0f"), Load.Fatigue.back()),
		       P.Font("Medium", 13.f), Soft);
	}

	// The islands ridden.
	C.Panel(788.f, 664.f, 468.f, 352.f);
	P.Text(812.f, 702.f, TEXT("Islands"), P.Font("Bold", 18.f), Ink);
	P.Text(1232.f, 702.f, FString::Printf(TEXT("%d of %d routes ridden"), RoutesRidden, Surveys.Num()),
	       P.Font("Medium", 15.f), Muted, EAlign::Right);
	double IslandMost = 1.0;
	for (const TPair<FString, double>& I : IslandKm)
	{
		IslandMost = FMath::Max(IslandMost, I.Value);
	}
	for (int32 I = 0; I < IslandKm.Num(); ++I)
	{
		const float Iy = 756.f + I * 50.f;
		const FLinearColor Colour = IslandColour(IslandKm[I].Key);
		P.Disc(818.f, Iy, 6.f, Colour);
		P.Text(834.f, Mid(Iy, 16.f), IslandKm[I].Key, P.Font("Bold", 16.f), Ink);
		P.Round(970.f, Iy - 4.f, 180.f, 8.f, 4.f, InkA(0.08f));
		P.Round(970.f, Iy - 4.f, FMath::Max(180.f * static_cast<float>(IslandKm[I].Value / IslandMost), 8.f), 8.f, 4.f,
		        Colour);
		P.Text(1232.f, Mid(Iy, 15.f), FString::Printf(TEXT("%.0f km"), IslandKm[I].Value), P.Font("Medium", 15.f), Soft,
		       EAlign::Right);
	}

	// The climbs' best times.
	const float Bx = 1280.f, Bw = RightEdge - Bx;
	C.Panel(Bx, 664.f, Bw, 352.f);
	P.Text(Bx + 24.f, 702.f, TEXT("Climb bests"), P.Font("Bold", 18.f), Ink);
	if (ClimbBests.Num() == 0)
	{
		P.Wrap(Bx + 24.f, 748.f, Bw - 48.f, 24.f, TEXT("Ride a route with a climb to set a time on it."),
		       P.Font("Medium", 16.f), Muted);
	}
	for (int32 I = 0; I < FMath::Min(ClimbBests.Num(), 5); ++I)
	{
		const FClimbBest& B = ClimbBests[I];
		const float Cy = 742.f + I * 54.f;
		P.Text(Bx + 24.f, Cy + 14.f, P.Fit(B.Name, P.Font("Bold", 17.f), Bw - 200.f), P.Font("Bold", 17.f), Ink);
		P.Text(Bx + 24.f, Cy + 36.f,
		       FString::Printf(TEXT("%s · %d %s"), *Watts(B.Watts), B.Tries, B.Tries == 1 ? TEXT("try") : TEXT("tries")),
		       P.Font("Medium", 14.f), Muted);
		P.Text(Bx + Bw - 24.f, Cy + 22.f, Clock(B.Seconds, false), P.Font("Bold", 20.f), B.bRecent ? C.Accent : Ink,
		       EAlign::Right);
		if (B.bRecent)
		{
			Label(P, Bx + Bw - 24.f, Cy + 42.f, TEXT("PR this week"), C.Accent, EAlign::Right, 11.f);
		}
	}
	if (History.Num() == 0)
	{
		P.Text(520.f, 460.f, TEXT("No rides yet: your first one starts the charts."), P.Font("Medium", 17.f), Muted,
		       EAlign::Centre);
	}
}

// ---- Pause ---------------------------------------------------------------------------------------

void URideMenu::PaintPause(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.72f);
	const bool bFinished = Pawn && Pawn->bFinished;
	const float Wd = 560.f, X = (C.W - Wd) * 0.5f;
	const int32 Buttons = bFinished ? 4 : 5;
	const float H = 196.f + Buttons * 70.f + 24.f;
	const float Y = (1080.f - H) * 0.5f;
	C.Panel(X, Y, Wd, H, 20.f, 0.88f);
	P.Text(X + 40.f, Y + 84.f, bFinished ? TEXT("Finished") : TEXT("Paused"), P.Serif(56.f), Ink);
	if (Pawn)
	{
		const float Km = static_cast<float>(Pawn->Ridden() / 100000.0);
		const float Route = Pawn->Track ? Pawn->Track->Length() / 100000.f : 0.f;
		const float Kmh = Pawn->Elapsed > 0.f ? Km / (Pawn->Elapsed / 3600.f) : 0.f;
		P.Wrap(X + 40.f, Y + 126.f, Wd - 80.f, 24.f,
		       FString::Printf(TEXT("%.2f of %.1f km in %s, averaging %.1f km/h"), Km, Route,
		                       *Clock(Pawn->Elapsed, false), Kmh),
		       P.Font("Medium", 18.f), Soft, 2);
	}
	float By = Y + 180.f;
	auto Add = [&](const TCHAR* Key, const TCHAR* Text, EButton Style, TFunction<void()> Click, const TCHAR* Icon)
	{
		C.Button(Key, X + 40.f, By, Wd - 80.f, 58.f, Text, Style, MoveTemp(Click), Icon);
		By += 70.f;
	};
	if (bFinished)
	{
		Add(TEXT("review"), TEXT("Review the ride"), EButton::Primary, [M] { M->Controller()->EndRide(); }, Icons::Stats);
	}
	else
	{
		Add(TEXT("resume"), TEXT("Resume"), EButton::Primary, [M] { M->Controller()->Resume(); }, nullptr);
		Add(TEXT("end"), TEXT("End ride"), EButton::Outline, [M] { M->Controller()->EndRide(); }, nullptr);
	}
	Add(TEXT("settings"), TEXT("Settings"), EButton::Outline, [M] { M->Show(ERideMenuPage::Settings); }, Icons::Gear);
	Add(TEXT("home"), TEXT("Back to the start"), EButton::Outline, [M] { M->Controller()->Restart(); }, nullptr);
	Add(TEXT("quit"), TEXT("Quit"), EButton::Quiet, [M] { M->Controller()->Quit(); }, Icons::Leave);
}

// ---- A ride's review -----------------------------------------------------------------------------

void URideMenu::PaintReview(FPaintContext& C) const
{
	const FPainter& P = C.P;
	URideMenu* M = &C.Menu;
	const ARidePawn* Pawn = C.Pawn;
	Shade(P, C.W, 0.7f);
	const float RightEdge = C.W - 48.f;
	const bool bAfterRide = Pawn && Pawn->bStarted;

	// The rides, newest first.
	C.Panel(48.f, 48.f, 300.f, 984.f);
	P.Text(72.f, 90.f, TEXT("Rides"), P.Font("Bold", 18.f), Ink);
	if (History.Num() == 0)
	{
		P.Wrap(72.f, 130.f, 252.f, 22.f, TEXT("No rides kept yet: rides over 30 seconds are saved."),
		       P.Font("Medium", 15.f), Muted);
	}
	constexpr int32 Shown = 12;
	for (int32 I = RidesFirst; I < FMath::Min(History.Num(), RidesFirst + Shown); ++I)
	{
		const FRideEntry& E = History[I];
		const float Ry = 108.f + (I - RidesFirst) * 72.f;
		const FString Key = FString::Printf(TEXT("ride%d"), I);
		const bool bOn = Review.Entry == I;
		if (bOn || C.Hot(Key))
		{
			P.Round(60.f, Ry, 276.f, 66.f, 10.f, bOn ? WithAlpha(C.Accent, 0.14f) : InkA(0.07f));
		}
		P.Text(76.f, Ry + 28.f, RideDay(E.Started), P.Font("Bold", 16.f),
		       bOn ? FMath::Lerp(C.Accent, FLinearColor::White, 0.45f) : Ink);
		P.Text(76.f, Ry + 52.f,
		       P.Fit(FString::Printf(TEXT("%s · %.1f km"), E.Route.IsEmpty() ? TEXT("Ride") : *E.Route,
		                             E.Summary.Metres / 1000.0),
		             P.Font("Medium", 14.f), 248.f),
		       P.Font("Medium", 14.f), Muted);
		C.Ring(Key, 60.f, Ry, 276.f, 66.f, 10.f);
		C.Hit(Key, 60.f, Ry, 276.f, 66.f).OnClick = [M, I] { M->ReviewRide(I); };
	}
	if (History.Num() > Shown)
	{
		P.Text(198.f, 1010.f,
		       FString::Printf(TEXT("%d to %d of %d"), RidesFirst + 1, FMath::Min(History.Num(), RidesFirst + Shown),
		                       History.Num()),
		       P.Font("Medium", 13.f), Muted, EAlign::Centre);
	}

	// The heading and the ways out.
	const float X0 = 380.f;
	float Bx = RightEdge;
	auto RightButton = [&](const TCHAR* Key, const FString& Text, EButton Style, TFunction<void()> Click,
	                       const TCHAR* Icon)
	{
		const float Bw = C.ButtonWidth(Text, 16.f, Icon != nullptr, 20.f);
		Bx -= Bw;
		C.Button(Key, Bx, 56.f, Bw, 48.f, Text, Style, MoveTemp(Click), Icon, 16.f);
		Bx -= 12.f;
	};
	if (bAfterRide)
	{
		RightButton(TEXT("home"), TEXT("Back to the start"), EButton::Primary, [M] { M->Controller()->Restart(); },
		            nullptr);
		RightButton(TEXT("another"), TEXT("Ride another route"), EButton::Outline,
		            [M]
		            {
			            URideMenu::bRoutesNext = true;
			            M->Controller()->Restart();
		            },
		            Icons::Map);
	}
	else
	{
		RightButton(TEXT("home"), bFromStats ? TEXT("Back to stats") : TEXT("Back"), EButton::Primary,
		            [M] { M->GoBack(); }, Icons::Back);
	}
	RightButton(TEXT("folder"), TEXT("Open rides folder"), EButton::Quiet,
	            [] { FPlatformProcess::ExploreFolder(*FRideRecorder::RidesFolder()); }, Icons::Download);
	if (!Review.bValid)
	{
		P.Text(X0, 140.f, TEXT("No ride to show"), P.Serif(56.f), Ink);
		return;
	}
	const Veloverse::FSummary& S = Review.Summary;
	FString Kicker = RideDay(Review.Started);
	if (!Review.Islands.IsEmpty())
	{
		Kicker += TEXT(" · ") + Review.Islands;
	}
	P.Text(X0, 80.f, P.Fit(Kicker.ToUpper(), P.Font("Bold", 14.f, 0.2f), Bx - X0 - 24.f), P.Font("Bold", 14.f, 0.2f),
	       C.Accent);
	const float Tw = P.Text(X0, 140.f, P.Fit(Review.Route, P.Serif(56.f), Bx - X0 - 260.f), P.Serif(56.f), Ink);
	FString Saved;
	if (Review.bJustRidden)
	{
		Saved = Pawn && Pawn->bRideSaved && !Review.Saved.IsEmpty() ? TEXT("Saved to your rides")
			: S.Seconds < FRideRecorder::MinSeconds                ? TEXT("Under 30 seconds, so not kept.")
			                                                        : TEXT("Not saved.");
	}
	if (Review.Xp > 0)
	{
		Saved += FString::Printf(TEXT("%s+%d XP"), Saved.IsEmpty() ? TEXT("") : TEXT(" · "), Review.Xp);
	}
	P.Text(X0 + Tw + 20.f, 136.f, Saved, P.Font("Medium", 16.f), Muted);

	// The totals.
	struct FTotal
	{
		FString Value, Unit;
		const TCHAR* Name;
	};
	const FTotal Row[8] = {
		{FString::Printf(TEXT("%.2f"), S.Metres / 1000.0), TEXT("km"), TEXT("Distance")},
		{Clock(S.Seconds, false), FString(), TEXT("Time")},
		{FString::Printf(TEXT("%.1f"), S.AverageSpeed * 3.6), TEXT("km/h"), TEXT("Average speed")},
		{Thousands(S.Climbed), TEXT("m"), TEXT("Climbing")},
		{S.AverageWatts > 0.0 ? FString::Printf(TEXT("%.0f"), S.AverageWatts) : FString(TEXT("–")), TEXT("W"),
		 TEXT("Average power")},
		{S.NormalizedWatts > 0.0 ? FString::Printf(TEXT("%.0f"), S.NormalizedWatts) : FString(TEXT("–")), TEXT("W"),
		 TEXT("Normalized")},
		{Thousands(S.Kilojoules), TEXT("kJ"), TEXT("Energy")},
		{FString::Printf(TEXT("%.0f"), S.Tss), FString(), TEXT("Training stress")},
	};
	const float Tcw = (RightEdge - X0) / 8.f;
	for (int32 I = 0; I < 8; ++I)
	{
		Stat(P, X0 + I * Tcw, 214.f, Row[I].Value, Row[I].Unit, Row[I].Name, 34.f);
	}

	// The traces along the ride.
	const float Tx = X0, Tw2 = RightEdge - X0;
	C.Panel(Tx, 288.f, Tw2, 408.f);
	const Veloverse::FTraces& Tr = Review.Traces;
	const float Gx = Tx + 194.f, Gw = Tx + Tw2 - 24.f - Gx;
	struct FTrace
	{
		const TCHAR* Name;
		const std::vector<double>* Values;
		FLinearColor Colour;
		float Top, H;
		const TCHAR* Unit;
		double Average, Max;
	};
	const FTrace Traces[4] = {
		{TEXT("Power"), &Tr.Watts, C.Accent, 304.f, 96.f, TEXT("W"), S.AverageWatts, S.MaxWatts},
		{TEXT("Heart rate"), &Tr.HeartRate, Heart, 412.f, 88.f, TEXT("bpm"), S.AverageHeartRate, S.MaxHeartRate},
		{TEXT("Cadence"), &Tr.Cadence, Sky, 512.f, 88.f, TEXT("rpm"), S.AverageCadence, 0.0},
		{TEXT("Elevation"), &Tr.Elevation, Soft, 612.f, 64.f, TEXT("m"), 0.0, 0.0},
	};
	for (int32 I = 0; I < 4; ++I)
	{
		const FTrace& T = Traces[I];
		const std::vector<double>& V = *T.Values;
		Label(P, Tx + 24.f, T.Top + 18.f, T.Name, T.Colour);
		double Lo = 0.0, Hi = 0.0;
		Range(V, Lo, Hi);
		const bool bAny = Hi > 0.0 && V.size() > 1;
		if (I == 3)
		{
			const double Climbed = S.Climbed;
			P.Text(Tx + 24.f, T.Top + 52.f, FString::Printf(TEXT("%.0f to %.0f m"), Lo, Hi), P.Font("Bold", 20.f), Ink);
			P.Text(Tx + 24.f, T.Top + 72.f, FString::Printf(TEXT("%s m up"), *Thousands(Climbed)),
			       P.Font("Medium", 13.f), Muted);
		}
		else if (T.Average > 0.0)
		{
			const float Vw = P.Text(Tx + 24.f, T.Top + 54.f, FString::Printf(TEXT("%.0f"), T.Average),
			                        P.Font("Bold", 28.f), Ink);
			P.Text(Tx + 30.f + Vw, T.Top + 54.f, T.Unit, P.Font("Medium", 14.f), Muted);
			if (T.Max > 0.0)
			{
				P.Text(Tx + 24.f, T.Top + 76.f, FString::Printf(TEXT("average, max %.0f"), T.Max),
				       P.Font("Medium", 13.f), Muted);
			}
			else
			{
				P.Text(Tx + 24.f, T.Top + 76.f, TEXT("average"), P.Font("Medium", 13.f), Muted);
			}
		}
		else
		{
			P.Text(Tx + 24.f, T.Top + 54.f, TEXT("–"), P.Font("Bold", 28.f), Muted);
		}
		P.Rect(Gx, T.Top + T.H, Gw, 1.f, InkA(0.1f));
		if (!bAny)
		{
			P.Text(Gx + 12.f, Mid(T.Top + T.H * 0.5f, 15.f),
			       I == 1 ? TEXT("No heart-rate strap on this ride") : TEXT("Nothing recorded"), P.Font("Medium", 15.f),
			       Muted);
			continue;
		}
		const int32 N = static_cast<int32>(V.size());
		if (I == 1)
		{
			Lo = FMath::Max(Lo - 10.0, 0.0);
		}
		else if (I == 3)
		{
			Hi = FMath::Max(Hi, Lo + 20.0);
		}
		else
		{
			Lo = 0.0;
		}
		Hi *= I == 3 ? 1.0 : 1.05;
		if (I == 3 && Review.IslandSpans.Num() > 0)
		{
			for (int32 Sp = 0; Sp < Review.IslandSpans.Num(); ++Sp)
			{
				const float From = Review.IslandSpans[Sp].Key;
				const float To = Sp + 1 < Review.IslandSpans.Num() ? Review.IslandSpans[Sp + 1].Key : 1.f;
				P.Area(Plot(V, N, Gx, T.Top, Gw, T.H, Lo, Hi, FMath::FloorToInt32(From * (N - 1)),
				            FMath::CeilToInt32(To * (N - 1))),
				       T.Top + T.H, WithAlpha(IslandColour(Review.IslandSpans[Sp].Value), 0.38f));
			}
		}
		else
		{
			P.Area(Plot(V, N, Gx, T.Top, Gw, T.H, Lo, Hi), T.Top + T.H, WithAlpha(T.Colour, 0.18f));
		}
		P.Lines(Plot(V, N, Gx, T.Top, Gw, T.H, Lo, Hi), T.Colour, 1.5f);
		if (I == 0 && T.Average > 0.0)
		{
			const float Ay = T.Top + T.H * (1.f - static_cast<float>(T.Average / Hi));
			P.Dashed({FVector2f(Gx, Ay), FVector2f(Gx + Gw, Ay)}, InkA(0.4f), 1.f, 5.f, 5.f);
		}
	}
	P.Text(Gx + Gw, 690.f, FString::Printf(TEXT("%.1f km"), Tr.Metres / 1000.0), P.Font("Medium", 12.f), Muted,
	       EAlign::Right);

	// Time in each power zone.
	C.Panel(X0, 716.f, 520.f, 300.f);
	P.Text(X0 + 24.f, 752.f, TEXT("Power zones"), P.Font("Bold", 18.f), Ink);
	P.Text(X0 + 496.f, 752.f, FString::Printf(TEXT("FTP %.0f W"), Review.Ftp), P.Font("Medium", 14.f), Muted,
	       EAlign::Right);
	double ZoneMost = 1.0, ZoneAll = 0.0;
	for (const double Z : S.ZoneSeconds)
	{
		ZoneMost = FMath::Max(ZoneMost, Z);
		ZoneAll += Z;
	}
	for (int32 Z = 0; Z < 7; ++Z)
	{
		const float Zy = 784.f + Z * 32.f;
		const FLinearColor Colour = Hex(ZoneHex[Z]);
		P.Text(X0 + 24.f, Mid(Zy, 14.f), FString::Printf(TEXT("Z%d"), Z + 1), P.Font("Bold", 14.f), Colour);
		P.Text(X0 + 56.f, Mid(Zy, 14.f), ZoneNames[Z], P.Font("Medium", 14.f), Soft);
		P.Round(X0 + 160.f, Zy - 8.f, 220.f, 16.f, 4.f, InkA(0.05f));
		if (S.ZoneSeconds[Z] > 0.0)
		{
			P.Round(X0 + 160.f, Zy - 8.f, FMath::Max(220.f * static_cast<float>(S.ZoneSeconds[Z] / ZoneMost), 4.f), 16.f,
			        4.f, Colour);
		}
		P.Text(X0 + 440.f, Mid(Zy, 14.f), Clock(S.ZoneSeconds[Z], false), P.Font("Bold", 14.f), Ink, EAlign::Right);
		P.Text(X0 + 496.f, Mid(Zy, 14.f),
		       FString::Printf(TEXT("%.0f%%"), ZoneAll > 0.0 ? S.ZoneSeconds[Z] / ZoneAll * 100.0 : 0.0),
		       P.Font("Medium", 14.f), Muted, EAlign::Right);
	}

	// Heart-rate zones and cadence.
	const float Hx = X0 + 540.f, Hw = 420.f;
	C.Panel(Hx, 716.f, Hw, 300.f);
	P.Text(Hx + 24.f, 752.f, TEXT("Heart-rate zones"), P.Font("Bold", 18.f), Ink);
	double HeartAll = 0.0;
	for (const double Hs : Review.HeartSeconds)
	{
		HeartAll += Hs;
	}
	if (HeartAll <= 0.0)
	{
		P.Text(Hx + 24.f, 790.f, TEXT("No heart-rate strap on this ride"), P.Font("Medium", 15.f), Muted);
	}
	else
	{
		P.Text(Hx + Hw - 24.f, 752.f, FString::Printf(TEXT("max %d"), Review.MaxHeartRate), P.Font("Medium", 14.f),
		       Muted, EAlign::Right);
		float Sx = Hx + 24.f;
		const float Bar = Hw - 48.f;
		for (int32 Z = 0; Z < 5; ++Z)
		{
			const float Wd = Bar * static_cast<float>(Review.HeartSeconds[Z] / HeartAll);
			if (Wd > 0.5f)
			{
				P.Rect(Sx, 770.f, FMath::Max(Wd - 2.f, 1.f), 14.f, Hex(HeartHex[Z]));
			}
			Sx += Wd;
		}
		for (int32 Z = 0; Z < 5; ++Z)
		{
			const float Lx = Hx + 24.f + (Z % 3) * 128.f, Ly = 812.f + (Z / 3) * 24.f;
			P.Disc(Lx + 5.f, Ly - 5.f, 5.f, Hex(HeartHex[Z]));
			P.Text(Lx + 16.f, Ly,
			       FString::Printf(TEXT("%s %.0f%%"), HeartNames[Z], Review.HeartSeconds[Z] / HeartAll * 100.0),
			       P.Font("Medium", 13.f), Soft);
		}
	}
	P.Text(Hx + 24.f, 880.f, TEXT("Cadence"), P.Font("Bold", 18.f), Ink);
	const Veloverse::FCadenceBands& Cad = Review.Cadence;
	const int32 Peak = Cad.Peak();
	if (Peak >= 0)
	{
		P.Text(Hx + Hw - 24.f, 880.f,
		       FString::Printf(TEXT("Mostly %.0f to %.0f rpm"), Cad.Low(Peak), Cad.Low(Peak) + 5.0),
		       P.Font("Medium", 14.f), Muted, EAlign::Right);
		double Most = 1.0;
		for (const double Sec : Cad.Seconds)
		{
			Most = FMath::Max(Most, Sec);
		}
		const float Cw2 = (Hw - 48.f) / Cad.Count();
		for (int32 B = 0; B < Cad.Count(); ++B)
		{
			const float H = static_cast<float>(Cad.Seconds[B] / Most) * 76.f;
			const float Cx = Hx + 24.f + B * Cw2;
			if (H > 0.f)
			{
				P.Round(Cx, 976.f - H, Cw2 - 4.f, H, 3.f, B == Peak ? Sky : WithAlpha(Sky, 0.35f));
			}
			if (B % 2 == 0)
			{
				P.Text(Cx + (Cw2 - 4.f) * 0.5f, 998.f, FString::Printf(TEXT("%.0f"), Cad.Low(B)), P.Font("Medium", 12.f),
				       Muted, EAlign::Centre);
			}
		}
	}
	else
	{
		P.Text(Hx + 24.f, 916.f, TEXT("No pedalling recorded"), P.Font("Medium", 15.f), Muted);
	}

	// Best efforts and the climbs.
	const float Ex = Hx + Hw + 20.f, Ew = RightEdge - Ex;
	C.Panel(Ex, 716.f, Ew, 300.f);
	P.Text(Ex + 24.f, 752.f, TEXT("Best efforts"), P.Font("Bold", 18.f), Ink);
	const TCHAR* Durations[5] = {TEXT("5 s"), TEXT("1 min"), TEXT("5 min"), TEXT("20 min"), TEXT("1 hr")};
	for (int32 I = 0; I < 5; ++I)
	{
		const float Ey = 782.f + I * 26.f;
		P.Text(Ex + 24.f, Ey, Durations[I], P.Font("Medium", 14.f), Muted);
		P.Text(Ex + 150.f, Ey, Watts(S.Best[I]), P.Font("Bold", 15.f), S.Best[I] > 0.0 ? Ink : Muted, EAlign::Right);
		if (!Review.BestNote[I].IsEmpty())
		{
			P.Text(Ex + 166.f, Ey, Review.BestNote[I], P.Font("Bold", 13.f), C.Accent);
		}
	}
	P.Text(Ex + 24.f, 930.f, TEXT("Climbs"), P.Font("Bold", 16.f), Ink);
	if (Review.Climbs.Num() == 0)
	{
		P.Text(Ex + 24.f, 958.f, TEXT("None on this ride"), P.Font("Medium", 14.f), Muted);
	}
	for (int32 I = 0; I < FMath::Min(Review.Climbs.Num(), 3); ++I)
	{
		const FRideClimb& Cl = Review.Climbs[I];
		const float Cy = 958.f + I * 24.f;
		const FString Place = Cl.bPr ? FString(TEXT("PR")) : Cl.Of <= 1 ? FString(TEXT("first time"))
		                                                                  : FString::Printf(TEXT("%d of %d"), Cl.Rank, Cl.Of);
		P.Text(Ex + 24.f, Cy, P.Fit(Cl.Name, P.Font("Medium", 14.f), Ew - 200.f), P.Font("Medium", 14.f), Soft);
		P.Text(Ex + Ew - 90.f, Cy, Clock(Cl.Seconds, false), P.Font("Bold", 14.f), Ink, EAlign::Right);
		P.Text(Ex + Ew - 24.f, Cy, Place, P.Font("Bold", 13.f), Cl.bPr ? C.Accent : Muted, EAlign::Right);
	}
}
