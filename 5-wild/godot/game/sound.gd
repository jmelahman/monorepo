class_name Sound
extends Node
## 5-wild's sound, as [code]src/ui/audio.ts[/code] and [code]music.ts[/code]
## play it: the same recordings, at the same levels, repitched the same way, and
## the three cues that were never recorded synthesized from the same recipes.
##
## The cues arrive from the shell exactly as the web's [code]Sound.cue[/code]
## receives them ([code]{name: "tile", index: 2, color: "green"}[/code]), so the
## mapping from a cue to what is heard is the part ported here, and the tables
## below are copies of that file's. When one of them changes there, it changes
## here; the comments that justify each number stay there, with the numbers'
## history, rather than being split across two files.
##
## The recordings are copied from [code]src/ui/sounds[/code] and
## [code]src/ui/tracks[/code] by [code]scripts/bundle.sh[/code].

const SOUNDS := "res://audio/sounds"
const TRACKS := "res://audio/tracks"

## [code]LEVELS[/code] in audio.ts: linear gain by file name.
const LEVELS := {
	"key": 0.05,
	"back": 0.035,
	"reject": 0.12,
	"tile": 0.12,
	"trigger": 0.12,
	"score": 0.14,
	"solve": 0.15,
	"break": 0.14,
	"sheet-open": 0.06,
	"sheet-close": 0.05,
	"intro-round": 0.05,
	"intro-boss": 0.18,
	"reroll-shop": 0.07,
	"win": 0.16,
	"lose": 0.16,
}
const DEFAULT_LEVEL := 0.12
## [code]DISTANT[/code]: recordings heard across the room, through a low-pass.
const DISTANT := {"reroll-shop": 2800.0}
const PENTATONIC: Array[int] = [0, 2, 4, 7, 9]
const LADDER_TOP := 9
## [code]TRACKS[/code] in music.ts: the level each recording plays at.
const TRACK_LEVELS := {"promises": 0.17, "forget-me-not": 0.1}
const TRACK_FADE := 1.5
const SWITCH_FADE := 0.3

const C5 := 523.25
const RATE := 44100
const VOICES := 12

var muted := false

var _samples: Dictionary[String, AudioStream] = {}
var _synth: Dictionary[String, AudioStreamWAV] = {}
var _voices: Array[AudioStreamPlayer] = []
var _next := 0
var _far_bus := "SFX"
var _music: AudioStreamPlayer
var _track := ""
var _music_on := false
var _fade: Tween


func _ready() -> void:
	process_mode = Node.PROCESS_MODE_ALWAYS
	for file: String in _list(SOUNDS):
		var stream := load(SOUNDS.path_join(file)) as AudioStream
		if stream != null:
			_samples[file.get_basename()] = stream
	for i: int in VOICES:
		var voice := AudioStreamPlayer.new()
		voice.bus = "SFX"
		add_child(voice)
		_voices.append(voice)
	_music = AudioStreamPlayer.new()
	_music.bus = "Music"
	add_child(_music)
	_far_bus = _make_far_bus()
	_synth["solve"] = _render_solve()
	_synth["break"] = _render_break()
	_synth["consume"] = _render_consume()


## Plays one cue from the shell.
func cue(c: Dictionary) -> void:
	if muted:
		return
	var cue_name: String = c.get("name", "")
	var tag := _variant(c)
	var specific := "%s-%s" % [cue_name, tag] if tag != "" else ""
	var file := specific if specific != "" and _samples.has(specific) else cue_name
	var stream: AudioStream = _samples.get(file)
	var gain: float = LEVELS.get(file, DEFAULT_LEVEL)
	var pitch := pow(2.0, _transpose(c) / 12.0)
	if stream == null:
		stream = _synth.get(cue_name)
		# The synthesized cues carry their own voice gains, as the recipes do.
		gain = 1.0
		pitch = 1.0
	if stream == null:
		return
	var voice := _voices[_next]
	_next = (_next + 1) % _voices.size()
	voice.stream = stream
	voice.volume_db = linear_to_db(gain)
	voice.pitch_scale = pitch
	voice.bus = StringName(_far_bus if DISTANT.has(file) else "SFX")
	voice.play()


## What the render says about sound: the effects' mute, and which recording
## plays, if any. A switch fades the old one out quickly and the new one in
## slowly, as music.ts does.
func music(on: bool, track: String) -> void:
	if on == _music_on and track == _track:
		return
	var switching := _music_on and on and track != _track
	_music_on = on
	_track = track
	if _fade != null:
		_fade.kill()
	_fade = create_tween()
	if not on:
		_fade.tween_property(_music, "volume_db", -60.0, SWITCH_FADE)
		_fade.tween_callback(_music.stop)
		return
	if switching:
		_fade.tween_property(_music, "volume_db", -60.0, SWITCH_FADE)
	_fade.tween_callback(_start_track.bind(track))
	var level: float = TRACK_LEVELS.get(track, 0.1)
	_fade.tween_property(_music, "volume_db", linear_to_db(level), TRACK_FADE)


func _start_track(track: String) -> void:
	var stream := load(TRACKS.path_join(track + ".ogg")) as AudioStreamOggVorbis
	if stream == null:
		return
	stream.loop = true
	_music.stream = stream
	_music.volume_db = -60.0
	_music.play()


static func _variant(c: Dictionary) -> String:
	match c.get("name"):
		"tile":
			return str(c.get("color", ""))
		"trigger":
			return str(c.get("kind", ""))
		"sheet":
			return "open" if c.get("open", false) else "close"
		"intro":
			return "boss" if c.get("boss", false) else "round"
		"reroll":
			return "shop" if c.get("shop", false) else ""
		"win":
			return "run" if c.get("run", false) else ""
	return ""


static func _transpose(c: Dictionary) -> float:
	match c.get("name"):
		"tile":
			var index: int = type_convert(c.get("index", 0), TYPE_INT)
			return PENTATONIC[index] if index < PENTATONIC.size() else 0
		"trigger":
			var fired: int = type_convert(c.get("n", 0), TYPE_INT)
			var n := mini(fired, LADDER_TOP)
			return 12 * floori(n / 5.0) + PENTATONIC[n % 5]
		"score":
			var ratio: float = type_convert(c.get("ratio", 0.0), TYPE_FLOAT)
			return mini(12, roundi(ratio * 6.0))
	return 0.0


## Resource names in [param dir], as an export sees them too: there the files
## are remapped, and listing the folder would show [code].import[/code] stubs.
static func _list(dir: String) -> PackedStringArray:
	var out: PackedStringArray = []
	if not DirAccess.dir_exists_absolute(dir):
		return out
	for file: String in ResourceLoader.list_directory(dir):
		if file.get_extension() in ["ogg", "wav", "mp3"]:
			out.append(file)
	return out


func _make_far_bus() -> String:
	var bus := "Far"
	if AudioServer.get_bus_index(bus) < 0:
		AudioServer.add_bus()
		var index := AudioServer.bus_count - 1
		AudioServer.set_bus_name(index, bus)
		AudioServer.set_bus_send(index, "SFX")
		var low := AudioEffectLowPassFilter.new()
		low.cutoff_hz = DISTANT["reroll-shop"]
		AudioServer.add_bus_effect(index, low)
	return bus


# ------------------------------------------------------------------ synth
#
# The recipes in audio.ts, rendered once at start into buffers. The web builds
# them live from oscillators and filters because Web Audio makes that free;
# here a buffer is the natural unit, and the three cues never vary.


static func _step(base: float, semitones: float) -> float:
	return base * pow(2.0, semitones / 12.0)


func _render_solve() -> AudioStreamWAV:
	var out := PackedFloat32Array()
	out.resize(int(RATE * 1.3))
	var notes: Array[int] = [0, 4, 7, 12, 16]
	for i: int in notes.size():
		var ms := 900.0 if i == notes.size() - 1 else 300.0
		_bell(out, _step(C5, notes[i]), ms, 0.05, i * 65.0)
	_noise(
		out,
		{"ms": 500.0, "band": 7000.0, "kind": "high", "gain": 0.012, "delay": 260.0, "attack": 80.0}
	)
	return _wav(out)


func _render_break() -> AudioStreamWAV:
	var out := PackedFloat32Array()
	out.resize(int(RATE * 0.35))
	_noise(out, {"ms": 220.0, "band": 2200.0, "to": 300.0, "q": 0.7, "gain": 0.06})
	_tone(out, _step(C5, -5), _step(C5, -17), 280.0, "saw", 0.04, 1400.0, 250.0)
	return _wav(out)


func _render_consume() -> AudioStreamWAV:
	var out := PackedFloat32Array()
	out.resize(int(RATE * 0.35))
	_tone(out, C5, _step(C5, 19), 280.0, "sine", 0.03, 0.0, 0.0)
	_noise(out, {"ms": 260.0, "band": 6000.0, "to": 9500.0, "q": 0.8, "gain": 0.012})
	return _wav(out)


## A struck bell: the note and an inharmonic partial at 2.76 times it, which is
## what makes a sine sound struck rather than blown, dying faster than it.
func _bell(out: PackedFloat32Array, freq: float, ms: float, gain: float, delay: float) -> void:
	var start := int(delay / 1000.0 * RATE)
	var length := int(ms / 1000.0 * RATE)
	for i: int in length:
		if start + i >= out.size():
			break
		var t := float(i) / RATE
		var attack := minf(1.0, t / 0.004)
		var body := exp(-5.0 * float(i) / length)
		var partial := exp(-5.0 * float(i) / (length * 0.4))
		out[start + i] += (
			gain
			* attack
			* (sin(TAU * freq * t) * body + 0.3 * sin(TAU * freq * 2.76 * t) * partial)
		)


## A pitched voice gliding from [param freq] to [param to], optionally through
## a low-pass sweeping from [param lp] to [param lp_to].
func _tone(
	out: PackedFloat32Array,
	freq: float,
	to: float,
	ms: float,
	kind: String,
	gain: float,
	lp: float,
	lp_to: float
) -> void:
	var length := int(ms / 1000.0 * RATE)
	var phase := 0.0
	var filter := _Biquad.new()
	for i: int in mini(length, out.size()):
		var k := float(i) / length
		var f := freq * pow(to / freq, k)
		phase = fmod(phase + f / RATE, 1.0)
		var s := sin(TAU * phase) if kind == "sine" else 2.0 * phase - 1.0
		if lp > 0.0:
			if i % 32 == 0:
				filter.lowpass(lp * pow(lp_to / lp, k), 0.707)
			s = filter.run(s)
		var env := minf(1.0, i / (RATE * 0.005)) * pow(1.0 - k, 2.0)
		out[i] += gain * env * s


func _noise(out: PackedFloat32Array, spec: Dictionary) -> void:
	var ms: float = spec["ms"]
	var band: float = spec["band"]
	var to: float = spec.get("to", band)
	var q: float = spec.get("q", 0.707)
	var gain: float = spec["gain"]
	var delay: float = spec.get("delay", 0.0)
	var start := int(delay / 1000.0 * RATE)
	var attack_ms: float = spec.get("attack", 5.0)
	var attack := attack_ms / 1000.0 * RATE
	var length := int(ms / 1000.0 * RATE)
	var filter := _Biquad.new()
	# Seeded, so the cue is the same hiss on every launch.
	var rng := RandomNumberGenerator.new()
	rng.seed = 5
	for i: int in length:
		if start + i >= out.size():
			break
		var k := float(i) / length
		if i % 32 == 0:
			var f := band * pow(to / band, k)
			if spec.get("kind", "band") == "high":
				filter.highpass(f, q)
			else:
				filter.bandpass(f, q)
		var env := minf(1.0, i / maxf(attack, 1.0)) * pow(1.0 - k, 2.0)
		out[start + i] += gain * env * filter.run(rng.randf_range(-1.0, 1.0)) * 4.0


static func _wav(samples: PackedFloat32Array) -> AudioStreamWAV:
	var data := PackedByteArray()
	data.resize(samples.size() * 2)
	for i: int in samples.size():
		data.encode_s16(i * 2, clampi(roundi(samples[i] * 32767.0), -32768, 32767))
	var wav := AudioStreamWAV.new()
	wav.format = AudioStreamWAV.FORMAT_16_BITS
	wav.mix_rate = RATE
	wav.stereo = false
	wav.data = data
	return wav


## A biquad filter (RBJ cookbook), for the recipes' sweeps.
class _Biquad:
	var b0 := 1.0
	var b1 := 0.0
	var b2 := 0.0
	var a1 := 0.0
	var a2 := 0.0
	var x1 := 0.0
	var x2 := 0.0
	var y1 := 0.0
	var y2 := 0.0

	func lowpass(freq: float, q: float) -> void:
		var w := TAU * minf(freq, RATE * 0.45) / RATE
		var alpha := sin(w) / (2.0 * q)
		var c := cos(w)
		_coeffs((1.0 - c) / 2.0, 1.0 - c, (1.0 - c) / 2.0, 1.0 + alpha, -2.0 * c, 1.0 - alpha)

	func highpass(freq: float, q: float) -> void:
		var w := TAU * minf(freq, RATE * 0.45) / RATE
		var alpha := sin(w) / (2.0 * q)
		var c := cos(w)
		_coeffs((1.0 + c) / 2.0, -(1.0 + c), (1.0 + c) / 2.0, 1.0 + alpha, -2.0 * c, 1.0 - alpha)

	func bandpass(freq: float, q: float) -> void:
		var w := TAU * minf(freq, RATE * 0.45) / RATE
		var alpha := sin(w) / (2.0 * q)
		var c := cos(w)
		_coeffs(alpha, 0.0, -alpha, 1.0 + alpha, -2.0 * c, 1.0 - alpha)

	func run(x: float) -> float:
		var y := b0 * x + b1 * x1 + b2 * x2 - a1 * y1 - a2 * y2
		x2 = x1
		x1 = x
		y2 = y1
		y1 = y
		return y

	func _coeffs(nb0: float, nb1: float, nb2: float, na0: float, na1: float, na2: float) -> void:
		b0 = nb0 / na0
		b1 = nb1 / na0
		b2 = nb2 / na0
		a1 = na1 / na0
		a2 = na2 / na0
