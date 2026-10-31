extends Node
## Development-only hooks, inert in any build a player can get.
##
## Today that is the agent bridge ([code]addons/agent_bridge[/code]), a debug
## server that lets a script or an agent drive the running game. It starts only
## when all of these hold: a debug build, the addon's files present, and an
## explicit request, [code]--agent-bridge[/code] after [code]--[/code] on the
## command line or [code]AGENT_BRIDGE_PORT[/code] in the environment. Release
## presets also exclude the addon, so a shipped game has neither the flag's
## effect nor the code it would load.

const BRIDGE := "res://addons/agent_bridge/bridge.gd"


func _ready() -> void:
	if not OS.is_debug_build():
		return
	var requested := (
		"--agent-bridge" in OS.get_cmdline_user_args() or OS.has_environment("AGENT_BRIDGE_PORT")
	)
	if not requested:
		return
	if not ResourceLoader.exists(BRIDGE):
		push_warning("dev: --agent-bridge requested but %s is not in this build" % BRIDGE)
		return
	var script: GDScript = load(BRIDGE)
	var bridge: Node = script.new()
	bridge.name = "AgentBridge"
	add_child(bridge)
