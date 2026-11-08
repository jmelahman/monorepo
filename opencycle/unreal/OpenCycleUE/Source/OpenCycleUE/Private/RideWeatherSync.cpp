#include "RideWeatherSync.h"

#include "Camera/PlayerCameraManager.h"
#include "GameFramework/PlayerController.h"
#include "Materials/MaterialParameterCollection.h"
#include "Materials/MaterialParameterCollectionInstance.h"
#include "RideTrack.h"

ARideWeatherSync::ARideWeatherSync()
{
	PrimaryActorTick.bCanEverTick = true;
	// After the sequence has moved the camera this frame.
	PrimaryActorTick.TickGroup = TG_PostUpdateWork;
}

void ARideWeatherSync::BeginPlay()
{
	Super::BeginPlay();
	Track = LoadObject<URideTrack>(nullptr, TEXT("/Game/Coast/Ride/DA_RideTrack.DA_RideTrack"));
}

void ARideWeatherSync::Tick(float Dt)
{
	Super::Tick(Dt);
	const APlayerController* Player = GetWorld()->GetFirstPlayerController();
	if (!Track || !Track->Collection || !Player || !Player->PlayerCameraManager)
	{
		return;
	}
	const float Where = Track->Nearest(Player->PlayerCameraManager->GetCameraLocation());
	const FRideWeather W = Track->WeatherAt(Where / 100.f);
	if (UMaterialParameterCollectionInstance* Values = GetWorld()->GetParameterCollectionInstance(Track->Collection))
	{
		Values->SetScalarParameterValue(TEXT("Rain"), W.Rain);
		Values->SetScalarParameterValue(TEXT("Wet"), W.Wet);
	}
}
