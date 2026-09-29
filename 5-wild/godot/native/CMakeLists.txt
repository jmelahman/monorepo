# The fivewild_js GDExtension: QuickJS (quickjs-ng) behind one Godot class,
# JsContext, so the TypeScript engine in ../../src/engine runs unchanged inside
# the desktop build. See src/js_context.h for why the seam is strings.
#
# Both dependencies are fetched by tarball and checked against a sha256, like
# every other download in scripts/versions.env. scripts/build-native.sh drives
# this; the library lands in ../bin/, where fivewild_js.gdextension finds it.
cmake_minimum_required(VERSION 3.28)
project(fivewild_js LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
set(CMAKE_CXX_VISIBILITY_PRESET hidden)
set(CMAKE_C_VISIBILITY_PRESET hidden)

include(FetchContent)

# One build serves the editor, debug and release exports alike: the class uses
# nothing the editor-only or debug API adds, and the .gdextension file points
# every feature tag at the same library.
set(GODOTCPP_TARGET template_release CACHE STRING "" FORCE)
set(GODOTCPP_API_VERSION 4.7 CACHE STRING "" FORCE)
FetchContent_Declare(godot-cpp
  URL https://github.com/godotengine/godot-cpp/archive/refs/tags/10.0.0-stable.tar.gz
  URL_HASH SHA256=ebbf64f0f8f8b8b66bc4bc4d3797db97f95b19cf0dfb5b3f8cf56683d9df7db4
  EXCLUDE_FROM_ALL)

# Only the library: no qjs/qjsc executables, no libc bindings (the engine
# needs neither files nor timers, and giving it none keeps it honest).
set(QJS_BUILD_LIBC OFF CACHE BOOL "" FORCE)
set(QJS_ENABLE_INSTALL OFF CACHE BOOL "" FORCE)
FetchContent_Declare(quickjs
  URL https://github.com/quickjs-ng/quickjs/archive/refs/tags/v0.17.0.tar.gz
  URL_HASH SHA256=559bc4c420475e55c7ab4510adbc562f55d7524d75e8e89d79ce4bb02f5687d9
  EXCLUDE_FROM_ALL)

FetchContent_MakeAvailable(godot-cpp quickjs)

add_library(fivewild_js SHARED src/register_types.cpp src/js_context.cpp)
target_link_libraries(fivewild_js PRIVATE godot::cpp qjs)

# fivewild_js.linux.x86_64.so / fivewild_js.windows.x86_64.dll, no lib prefix,
# so the .gdextension file can name both the same way.
string(TOLOWER "${CMAKE_SYSTEM_NAME}" platform)
set_target_properties(fivewild_js PROPERTIES
  PREFIX ""
  OUTPUT_NAME "fivewild_js.${platform}.x86_64"
  LIBRARY_OUTPUT_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/../bin"
  RUNTIME_OUTPUT_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/../bin")
if(WIN32)
  # A MinGW DLL otherwise needs libstdc++-6.dll and friends beside it.
  target_link_options(fivewild_js PRIVATE -static-libgcc -static-libstdc++ -static)
endif()
