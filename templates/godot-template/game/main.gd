extends Control
## The presentation half: owns the current [RunState], turns input into
## actions, runs them through [method Rules.reduce], and shows what came back.
## It decides nothing about the rules. Everything it says is looked up with
## [method Object.tr], so the sim's codes become sentences here and only here.
##
## The flow is one-way, and worth keeping so as the game grows:
## input -> [method dispatch] -> reduce -> save -> [method _render].
## [method dispatch] is public so tests and the agent bridge drive the game the
## same way a player does.

const SLOT := "run"
## Spacewar's first achievement (see [constant Platform.APP_ID]); swap in the
## game's own API names from Steamworks.
const ACH_FIRST_WIN := "ACH_WIN_ONE_GAME"
const LOCALES: Array[String] = ["", "en", "es"]

var state: RunState
## What the last action did, kept so a language switch can re-say it.
var _last_events: Array[Dictionary] = []

@onready var _score: Label = %Score
@onready var _pot: Label = %Pot
@onready var _message: Label = %Message
@onready var _roll: Button = %Roll
@onready var _bank: Button = %Bank
@onready var _new_game: Button = %NewGame
@onready var _options: Control = %Options
@onready var _resume: Button = %Resume
@onready var _music: HSlider = %Music
@onready var _sfx: HSlider = %Sfx
@onready var _fullscreen: CheckBox = %Fullscreen
@onready var _language: OptionButton = %Language


func _ready() -> void:
	_roll.pressed.connect(dispatch.bind({"type": "roll"}))
	_bank.pressed.connect(dispatch.bind({"type": "bank"}))
	_new_game.pressed.connect(new_game)
	_resume.pressed.connect(_toggle_options)
	_bind_options()
	Settings.changed.connect(_on_setting_changed)
	var saved := Saves.read(SLOT)
	if saved.is_empty():
		new_game()
	else:
		state = RunState.from_dict(saved)
		_render()


## _input rather than _unhandled_input, so the hotkeys work whatever has focus.
## The board's buttons take no focus at all (focus_mode in the scene): Space is
## both `roll` and `ui_accept`, and a focused Roll button made one keypress two
## rolls.
func _input(event: InputEvent) -> void:
	if event.is_action_pressed("pause"):
		_toggle_options()
	elif _options.visible:
		return
	elif event.is_action_pressed("roll"):
		dispatch({"type": "roll"})
	elif event.is_action_pressed("bank"):
		dispatch({"type": "bank"})
	else:
		return
	get_viewport().set_input_as_handled()


func dispatch(action: Dictionary) -> void:
	var step := Rules.reduce(state, action)
	state = step.state
	_last_events = step.events
	for event: Dictionary in step.events:
		if event["type"] == &"won":
			Platform.unlock(ACH_FIRST_WIN)
	Saves.write(SLOT, state.to_dict())
	_render()


func new_game() -> void:
	# The seed is the one random thing the presentation layer is allowed; it
	# goes into the run, and everything after is derived from it.
	state = RunState.start(randi())
	_last_events = []
	Saves.write(SLOT, state.to_dict())
	_render()


func _render() -> void:
	_score.text = tr("Score: %d / %d") % [state.score, Rules.TARGET]
	_pot.text = tr("Pot: %d") % state.pot
	var lines: PackedStringArray = []
	for event: Dictionary in _last_events:
		lines.append(_describe(event))
	_message.text = (
		" ".join(lines) if lines else tr("Roll to fill the pot. A 1 loses it. Bank to keep it.")
	)
	_roll.visible = not state.won
	_bank.visible = not state.won
	_bank.disabled = state.pot == 0
	_new_game.visible = state.won


func _describe(event: Dictionary) -> String:
	match event["type"]:
		&"rolled":
			return tr("You rolled %d.") % event["value"]
		&"busted":
			return tr("Bust! The pot of %d is gone.") % event["lost"]
		&"banked":
			return tr("Banked %d.") % event["amount"]
		&"won":
			return tr("You won in %d turns!") % event["turns"]
		&"refused":
			return _refusal(event)
	return ""


func _refusal(event: Dictionary) -> String:
	match event["code"]:
		&"empty_pot":
			return tr("Nothing to bank yet.")
		&"game_over":
			return tr("This game is over.")
	return tr("That does nothing.")


## Pausing stops whatever is pausable (tweens, timers, physics, once there
## are some); this node is PROCESS_MODE_ALWAYS in the scene so the options it
## opens still take input.
func _toggle_options() -> void:
	_options.visible = not _options.visible
	get_tree().paused = _options.visible
	if _options.visible:
		_music.grab_focus.call_deferred()
	else:
		_render()


func _bind_options() -> void:
	_show_settings()
	_music.value_changed.connect(func(v: float) -> void: Settings.set_value("audio/music", v))
	_sfx.value_changed.connect(func(v: float) -> void: Settings.set_value("audio/sfx", v))
	_fullscreen.toggled.connect(
		func(on: bool) -> void: Settings.set_value("display/fullscreen", on)
	)
	_language.item_selected.connect(
		func(i: int) -> void: Settings.set_value("game/locale", LOCALES[i])
	)


## Settings can change from elsewhere (F11 one day, the agent bridge today), so
## the controls follow Settings rather than remembering what they last sent.
## The no_signal setters are what keep that from echoing back into Settings.
func _show_settings() -> void:
	_music.set_value_no_signal(Settings.get_float("audio/music"))
	_sfx.set_value_no_signal(Settings.get_float("audio/sfx"))
	_fullscreen.set_pressed_no_signal(Settings.get_bool("display/fullscreen"))
	# select() does not emit item_selected.
	_language.select(maxi(LOCALES.find(Settings.get_string("game/locale")), 0))


func _on_setting_changed(key: String, _value: Variant) -> void:
	_show_settings()
	# Nodes with static text retranslate themselves; what _render wrote does not.
	if key == "game/locale":
		_render()
