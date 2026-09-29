class_name Wild
extends RefCounted
## The 5-wild engine: [code]src/engine[/code], the TypeScript the web build
## runs, bundled by [code]scripts/bundle.sh[/code] and run in QuickJS through
## the [JsContext] class in [code]native/[/code].
##
## Nothing about the rules lives on this side. A run goes in as actions and
## comes back as [code]{state, events}[/code] dictionaries to read; the run
## itself stays in JS between calls (see [code]js/engine.ts[/code] for why), and
## [method save_text] and [method resume] are the only way it crosses as a whole.
## Every number in what comes back is a float, because JSON has one number type.

const BUNDLE := "res://js/build/engine.js"
const WORDS := "res://words"

var _js := JsContext.new()


## A fresh engine with [param bundle] loaded, or null with the error pushed.
static func open(bundle := BUNDLE) -> Wild:
	var wild := Wild.new()
	var source := FileAccess.get_file_as_string(bundle)
	if source.is_empty():
		push_error("wild: cannot read %s (run scripts/bundle.sh)" % bundle)
		return null
	if not wild._js.load(source, bundle.get_file()):
		push_error("wild: %s failed to load: %s" % [bundle, wild._js.get_error()])
		return null
	return wild


## Deals from [param lang]'s lists ([code]en[/code], [code]es[/code],
## [code]fr[/code], [code]de[/code]) from the next [method start] on. A run in
## progress keeps its answer; see "5wild:run:lang" in 5-wild's CLAUDE.md for
## why the web build never switches under one, and the same holds here.
func set_language(lang: String) -> bool:
	var dir := WORDS.path_join(lang)
	var answers := FileAccess.get_file_as_string(dir.path_join("answers.txt"))
	var allowed := FileAccess.get_file_as_string(dir.path_join("allowed.txt"))
	if answers.is_empty() or allowed.is_empty():
		push_error("wild: no word lists in %s (run scripts/bundle.sh)" % dir)
		return false
	return _call("setWords", [answers, allowed]) != ""


func content_version() -> int:
	return _call("contentVersion").to_int()


## Starts a run. Returns [code]{state, events}[/code], as [method dispatch] does.
func start(run_seed: int, ascension := 0) -> Dictionary:
	return _parse(_call("start", [str(run_seed), str(ascension)]))


## One action, e.g. [code]{"type": "type_letter", "letter": "A"}[/code]; the
## shapes are [code]Action[/code] in [code]src/engine/state.ts[/code]. A
## refusal is not a failure: it comes back as a [code]rejected[/code] event with
## the state unchanged, as it does on the web.
func dispatch(action: Dictionary) -> Dictionary:
	return _parse(_call("dispatch", [JSON.stringify(action)]))


## The run in progress as JSON text, for a save. Kept as text rather than a
## Dictionary so it goes back into [method resume] exactly as it came out.
func save_text() -> String:
	return _call("state")


func resume(text: String) -> bool:
	return _call("resume", [text]) != ""


func _call(fn: String, args: PackedStringArray = []) -> String:
	var out := _js.invoke("fivewild." + fn, args)
	var error := _js.get_error()
	if error != "":
		push_error("wild: %s: %s" % [fn, error])
	return out


func _parse(text: String) -> Dictionary:
	var parsed: Variant = JSON.parse_string(text) if text != "" else null
	return parsed if parsed is Dictionary else {}
