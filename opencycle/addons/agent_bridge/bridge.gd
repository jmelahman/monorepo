extends Node
## A debug server for driving the running game from outside it: a test script,
## a screenshot job, or a coding agent checking its own work. Started by
## [code]core/dev.gd[/code], and only in a debug build that asked for it.
##
## The protocol is one JSON object per line over TCP on 127.0.0.1, one request
## at a time per connection:
## [codeblock]
## -> {"id": 1, "cmd": "get", "args": {"path": "/root/Main/Score", "prop": "text"}}
## <- {"id": 1, "ok": true, "result": "Score: 0"}
## [/codeblock]
## [code]scripts/bridge.py[/code] is the client, and [method _commands] is the
## list. Values that JSON cannot carry (vectors, colors, nodes) come back as
## [method @GlobalScope.var_to_str] text.
##
## It binds loopback only and has no authentication, so treat [code]eval[/code]
## as what it is: any process on this machine can run code in the game while
## the bridge is up. That is the trade for being useful, and why it is opt-in,
## debug-only and excluded from every export preset.
##
## Written for this template rather than vendored: the MCP servers that exist
## are thousands of lines, start with the game, and ship in release builds
## unless someone remembers otherwise. This one is small enough to read.

const DEFAULT_PORT := 9877

var _server := TCPServer.new()
var _clients: Array[_Client] = []


class _Client:
	var peer: StreamPeerTCP
	var buffer := PackedByteArray()
	var busy := false

	func _init(p: StreamPeerTCP) -> void:
		peer = p


func _ready() -> void:
	# Keep answering while the game is paused; a paused menu is exactly what
	# an agent may want to inspect.
	process_mode = Node.PROCESS_MODE_ALWAYS
	var port := DEFAULT_PORT
	if OS.has_environment("AGENT_BRIDGE_PORT"):
		port = int(OS.get_environment("AGENT_BRIDGE_PORT"))
	var err := _server.listen(port, "127.0.0.1")
	if err != OK:
		push_error("agent_bridge: cannot listen on 127.0.0.1:%d (%s)" % [port, error_string(err)])
		return
	# The launcher scripts wait for this line before connecting.
	print("agent_bridge: listening on 127.0.0.1:%d" % port)


func _exit_tree() -> void:
	_server.stop()


func _process(_delta: float) -> void:
	while _server.is_connection_available():
		_clients.append(_Client.new(_server.take_connection()))
	for client: _Client in _clients.duplicate():
		client.peer.poll()
		if client.peer.get_status() != StreamPeerTCP.STATUS_CONNECTED:
			_clients.erase(client)
			continue
		var available := client.peer.get_available_bytes()
		if available > 0:
			var chunk: PackedByteArray = client.peer.get_data(available)[1]
			client.buffer.append_array(chunk)
		if not client.busy:
			_next_line(client)


func _next_line(client: _Client) -> void:
	var newline := client.buffer.find(10)
	if newline == -1:
		return
	var line := client.buffer.slice(0, newline).get_string_from_utf8()
	client.buffer = client.buffer.slice(newline + 1)
	client.busy = true
	var reply := await _handle(line)
	client.peer.put_data((JSON.stringify(reply) + "\n").to_utf8_buffer())
	client.busy = false


func _handle(line: String) -> Dictionary:
	var request: Variant = JSON.parse_string(line)
	if not request is Dictionary:
		return {"ok": false, "error": "not a JSON object: %s" % line.left(200)}
	var req: Dictionary = request
	var reply: Dictionary = {"id": req.get("id")}
	var cmd := str(req.get("cmd", ""))
	var args: Dictionary = req.get("args", {}) if req.get("args") is Dictionary else {}
	var commands := _commands()
	if cmd not in commands:
		reply.merge(
			{"ok": false, "error": "unknown cmd %s; try one of %s" % [cmd, commands.keys()]}
		)
		return reply
	var result: Variant = await (commands[cmd] as Callable).call(args)
	if result is _Failure:
		var failure: _Failure = result
		reply.merge({"ok": false, "error": failure.message})
	else:
		reply.merge({"ok": true, "result": _to_json(result)})
	return reply


func _commands() -> Dictionary[String, Callable]:
	return {
		"ping": _ping,
		"tree": _tree,
		"get": _get_prop,
		"set": _set_prop,
		"call": _call,
		"eval": _eval,
		"action": _action,
		"key": _key,
		"click": _click,
		"wait": _wait,
		"screenshot": _screenshot,
		"quit": _quit,
	}


## {} -> the engine version and the current scene's path.
func _ping(_args: Dictionary) -> Variant:
	var scene := get_tree().current_scene
	return {
		"godot": Engine.get_version_info().string,
		"scene": scene.scene_file_path if scene else "",
		"paused": get_tree().paused,
	}


## {path = "/root", depth = 3} -> nested {name, type, children}, plus text for
## anything that has some, which is usually what an agent is looking for.
func _tree(args: Dictionary) -> Variant:
	var node := get_node_or_null(str(args.get("path", "/root")))
	if node == null:
		return _Failure.new("no node at %s" % args.get("path"))
	return _describe(node, _int(args, "depth", 3))


func _describe(node: Node, depth: int) -> Dictionary:
	var out: Dictionary = {"name": str(node.name), "type": node.get_class()}
	if "text" in node:
		out["text"] = str(node.get("text"))
	if node is CanvasItem and not (node as CanvasItem).visible:
		out["visible"] = false
	if depth > 0 and node.get_child_count() > 0:
		var children: Array[Dictionary] = []
		for child: Node in node.get_children():
			if child != self:
				children.append(_describe(child, depth - 1))
		out["children"] = children
	return out


## {path, prop} -> the property's value.
func _get_prop(args: Dictionary) -> Variant:
	var node := get_node_or_null(str(args.get("path", "")))
	if node == null:
		return _Failure.new("no node at %s" % args.get("path"))
	return node.get_indexed(NodePath(str(args.get("prop", ""))))


## {path, prop, value} -> null. value is JSON, or var_to_str text in a string
## prefixed with "var:" for anything JSON cannot say ("var:Vector2(1, 2)").
func _set_prop(args: Dictionary) -> Variant:
	var node := get_node_or_null(str(args.get("path", "")))
	if node == null:
		return _Failure.new("no node at %s" % args.get("path"))
	node.set_indexed(NodePath(str(args.get("prop", ""))), _from_json(args.get("value")))
	return null


## {path, method, args = []} -> the return value, awaited if it is a coroutine.
func _call(args: Dictionary) -> Variant:
	var node := get_node_or_null(str(args.get("path", "")))
	if node == null:
		return _Failure.new("no node at %s" % args.get("path"))
	var method := str(args.get("method", ""))
	if not node.has_method(method):
		return _Failure.new("%s has no method %s" % [node.get_path(), method])
	var given: Array = args.get("args") if args.get("args") is Array else []
	var call_args := given.map(_from_json)
	return await node.callv(method, call_args)


## {expr} -> the value of a GDScript expression, evaluated with `root` (the
## scene tree's root), `tree` and `scene` (the current scene) in scope.
func _eval(args: Dictionary) -> Variant:
	var expression := Expression.new()
	var names := PackedStringArray(["root", "tree", "scene"])
	var err := expression.parse(str(args.get("expr", "")), names)
	if err != OK:
		return _Failure.new(expression.get_error_text())
	var tree := get_tree()
	var value: Variant = expression.execute([tree.root, tree, tree.current_scene], self)
	if expression.has_execute_failed():
		return _Failure.new(expression.get_error_text())
	return value


## {name, frames = 2} -> null. Presses an input action for [frames] frames and
## releases it, through Input so it reaches _input handlers and is_action_*
## polling alike. {name, pressed} holds or releases it instead.
func _action(args: Dictionary) -> Variant:
	var action := StringName(str(args.get("name", "")))
	if not InputMap.has_action(action):
		return _Failure.new("no input action %s" % action)
	if args.has("pressed"):
		_push_action(action, _int(args, "pressed", 0) != 0)
		return null
	_push_action(action, true)
	await _frames(_int(args, "frames", 2))
	_push_action(action, false)
	await _frames(1)
	return null


func _push_action(action: StringName, pressed: bool) -> void:
	var event := InputEventAction.new()
	event.action = action
	event.pressed = pressed
	Input.parse_input_event(event)


## {key = "Space", frames = 2} -> null. Key names are OS.find_keycode_from_string's.
func _key(args: Dictionary) -> Variant:
	var keycode := OS.find_keycode_from_string(str(args.get("key", "")))
	if keycode == KEY_NONE:
		return _Failure.new("unknown key %s" % args.get("key"))
	for pressed: bool in [true, false]:
		var event := InputEventKey.new()
		event.keycode = keycode
		event.physical_keycode = keycode
		event.pressed = pressed
		Input.parse_input_event(event)
		await _frames(_int(args, "frames", 2) if pressed else 1)
	return null


## {x, y, button = 1} -> null. Viewport coordinates, as a screenshot shows them.
func _click(args: Dictionary) -> Variant:
	var at := Vector2(_float(args, "x", 0.0), _float(args, "y", 0.0))
	var motion := InputEventMouseMotion.new()
	motion.position = at
	motion.global_position = at
	Input.parse_input_event(motion)
	await _frames(1)
	for pressed: bool in [true, false]:
		var event := InputEventMouseButton.new()
		event.button_index = _int(args, "button", MOUSE_BUTTON_LEFT) as MouseButton
		event.position = at
		event.global_position = at
		event.pressed = pressed
		Input.parse_input_event(event)
		await _frames(1)
	return null


## {frames = 1} -> null. For letting a tween or a scene change land.
func _wait(args: Dictionary) -> Variant:
	await _frames(_int(args, "frames", 1))
	return null


## {path} -> the absolute path written. A PNG of the next drawn frame; a
## relative path is taken from the game's working directory.
func _screenshot(args: Dictionary) -> Variant:
	var path := str(args.get("path", "user://screenshot.png"))
	await RenderingServer.frame_post_draw
	var image := get_viewport().get_texture().get_image()
	var absolute := ProjectSettings.globalize_path(path)
	var err := image.save_png(absolute)
	if err != OK:
		return _Failure.new("cannot write %s (%s)" % [absolute, error_string(err)])
	return absolute


## {code = 0} -> null, then the game exits.
func _quit(args: Dictionary) -> Variant:
	get_tree().quit.call_deferred(_int(args, "code", 0))
	return null


func _frames(count: int) -> void:
	for _i: int in maxi(count, 0):
		await get_tree().process_frame


static func _to_json(value: Variant) -> Variant:
	match typeof(value):
		TYPE_NIL, TYPE_BOOL, TYPE_INT, TYPE_FLOAT, TYPE_STRING:
			return value
		TYPE_STRING_NAME, TYPE_NODE_PATH:
			return str(value)
		TYPE_ARRAY, TYPE_PACKED_STRING_ARRAY, TYPE_PACKED_INT32_ARRAY, TYPE_PACKED_FLOAT32_ARRAY:
			var items: Array = type_convert(value, TYPE_ARRAY)
			return items.map(_to_json)
		TYPE_DICTIONARY:
			var dict: Dictionary = value
			var out: Dictionary = {}
			for k: Variant in dict:
				out[str(k)] = _to_json(dict[k])
			return out
		TYPE_OBJECT:
			if value is Node:
				var node: Node = value
				return "Node(%s)" % node.get_path()
			return str(value)
	return var_to_str(value)


static func _from_json(value: Variant) -> Variant:
	var text: String = value if value is String else ""
	if text.begins_with("var:"):
		return str_to_var(text.substr(4))
	return value


## JSON numbers arrive as floats and a caller may send "3"; type_convert takes
## all of them, where int(Variant) is the unsafe call the warnings flag.
static func _int(args: Dictionary, key: String, default: int) -> int:
	return type_convert(args.get(key, default), TYPE_INT)


static func _float(args: Dictionary, key: String, default: float) -> float:
	return type_convert(args.get(key, default), TYPE_FLOAT)


class _Failure:
	var message: String

	func _init(text: String) -> void:
		message = text
