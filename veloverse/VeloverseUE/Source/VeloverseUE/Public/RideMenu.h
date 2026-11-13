// The menus: the start screen, choosing a route, the trainer and sensors, the settings, the stats,
// the pause dialog and a ride's review. One UMG widget painted in code from the design canvas's
// boards (a 1920x1080 frame scaled to the screen's height), so it needs no Blueprint asset.
// ARidePlayerController opens and closes it.
//
// Everything that can be clicked is a hit box, rebuilt as each frame is painted: the mouse hovers
// and clicks them, and the arrow keys or a gamepad's d-pad move a focus ring between them, Enter
// or A presses, and Left and Right step a choice or a slider. Esc, P or Start goes back.
#pragma once

#include "CoreMinimal.h"
#include "Blueprint/UserWidget.h"
#include "RideRecord.h"
#include "Styling/SlateBrush.h"
#include "Workout.h"
#include "RideMenu.generated.h"

class ARidePawn;
class ARidePlayerController;
class UBackgroundBlur;
class URideTrack;
class UTexture2D;

enum class ERideMenuPage : uint8
{
	Start,
	Routes,
	Sensors,
	Settings,
	Stats,
	Pause,
	Summary, // a ride's review: the one just ended, or any saved one
};

enum class ERideSettingsTab : uint8
{
	Graphics,
	Detail,
	Rider,
};

UCLASS()
class VELOVERSEUE_API URideMenu : public UUserWidget
{
	GENERATED_BODY()

public:
	void Show(ERideMenuPage Page);
	void SetTab(ERideSettingsTab Tab);
	ERideMenuPage Page() const { return Current; }

	// The frame's painter and what a page needs while it paints; defined in RideMenu.cpp.
	struct FPaintContext;

protected:
	virtual void NativeOnInitialized() override;
	virtual TSharedRef<SWidget> RebuildWidget() override;
	virtual void NativeTick(const FGeometry& Geometry, float DeltaTime) override;
	virtual int32 NativePaint(const FPaintArgs& Args, const FGeometry& Geometry, const FSlateRect& Culling,
	                          FSlateWindowElementList& Out, int32 Layer, const FWidgetStyle& Style,
	                          bool bParentEnabled) const override;
	virtual FReply NativeOnKeyDown(const FGeometry& Geometry, const FKeyEvent& Event) override;
	virtual FReply NativeOnMouseButtonDown(const FGeometry& Geometry, const FPointerEvent& Event) override;
	virtual FReply NativeOnMouseButtonUp(const FGeometry& Geometry, const FPointerEvent& Event) override;
	virtual FReply NativeOnMouseMove(const FGeometry& Geometry, const FPointerEvent& Event) override;
	virtual FReply NativeOnMouseWheel(const FGeometry& Geometry, const FPointerEvent& Event) override;

private:
	// Something to click, in frame units, from the last frame painted.
	struct FHit
	{
		FString Key; // stable from frame to frame, so hover and focus follow it
		float X = 0.f, Y = 0.f, W = 0.f, H = 0.f;
		TFunction<void()> OnClick;
		TFunction<void()> OnLeft, OnRight; // a choice's previous and next
		TFunction<void(float)> OnDrag;     // a slider: the share of its width the pointer is at
		bool Contains(FVector2f P) const { return P.X >= X && P.X < X + W && P.Y >= Y && P.Y < Y + H; }
	};
	mutable TArray<FHit> Hits;
	mutable float Scale = 1.f;               // screen pixels a frame unit, from the last paint
	FVector2f Mouse = FVector2f(-1.f, -1.f); // in frame units
	FString Focus;                           // the hit the keys move, ringed once they've been used
	bool bKeys = false;
	FString Dragging;
	mutable FString Hover; // the hit under the mouse, found as the frame paints

	// Each route's numbers and lines, read once per track.
	struct FRouteSurvey
	{
		TWeakObjectPtr<const URideTrack> Track;
		FString Title, Blurb;
		float Km = 0.f, Climb = 0.f, Highest = 0.f;
		TArray<float> Heights;                 // metres, evenly along the route
		TArray<TPair<float, FString>> Islands; // where each island's stretch starts, as shares of the route
		TArray<FVector2f> Map;                 // the road in map pixels
		bool bClosed = true;
	};
	TArray<FRouteSurvey> Surveys;
	const FRouteSurvey* SurveyOf(const URideTrack* Track) const;
	void SurveyRoutes();

	// The island map from Content/Menu (scripts/make_menu_map.py).
	UPROPERTY(Transient) TObjectPtr<UTexture2D> MapTexture;
	mutable FSlateBrush MapBrush;
	float MapScale = 0.f, MapX0 = 0.f, MapY0 = 0.f, MapWidth = 1232.f, MapHeight = 1130.f;
	TMap<FString, FVector2f> MapLabels;
	void ReadMap();

	UPROPERTY(Transient) TObjectPtr<UBackgroundBlur> Blur;

	// The saved rides, newest first, read as the stats or a review opens.
	TArray<FRideEntry> History;
	void ReadHistory();

	// The stats page, worked out from History.
	int32 Period = 1; // this week, month, year, all time
	struct FTotals
	{
		int32 Rides = 0;
		double Km = 0.0, Seconds = 0.0, Climbed = 0.0, Kj = 0.0, Tss = 0.0;
	};
	FTotals Totals[4];
	TArray<double> WeekKm; // the last 12 weeks, this one last
	TArray<FDateTime> WeekStarts;
	Veloverse::FCurve CurveAll = {}, Curve90 = {};
	Veloverse::FLoad Load;  // a day at a time, the last 90 days
	TArray<double> DayTss;  // and their load
	TArray<TPair<FString, double>> IslandKm;
	int32 RoutesRidden = 0;
	struct FClimbBest
	{
		FString Name;
		double Seconds = 0.0, Watts = 0.0;
		int32 Tries = 0;
		bool bRecent = false; // a PR in the last week
	};
	TArray<FClimbBest> ClimbBests;
	void WorkOutStats();

	// The review: one ride, drawn from its samples.
	struct FReview
	{
		bool bValid = false;
		bool bJustRidden = false; // the ride that just ended, rather than one from the list
		int32 Entry = -1;         // its History index, if it was kept
		FString Route, Islands, Saved;
		FDateTime Started;
		float Ftp = 250.f;
		int32 MaxHeartRate = 190;
		int32 Xp = 0;
		Veloverse::FSummary Summary;
		Veloverse::FTraces Traces;
		TArray<TPair<float, FString>> IslandSpans; // where each island starts, as shares of the ride
		std::array<double, 5> HeartSeconds = {};
		Veloverse::FCadenceBands Cadence;
		TArray<FRideClimb> Climbs;
		FString BestNote[5];
	};
	FReview Review;
	int32 RidesFirst = 0; // the review's ride list, scrolled
	void ReviewRide(int32 Entry);
	void ReviewJustRidden();
	void FillReview(const std::vector<Veloverse::FSample>& Samples);

	ERideMenuPage Current = ERideMenuPage::Start;
	ERideMenuPage Back = ERideMenuPage::Start; // where the sub-pages return to
	ERideSettingsTab Tab = ERideSettingsTab::Graphics;
	TArray<Veloverse::FWorkout> Workouts; // the library, for the start page's workout choice
	float WorkoutsFtp = -1.f;             // the FTP the library was read with (files in watts need it)
	float RefreshClock = 0.f;
	bool bWelcome = false;   // a first run: the rider's settings open before the start page
	bool bFromStats = false; // the review was opened from the stats page
	FString Version;
	static bool bRoutesNext; // "Ride another route": the next start screen opens on the routes

	ARidePlayerController* Controller() const;
	ARidePawn* Rider() const;
	void GoBack();
	void LeaveRider();
	void ReadWorkouts(float RiderFtp);
	void StepWorkout(int32 Step);
	FString DefaultFocus() const;
	void MoveFocus(FVector2f Direction);
	const FHit* Find(const FString& Key) const;
	FVector2f ToFrame(const FGeometry& Geometry, FVector2D Screen) const;
	void Press(const FString& Key);

	void PaintStart(FPaintContext& C) const;
	void PaintRoutes(FPaintContext& C) const;
	void PaintSensors(FPaintContext& C) const;
	void PaintSettings(FPaintContext& C) const;
	void PaintStats(FPaintContext& C) const;
	void PaintPause(FPaintContext& C) const;
	void PaintReview(FPaintContext& C) const;
	void PaintHeader(FPaintContext& C, const FString& Title) const;
};
