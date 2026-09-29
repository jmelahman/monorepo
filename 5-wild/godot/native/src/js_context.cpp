#include "js_context.h"

#include <godot_cpp/core/class_db.hpp>
#include <vector>
#include <godot_cpp/variant/utility_functions.hpp>

extern "C" {
#include <quickjs.h>
}

using namespace godot;

namespace {

String to_godot(::JSContext *ctx, JSValueConst value) {
	size_t len = 0;
	const char *str = JS_ToCStringLen(ctx, &len, value);
	if (str == nullptr) {
		return String();
	}
	String out = String::utf8(str, static_cast<int64_t>(len));
	JS_FreeCString(ctx, str);
	return out;
}

// print(...) for the bundle: the engine never logs, but a bundle that fails
// in a way a stack doesn't explain is much easier to read with it.
JSValue js_print(::JSContext *ctx, JSValueConst, int argc, JSValueConst *argv) {
	PackedStringArray parts;
	for (int i = 0; i < argc; i++) {
		parts.push_back(to_godot(ctx, argv[i]));
	}
	UtilityFunctions::print(String(" ").join(parts));
	return JS_UNDEFINED;
}

} // namespace

JsContext::JsContext() {
	rt = JS_NewRuntime();
	ctx = JS_NewContext(rt);
	JSValue global = JS_GetGlobalObject(ctx);
	JS_SetPropertyStr(ctx, global, "print", JS_NewCFunction(ctx, js_print, "print", 1));
	JS_FreeValue(ctx, global);
}

JsContext::~JsContext() {
	if (ctx != nullptr) {
		JS_FreeContext(ctx);
	}
	if (rt != nullptr) {
		JS_FreeRuntime(rt);
	}
}

String JsContext::take_exception() {
	JSValue exc = JS_GetException(ctx);
	String message = to_godot(ctx, exc);
	if (JS_IsError(exc)) {
		JSValue stack = JS_GetPropertyStr(ctx, exc, "stack");
		if (!JS_IsUndefined(stack)) {
			message += "\n" + to_godot(ctx, stack);
		}
		JS_FreeValue(ctx, stack);
	}
	JS_FreeValue(ctx, exc);
	return message;
}

bool JsContext::load(const String &source, const String &filename) {
	CharString code = source.utf8();
	CharString name = filename.utf8();
	// JS_Eval needs the NUL after the text, which CharString keeps.
	JSValue result = JS_Eval(ctx, code.get_data(), static_cast<size_t>(code.length()),
			name.get_data(), JS_EVAL_TYPE_GLOBAL);
	bool ok = !JS_IsException(result);
	error = ok ? String() : take_exception();
	JS_FreeValue(ctx, result);
	return ok;
}

String JsContext::invoke(const String &path, const PackedStringArray &args) {
	JSValue self = JS_GetGlobalObject(ctx);
	JSValue fn = JS_DupValue(ctx, self);
	PackedStringArray names = path.split(".");
	for (int i = 0; i < names.size(); i++) {
		JS_FreeValue(ctx, self);
		self = fn;
		fn = JS_GetPropertyStr(ctx, self, names[i].utf8().get_data());
	}
	if (!JS_IsFunction(ctx, fn)) {
		JS_FreeValue(ctx, fn);
		JS_FreeValue(ctx, self);
		error = path + String(" is not a function");
		return String();
	}

	std::vector<JSValue> argv;
	argv.reserve(static_cast<size_t>(args.size()));
	for (int i = 0; i < args.size(); i++) {
		CharString arg = args[i].utf8();
		argv.push_back(JS_NewStringLen(ctx, arg.get_data(), static_cast<size_t>(arg.length())));
	}
	JSValue result = JS_Call(ctx, fn, self, static_cast<int>(argv.size()), argv.data());
	for (JSValue &arg : argv) {
		JS_FreeValue(ctx, arg);
	}
	JS_FreeValue(ctx, fn);
	JS_FreeValue(ctx, self);

	String out;
	if (JS_IsException(result)) {
		error = take_exception();
	} else {
		error = String();
		out = to_godot(ctx, result);
	}
	JS_FreeValue(ctx, result);
	return out;
}

String JsContext::get_error() const {
	return error;
}

void JsContext::_bind_methods() {
	ClassDB::bind_method(D_METHOD("load", "source", "filename"), &JsContext::load);
	ClassDB::bind_method(D_METHOD("invoke", "path", "args"), &JsContext::invoke,
			DEFVAL(PackedStringArray()));
	ClassDB::bind_method(D_METHOD("get_error"), &JsContext::get_error);
}
