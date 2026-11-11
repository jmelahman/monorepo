#include "WorldBuilderLibrary.h"

#include "Landscape.h"
#include "EngineUtils.h"
#include "LandscapeComponent.h"
#include "LandscapeEditTypes.h"
#include "LandscapeInfo.h"
#include "LandscapeLayerInfoObject.h"
#include "LandscapeProxy.h"
#include "LandscapeSubsystem.h"
#include "Materials/Material.h"
#include "Materials/MaterialInstanceConstant.h"
#include "MaterialShared.h"
#include "Misc/FileHelper.h"
#include "RenderingThread.h"
#include "ShaderCompiler.h"
#include "UObject/Package.h"
#include "UObject/SavePackage.h"

DEFINE_LOG_CATEGORY_STATIC(LogWorldBuilder, Log, All);

namespace
{
bool ReadSamples(const FString& Path, int32 Count, int32 Bytes, TArray<uint8>& Out)
{
	if (!FFileHelper::LoadFileToArray(Out, *Path) || Out.Num() != Count * Bytes)
	{
		UE_LOG(LogWorldBuilder, Error, TEXT("%s: expected %d samples of %d bytes, found %d bytes"), *Path, Count,
			Bytes, Out.Num());
		return false;
	}
	return true;
}
} // namespace

ALandscape* UWorldBuilderLibrary::MakeLandscape(UWorld* World, const FString& HeightFile, int32 ComponentsX,
	int32 ComponentsY, int32 QuadsPerComponent, FVector Location, FVector Scale, UMaterialInterface* Material,
	const TArray<ULandscapeLayerInfoObject*>& Layers, const TArray<FString>& LayerFiles, int32 GridSize)
{
	if (!World || Layers.Num() != LayerFiles.Num())
	{
		UE_LOG(LogWorldBuilder, Error, TEXT("MakeLandscape: no world, or layers and files don't pair up"));
		return nullptr;
	}
	const int32 SizeX = ComponentsX * QuadsPerComponent + 1;
	const int32 SizeY = ComponentsY * QuadsPerComponent + 1;
	TArray<uint8> Raw;
	if (!ReadSamples(HeightFile, SizeX * SizeY, 2, Raw))
	{
		return nullptr;
	}
	TArray<uint16> Heights;
	Heights.SetNumUninitialized(SizeX * SizeY);
	FMemory::Memcpy(Heights.GetData(), Raw.GetData(), Raw.Num()); // little-endian, as Linux

	TArray<FLandscapeImportLayerInfo> Imports;
	for (int32 I = 0; I < Layers.Num(); ++I)
	{
		if (!Layers[I])
		{
			continue;
		}
		FLandscapeImportLayerInfo& Layer = Imports.Emplace_GetRef(Layers[I]->GetLayerName());
		Layer.LayerInfo = Layers[I];
		Layer.SourceFilePath = LayerFiles[I];
		if (!ReadSamples(LayerFiles[I], SizeX * SizeY, 1, Layer.LayerData))
		{
			return nullptr;
		}
	}
	TMap<FGuid, TArray<uint16>> HeightPerLayer;
	HeightPerLayer.Add(FGuid(), MoveTemp(Heights));
	TMap<FGuid, TArray<FLandscapeImportLayerInfo>> WeightPerLayer;
	WeightPerLayer.Add(FGuid(), Imports);

	ALandscape* Landscape = World->SpawnActor<ALandscape>(Location, FRotator::ZeroRotator);
	Landscape->SetActorRelativeScale3D(Scale);
	Landscape->LandscapeMaterial = Material;
	// The target layers come first: the import drops the weightmap of any layer the landscape
	// doesn't already list.
	for (int32 I = 0; I < Layers.Num(); ++I)
	{
		if (Layers[I])
		{
			Landscape->AddTargetLayer(Layers[I]->GetLayerName(), FLandscapeTargetLayerSettings(Layers[I], LayerFiles[I]));
		}
	}
	Landscape->Import(FGuid::NewGuid(), 0, 0, SizeX - 1, SizeY - 1, 1, QuadsPerComponent, HeightPerLayer, *HeightFile,
		WeightPerLayer, ELandscapeImportAlphamapType::Additive, TArrayView<const FLandscapeLayer>());
	ULandscapeInfo* Info = Landscape->GetLandscapeInfo();
	if (!Info)
	{
		UE_LOG(LogWorldBuilder, Error, TEXT("MakeLandscape: the import left no landscape info"));
		return nullptr;
	}
	Info->UpdateLayerInfoMap(Landscape);
	for (int32 I = 0; I < Layers.Num(); ++I)
	{
		if (Layers[I])
		{
			const int32 Index = Info->GetLayerInfoIndex(Layers[I]->GetLayerName());
			if (Index != INDEX_NONE)
			{
				Info->Layers[Index].LayerInfoObj = Layers[I];
			}
		}
	}
	// A large landscape can lose the GPU partway through these steps. Finish each step's GPU work
	// before the next is queued, and log each one with the free memory so a failure names its step.
	auto Step = [](const TCHAR* Name) {
		FlushRenderingCommands();
		const uint64 FreeMB = FPlatformMemory::GetStats().AvailablePhysical / (1024 * 1024);
		UE_LOG(LogWorldBuilder, Display, TEXT("MakeLandscape: %s done, %llu MB free"), Name, FreeMB);
	};
	Step(TEXT("import"));
	ULandscapeSubsystem* Subsystem = World->GetSubsystem<ULandscapeSubsystem>();
	if (Subsystem && GridSize > 0)
	{
		Subsystem->ChangeGridSize(Info, GridSize);
		Step(TEXT("grid"));
	}
	// The editor does these on its tick, which a script never gives it: render the edit layers into
	// the components' height and weight maps, give each component its material instance for the
	// layers it holds, and bake the grass maps, then mark every proxy for saving.
	Landscape->ForceUpdateLayersContent();
	Step(TEXT("edit layers"));
	Info->UpdateAllComponentMaterialInstances(true);
	Step(TEXT("material instances"));
	if (Subsystem)
	{
		using UE::Landscape::EBuildFlags;
		Subsystem->BuildGrassMaps(EBuildFlags::ForceRebuild | EBuildFlags::WriteFinalLog);
		Step(TEXT("grass maps"));
		Subsystem->MarkModifiedLandscapesAsDirty(EBuildFlags::None);
	}
	Info->ForEachLandscapeProxy([](ALandscapeProxy* Proxy) {
		Proxy->MarkPackageDirty();
		return true;
	});
	UE_LOG(LogWorldBuilder, Display, TEXT("MakeLandscape: %d x %d vertices, %d layers"), SizeX, SizeY, Imports.Num());
	return Landscape;
}

ULandscapeLayerInfoObject* UWorldBuilderLibrary::MakeLayerInfo(const FString& PackagePath, FName LayerName)
{
	const FName Name(*FPackageName::GetShortName(PackagePath));
	// A layer info from an earlier run is kept: the level's landscape may already refer to it.
	if (ULandscapeLayerInfoObject* Existing =
			LoadObject<ULandscapeLayerInfoObject>(nullptr, *(PackagePath + TEXT(".") + Name.ToString()), nullptr, LOAD_NoWarn))
	{
		return Existing;
	}
	UPackage* Package = CreatePackage(*PackagePath);
	ULandscapeLayerInfoObject* Info =
		NewObject<ULandscapeLayerInfoObject>(Package, Name, RF_Public | RF_Standalone | RF_Transactional);
	Info->SetLayerName(LayerName, false);
	Package->MarkPackageDirty();
	FSavePackageArgs Args;
	Args.TopLevelFlags = RF_Public | RF_Standalone;
	UPackage::SavePackage(Package, Info,
		*FPackageName::LongPackageNameToFilename(PackagePath, FPackageName::GetAssetPackageExtension()), Args);
	return Info;
}

TArray<FString> UWorldBuilderLibrary::MaterialErrors(UMaterial* Material)
{
	if (!Material)
	{
		return {TEXT("no material")};
	}
	Material->ForceRecompileForRendering();
	if (GShaderCompilingManager)
	{
		GShaderCompilingManager->FinishAllCompilation();
	}
	const FMaterialResource* Resource = Material->GetMaterialResource(GMaxRHIShaderPlatform);
	if (!Resource)
	{
		return {TEXT("no material resource for this platform")};
	}
	return Resource->GetCompileErrors();
}

TArray<FString> UWorldBuilderLibrary::DescribeLandscape(UWorld* World, int32 MaxComponents)
{
	TArray<FString> Lines;
	if (!World)
	{
		return Lines;
	}
	for (TActorIterator<ALandscapeProxy> It(World); It && Lines.Num() < MaxComponents; ++It)
	{
		for (ULandscapeComponent* Component : It->LandscapeComponents)
		{
			if (!Component || Lines.Num() >= MaxComponents)
			{
				continue;
			}
			FString Line = FString::Printf(TEXT("%s %s: %d instances"), *It->GetName(), *Component->GetName(),
				Component->MaterialInstances.Num());
			for (UMaterialInstanceConstant* Instance : Component->MaterialInstances)
			{
				Line += FString::Printf(TEXT(" [%s <- %s]"), Instance ? *Instance->GetName() : TEXT("null"),
					Instance && Instance->Parent ? *Instance->Parent->GetPathName() : TEXT("null"));
			}
			Line += TEXT("; layers");
			for (const FWeightmapLayerAllocationInfo& Allocation : Component->GetWeightmapLayerAllocations())
			{
				Line += TEXT(" ") + (Allocation.LayerInfo ? Allocation.LayerInfo->GetLayerName().ToString() : FString(TEXT("none")));
			}
			Line += FString::Printf(TEXT("; %d weightmaps"), Component->GetWeightmapTextures().Num());
			Lines.Add(Line);
		}
	}
	return Lines;
}
