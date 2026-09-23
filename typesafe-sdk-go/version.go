package typesafe

import (
	"fmt"
	"runtime"
	"strings"
)

// Version is the version of this SDK, reported in the User-Agent and
// X-TypeSafe-SDK request headers.
const Version = "0.1.0"

// userAgent identifies this SDK to the API.
const userAgent = "typesafe-sdk/" + Version

// runtimeHeader describes the Go runtime for the X-TypeSafe-Runtime header,
// matching the shape the other TypeSafe SDKs report, such as
// "node/22.1.0 (linux; x64)".
var runtimeHeader = fmt.Sprintf("go/%s (%s; %s)",
	strings.TrimPrefix(runtime.Version(), "go"), runtime.GOOS, runtime.GOARCH)
