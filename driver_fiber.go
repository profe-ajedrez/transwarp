//go:build fiber

// This file is conditionally compiled. It is ONLY included in the build
// when the `-tags fiber` flag is passed to the Go compiler.
//
// Usage:
//   go build -tags fiber -o myapp main.go

package transwarp

import (
	// Import Fiber v3.
	// Fiber is an Express-inspired web framework built on top of Fasthttp,
	// the fastest HTTP engine for Go. It is designed for zero memory allocation
	// and high performance.
	"github.com/gofiber/fiber/v3"

	// Import the internal adapter that bridges the gap between Transwarp's
	// unified interface and Fiber's specific implementation.
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/fiberadapter"
)

// init serves as the registration hook for the Fiber driver.
//
// When the application starts, this function executes automatically, but
// ONLY if the `fiber` build tag was present during compilation.
//
// This architecture ensures that if you compile for Gin or Chi, the Fiber
// library (and its heavy reliance on unsafe/fasthttp) is completely excluded
// from your final binary, keeping it lightweight and compliant.
func init() {
	Register(DriverFiber, func() Transwarp {
		// 1. Initialize the Fiber application.
		// We set a custom AppName to identify the engine in headers/logs.
		app := fiber.New(fiber.Config{
			AppName: "Transwarp Engine (Fiber v3)",
		})

		// 2. Return the Adapter.
		// Fiber's architecture separates the 'App' (the engine) from the 'Router'
		// in some contexts, but 'app' satisfies both in v3.
		// We pass it to our adapter to translate standardized Transwarp routes
		// (e.g., /:id) into Fiber's routing system.
		return &fiberadapter.FiberAdapter{
			App:    app,
			Router: app,
		}
	})
}
