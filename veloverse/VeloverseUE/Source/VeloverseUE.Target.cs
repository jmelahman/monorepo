using UnrealBuildTool;

public class VeloverseUETarget : TargetRules
{
	public VeloverseUETarget(TargetInfo Target) : base(Target)
	{
		Type = TargetType.Game;
		DefaultBuildSettings = BuildSettingsVersion.Latest;
		IncludeOrderVersion = EngineIncludeOrderVersion.Latest;
		ExtraModuleNames.Add("VeloverseUE");
	}
}
