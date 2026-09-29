#pragma once

// A QuickJS runtime and context, owned by a RefCounted so GDScript can hold
// one like any other object and it is freed when the last reference goes.
//
// The seam is strings on purpose. The engine's state is plain JSON by
// contract (5-wild/CLAUDE.md), so JSON text crosses it losslessly, and a
// string-in, string-out boundary is the whole of what this class has to get
// right. Converting Variants to JS values property by property would be more
// code in the one place a bug is hardest to see, and it would have to decide
// what an int is: JSON has one number type and so does JS, and GDScript's
// JSON.parse_string already answers that the same way on the way back.

#include <godot_cpp/classes/ref_counted.hpp>
#include <godot_cpp/variant/packed_string_array.hpp>
#include <godot_cpp/variant/string.hpp>

struct JSRuntime;
struct JSContext;

namespace godot {

class JsContext : public RefCounted {
	GDCLASS(JsContext, RefCounted)

public:
	JsContext();
	~JsContext() override;

	/// Runs `source` as a classic script in the global scope. Returns true on
	/// success; on failure, `get_error()` has the message and stack.
	bool load(const String &source, const String &filename);

	/// Calls the function at the dotted global `path` ("fivewild.reduce") with
	/// `args` as JS strings and returns its result as a string: a string result
	/// as is, anything else through JS String(). A throw returns "" and sets
	/// `get_error()`; success clears it.
	String invoke(const String &path, const PackedStringArray &args);

	/// The last failure's message and JS stack, or "" if the last call worked.
	String get_error() const;

protected:
	static void _bind_methods();

private:
	String take_exception();

	::JSRuntime *rt = nullptr;
	::JSContext *ctx = nullptr;
	String error;
};

} // namespace godot
