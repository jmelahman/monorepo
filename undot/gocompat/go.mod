// Go-stdlib-backed implementations of the solod.dev APIs used by
// undot. The default root modfile replaces solod.dev with this module
// so that a regular `go build` produces a fully functional binary (the real
// solod.dev package bodies are transpiler stubs). Only the API surface
// undot uses is implemented.
module solod.dev

go 1.22
