// Builds the parts of a world the Python scripts can't reach through the editor's Python API.
#pragma once

#include "CoreMinimal.h"
#include "Kismet/BlueprintFunctionLibrary.h"
#include "WorldBuilderLibrary.generated.h"

class ALandscape;
class ULandscapeLayerInfoObject;
class UMaterial;
class UMaterialInterface;

UCLASS()
class OPENCYCLEEDITOR_API UWorldBuilderLibrary : public UBlueprintFunctionLibrary
{
	GENERATED_BODY()

public:
	// A landscape in World, its corner at Location, from HeightFile: 16-bit little-endian heights,
	// (ComponentsX * QuadsPerComponent + 1) by (ComponentsY * QuadsPerComponent + 1), rows along X,
	// 32768 at Location's height and Scale.Z / 128 cm a step. Each of Layers paints from the 8-bit
	// file of the same index in LayerFiles, the same size. In a World Partition world it is split
	// into streaming proxies GridSize components a side. QuadsPerComponent is 7, 15, 31, 63, 127 or
	// 255.
	UFUNCTION(BlueprintCallable, Category = "OpenCycle")
	static ALandscape* MakeLandscape(UWorld* World, const FString& HeightFile, int32 ComponentsX, int32 ComponentsY,
		int32 QuadsPerComponent, FVector Location, FVector Scale, UMaterialInterface* Material,
		const TArray<ULandscapeLayerInfoObject*>& Layers, const TArray<FString>& LayerFiles, int32 GridSize);

	// A layer info asset at PackagePath (e.g. /Game/World/Layers/LI_Grass) named LayerName, saved.
	UFUNCTION(BlueprintCallable, Category = "OpenCycle")
	static ULandscapeLayerInfoObject* MakeLayerInfo(const FString& PackagePath, FName LayerName);

	// Compiles Material for this machine's shader platform, waits, and returns its errors, empty if
	// it compiled. The editor's Python can recompile a material but can't read why it failed.
	UFUNCTION(BlueprintCallable, Category = "OpenCycle")
	static TArray<FString> MaterialErrors(UMaterial* Material);

	// One line per loaded landscape component in World: its proxy, material instances and their
	// parents, and the weightmap layers it holds. For checking a generated landscape.
	UFUNCTION(BlueprintCallable, Category = "OpenCycle")
	static TArray<FString> DescribeLandscape(UWorld* World, int32 MaxComponents = 8);
};
