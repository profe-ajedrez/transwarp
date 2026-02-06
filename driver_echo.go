//go:build echo

// This file is conditionally compiled. It is ONLY included in the build
// when the `-tags echo` flag is passed to the Go compiler.
//
// Usage:
//   go build -tags echo -o myapp main.go

package transwarp

import (
	// Import the specific version of Echo (v5) supported by this adapter.
	echo "github.com/labstack/echo/v5"

	// Import the internal adapter that translates Transwarp calls to Echo calls.
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/echoadapter"
)

// init serves as the registration hook for the Echo driver.
//
// When the application starts, Go automatically executes all init() functions.
// Because this file is protected by the `//go:build echo` tag, this function
// will only run if the user explicitly requested the Echo driver during compilation.
//
// This mechanism ensures that if the user chooses a different driver (like Fiber),
// the Echo dependencies are not registered, keeping the binary size optimized.
func init() {
	Register(DriverEcho, func() Transwarp {
		// 1. Instantiate the native Echo engine.
		e := echo.New()

		// 2. (Optional) Apply default configurations.
		// For example, hiding the startup banner to keep logs clean in production.
		// e.HideBanner = true

		// 3. Return the adapter.
		// We wrap the native 'e' instance in our EchoV5Adapter struct,
		// which satisfies the Transwarp interface.
		return &echoadapter.EchoV5Adapter{Instance: e}
	})
}
