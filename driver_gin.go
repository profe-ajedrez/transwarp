//go:build gin

// This file is conditionally compiled. It is ONLY included in the build
// when the `-tags gin` flag is passed to the Go compiler.
//
// Usage:
//   go build -tags gin -o myapp main.go

package transwarp

import (
	// Import the Gin Gonic framework (v1).
	// Gin is known for its stability, extensive middleware ecosystem,
	// and high performance using httprouter.
	"github.com/gin-gonic/gin"

	// Import the internal adapter that wraps Gin's specific logic
	// to satisfy the Transwarp interface.
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/ginadapter"
)

// init serves as the registration hook for the Gin driver.
//
// When the application starts, Go automatically executes all init() functions.
// Because this file is guarded by the `//go:build gin` tag, this registration
// logic will ONLY execute if the user explicitly chose Gin during compilation.
func init() {
	Register(DriverGin, func() Transwarp {
		// 1. Optimize for Production.
		// By default, Gin runs in "Debug Mode", which outputs verbose logs
		// to the console. We set it to "Release Mode" here to ensure
		// maximum performance and cleaner logs by default.
		// Use environment variables (GIN_MODE=debug) if debugging is needed.
		gin.SetMode(gin.ReleaseMode)

		// 2. Initialize the Default Engine.
		// gin.Default() creates a router instance with the Logger and Recovery
		// middleware already attached.
		// - Logger: Writes request logs to io.Writer (default os.Stdout).
		// - Recovery: Recovers from any panics and writes a 500 error if there was one.
		g := gin.Default()

		// 3. Return the Adapter.
		// We wrap the Gin engine in our adapter struct.
		return &ginadapter.GinAdapter{Router: g}
	})
}
