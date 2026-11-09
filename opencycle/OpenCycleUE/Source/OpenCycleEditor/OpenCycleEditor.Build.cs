using UnrealBuildTool;

// Editor-only helpers the build scripts call from Python: what the editor's own tools do through
// their UI, such as making a landscape from a heightmap.
public class OpenCycleEditor : ModuleRules
{
	public OpenCycleEditor(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;
		PublicDependencyModuleNames.AddRange(new[] { "Core", "CoreUObject", "Engine", "Landscape" });
		PrivateDependencyModuleNames.AddRange(new[] { "UnrealEd", "AssetTools", "RHI", "RenderCore" });
	}
}
