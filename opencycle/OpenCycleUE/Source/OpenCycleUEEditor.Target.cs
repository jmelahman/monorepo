using UnrealBuildTool;

public class OpenCycleUEEditorTarget : TargetRules
{
	public OpenCycleUEEditorTarget(TargetInfo Target) : base(Target)
	{
		Type = TargetType.Editor;
		DefaultBuildSettings = BuildSettingsVersion.Latest;
		IncludeOrderVersion = EngineIncludeOrderVersion.Latest;
		ExtraModuleNames.Add("OpenCycleUE");
	}
}
