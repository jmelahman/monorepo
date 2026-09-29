extends Node
## The storefront, which today means Steam and may one day mean none. Game code
## calls [method unlock] and never learns which: without the GodotSteam
## extension (addons/godotsteam is fetched by [code]scripts/setup.sh[/code] and
## gitignored), without a running Steam client, or in a test run, every call
## here is a quiet no-op and the game plays on.
##
## Not named Steam: GodotSteam registers an engine singleton by that name, and
## an autoload may not shadow one. It is reached through
## [method Engine.get_singleton] rather than the global identifier for the same
## reason the extension is optional: a script naming [code]Steam[/code]
## directly fails to parse on a checkout that has not fetched it.

## Spacewar, Valve's public test app: overlay and achievements work against it
## with no Steamworks account. Replace with the game's own id before release;
## a Steam launch also reads it from the depot, so it matters most for runs
## started outside Steam (the editor, the scripts).
const APP_ID := 480

var _steam: Object


func _ready() -> void:
	if OS.has_environment("GODOT_TEST") or DisplayServer.get_name() == "headless":
		return
	if not Engine.has_singleton("Steam"):
		print_verbose("platform: GodotSteam not installed; running without Steam")
		return
	var steam := Engine.get_singleton("Steam")
	# embed_callbacks: GodotSteam polls Steam's callbacks every frame itself,
	# rather than this node calling run_callbacks() from _process.
	var result: Dictionary = steam.call("steamInitEx", APP_ID, true)
	# 0 is k_ESteamAPIInitResult_OK; anything else (no client, wrong user, an
	# outdated client) leaves the game playable offline.
	var status: int = result.get("status", -1)
	if status != 0:
		print("platform: Steam unavailable (%s)" % result.get("verbal", "unknown"))
		return
	_steam = steam


func is_online() -> bool:
	return _steam != null


## Display name of the signed-in player, or "" offline.
func player_name() -> String:
	return str(_steam.call("getPersonaName")) if _steam else ""


## Unlocks the achievement with API name [param id] (as configured in
## Steamworks). Idempotent, so call it every time the condition holds.
func unlock(id: String) -> void:
	if _steam == null:
		return
	if not _steam.call("setAchievement", id):
		push_warning("platform: unknown achievement %s" % id)
		return
	# Nothing reaches Steam's servers (or the overlay's popup) until stored.
	_steam.call("storeStats")
