using UnrealBuildTool;

public class OpenCycleUETarget : TargetRules
{
	public OpenCycleUETarget(TargetInfo Target) : base(Target)
	{
		Type = TargetType.Game;
		DefaultBuildSettings = BuildSettingsVersion.Latest;
		IncludeOrderVersion = EngineIncludeOrderVersion.Latest;
		ExtraModuleNames.Add("OpenCycleUE");
	}
}
