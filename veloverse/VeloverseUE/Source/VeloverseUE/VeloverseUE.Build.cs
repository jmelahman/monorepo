using UnrealBuildTool;

public class VeloverseUE : ModuleRules
{
	public VeloverseUE(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;
		// Each file keeps its drawing helpers in an anonymous namespace, and some share names.
		bUseUnity = false;
		PublicDependencyModuleNames.AddRange(new[] { "Core", "CoreUObject", "Engine", "InputCore", "UMG", "Slate", "SlateCore", "RHI", "RenderCore", "Json" });
	}
}
