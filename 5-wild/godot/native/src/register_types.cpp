#include "js_context.h"

#include <gdextension_interface.h>
#include <godot_cpp/core/class_db.hpp>
#include <godot_cpp/godot.hpp>

using namespace godot;

namespace {

void initialize(ModuleInitializationLevel level) {
	if (level == MODULE_INITIALIZATION_LEVEL_SCENE) {
		GDREGISTER_CLASS(JsContext);
	}
}

void uninitialize(ModuleInitializationLevel) {}

} // namespace

extern "C" GDExtensionBool GDE_EXPORT fivewild_js_init(
		GDExtensionInterfaceGetProcAddress get_proc_address,
		GDExtensionClassLibraryPtr library,
		GDExtensionInitialization *init) {
	GDExtensionBinding::InitObject object(get_proc_address, library, init);
	object.register_initializer(initialize);
	object.register_terminator(uninitialize);
	object.set_minimum_library_initialization_level(MODULE_INITIALIZATION_LEVEL_SCENE);
	return object.init();
}
