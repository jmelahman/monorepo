using UnrealBuildTool;

public class VeloverseUEEditorTarget : TargetRules
{
	public VeloverseUEEditorTarget(TargetInfo Target) : base(Target)
	{
		Type = TargetType.Editor;
		DefaultBuildSettings = BuildSettingsVersion.Latest;
		IncludeOrderVersion = EngineIncludeOrderVersion.Latest;
		ExtraModuleNames.AddRange(new[] { "VeloverseUE", "VeloverseEditor" });
	}
}
