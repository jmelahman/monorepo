using UnrealBuildTool;

public class VeloverseUE : ModuleRules
{
	public VeloverseUE(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;
		PublicDependencyModuleNames.AddRange(new[] { "Core", "CoreUObject", "Engine", "InputCore", "UMG", "Slate", "SlateCore", "RHI", "RenderCore" });
	}
}
