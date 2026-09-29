class_name WildShell
extends RefCounted
## The whole game as the web build plays it, minus the browser:
## [code]js/shell.ts[/code], which is [code]src/ui/app.ts[/code]'s controller
## half running [code]src/ui/views.ts[/code] against a fake DOM, bundled by
## [code]scripts/bundle.sh[/code] and run in QuickJS.
##
## [Wild] is the engine alone and stays what the golden tests drive. This is
## what the game drives: it answers every "what happens when" the way the web
## build does (which screen a phase gets, what a tap dispatches, what the
## record counts), and hands back the screen as a tree for
## [code]game/render.gd[/code] to draw. Godot never holds the run, only the
## store it lives in; see [method flush].
##
## Calls that take input return [i]effects[/i], a Dictionary saying what to do
## about it: [code]render[/code], [code]cues[/code], [code]toast[/code],
## [code]shake[/code], [code]bump[/code], [code]animate[/code],
## [code]open[/code]. See [code]Effects[/code] in [code]js/shell.ts[/code].

const BUNDLE := "res://js/build/shell.js"
const WORDS := "res://words"

var _js := JsContext.new()


## A fresh shell with [param bundle] loaded, or null with the error pushed.
static func open(bundle := BUNDLE) -> WildShell:
	var shell := WildShell.new()
	var source := FileAccess.get_file_as_string(bundle)
	if source.is_empty():
		push_error("shell: cannot read %s (run scripts/bundle.sh)" % bundle)
		return null
	if not shell._js.load(source, bundle.get_file()):
		push_error("shell: %s failed to load: %s" % [bundle, shell._js.get_error()])
		return null
	return shell


## Hands the shell the web build's storage, as [code]{key: text}[/code], and
## the device's look, then feeds it whatever word lists it asks for. The keys
## are the web build's ([code]5wild:run:v2[/code] and the rest; see 5-wild's
## CLAUDE.md), so the two builds read a record the same way.
func boot(items: Dictionary, device_theme: String) -> Dictionary:
	_call("load", [JSON.stringify(items)])
	feed()
	return _parse(_call("boot", [device_theme]))


## Reads every list the shell says it needs before its next input. A language
## change asks for the next run's list, and reading one is a millisecond, so
## this runs after every call rather than ahead of the one that needs it.
func feed() -> void:
	var wanted: Variant = JSON.parse_string(_call("wanted"))
	if wanted is not Array:
		return
	for lang: String in wanted:
		var dir := WORDS.path_join(lang)
		var answers := FileAccess.get_file_as_string(dir.path_join("answers.txt"))
		var allowed := FileAccess.get_file_as_string(dir.path_join("allowed.txt"))
		if answers.is_empty() or allowed.is_empty():
			push_error("shell: no word lists in %s (run scripts/bundle.sh)" % dir)
			continue
		_call("words", [lang, answers, allowed])


## The screen, as [code]{screen, sheet, theme, decor, speed, lang, muted,
## musicOff, track, busy, cues}[/code]. [code]screen[/code] and
## [code]sheet[/code] are trees; see [code]serialize[/code] in
## [code]js/dom-shim.ts[/code].
func render() -> Dictionary:
	return _parse(_call("render"))


## A click on the node the last [method render] numbered [param id].
func click(id: int) -> Dictionary:
	return _after(_call("click", [str(id)]))


## [code]enter[/code], [code]back[/code], [code]escape[/code] or a letter a–z.
func key(name: String) -> Dictionary:
	return _after(_call("key", [name]))


## The scoring animation is over.
func settle() -> Dictionary:
	return _after(_call("settle"))


## The store if anything was written since the last ask, else an empty
## Dictionary. Godot writes it to disk; the shell never learns a disk exists.
func flush() -> Dictionary:
	return _parse(_call("flush"))


## The run as the engine holds it, for tests and the agent bridge.
func state() -> Dictionary:
	return _parse(_call("state"))


## For tests: a run from a known seed, past the title.
func start_seeded(run_seed: int) -> Dictionary:
	return _after(_call("startSeeded", [str(run_seed)]))


## [param value] as the language in force writes a number, for the count-up.
func num(value: float) -> String:
	return _call("num", [str(roundi(value))])


func _after(text: String) -> Dictionary:
	feed()
	return _parse(text)


func _call(fn: String, args: PackedStringArray = []) -> String:
	var out := _js.invoke("fivewildShell." + fn, args)
	var error := _js.get_error()
	if error != "":
		push_error("shell: %s: %s" % [fn, error])
	return out


func _parse(text: String) -> Dictionary:
	var parsed: Variant = JSON.parse_string(text) if text != "" else null
	return parsed if parsed is Dictionary else {}
