// Package transwarp provides a unified, framework-agnostic interface for HTTP routing in Go.
//
// It acts as a wrapper layer that allows developers to write routing logic once and
// switch between different underlying web engines (such as Fiber, Gin, Echo, or Chi)
// at compile time using Go build tags.
package transwarp

import (
	"net/http"

	"github.com/profe-ajedrez/transwarp/internal"
)

// Driver represents the specific web framework engine that Transwarp will use
// to handle HTTP requests.
//
// The value of the Driver must match the build tag provided during compilation.
type Driver string

const (
	// DriverGin selects the Gin Gonic framework (v1).
	// To use this driver, you must compile with: -tags gin
	DriverGin Driver = "gin"

	// DriverEcho selects the Echo framework (v5).
	// To use this driver, you must compile with: -tags echo
	DriverEcho Driver = "echo"

	// DriverFiber selects the GoFiber framework (v3 Beta).
	// This driver offers high performance but requires specific handling for
	// zero-allocation contexts.
	// To use this driver, you must compile with: -tags fiber
	DriverFiber Driver = "fiber"

	// DriverChi selects the go-chi framework (v5).
	// This driver is fully compatible with standard net/http and is lightweight.
	// To use this driver, you must compile with: -tags chi
	DriverChi Driver = "chi"

	// DriverMock selects the internal Mock router.
	// This driver is intended for unit testing logic without spinning up
	// a real TCP network listener. It requires no specific build tags.
	DriverMock Driver = "mock"

	DriverNative Driver = "native"
)

// Middleware defines the standard function signature for HTTP interceptors.
//
// Transwarp enforces the standard "net/http" middleware pattern:
//
//	func(next http.Handler) http.Handler
//
// This ensures that middlewares are portable and compatible across all
// supported drivers (Gin, Fiber, etc.), as Transwarp handles the necessary
// internal adaptations for non-standard frameworks.
type Middleware func(http.Handler) http.Handler

// Transwarp is the main interface that abstracts the routing logic.
//
// It embeds the internal Router interface, exposing methods to:
//   - Register routes (GET, POST, PUT, DELETE, etc.)
//   - Create route groups with shared middleware.
//   - Register middleware (Use).
//   - Retrieve URL parameters in a unified way.
//   - Start the HTTP server (Serve).
//
// Implementation details are hidden behind this interface, allowing the
// underlying engine to be swapped without changing the consuming code.
type Transwarp interface {
	internal.Router
}

// Config holds the initialization settings for the Transwarp engine.
type Config struct {
	// Driver specifies which underlying engine should be initialized.
	//
	// Important: The chosen Driver must match the build tags used during
	// compilation. For example, if DriverFiber is set, the application
	// must be built with `go build -tags fiber`.
	//
	// If the Driver does not match the compiled build tags, the Factory
	// method (New) will panic to prevent runtime inconsistencies.
	Driver Driver
}

// Chain is a helper that wraps a specific handler with a list of middlewares.
//
// It allows applying middlewares to a single endpoint without creating a Group.
// The middlewares are applied in the order they are passed (first matches first).
//
// Usage:
//
//	server.GET("/private", transwarp.Chain(myHandler, AuthMW, LoggingMW))
func Chain(h http.HandlerFunc, mws ...Middleware) http.HandlerFunc {
	// If no middlewares, return original handler
	if len(mws) == 0 {
		return h
	}

	// Wrap the handler with middlewares in reverse order
	// so the first passed middleware is the first to execute.
	var final http.Handler = h
	for i := len(mws) - 1; i >= 0; i-- {
		final = mws[i](final)
	}

	return final.ServeHTTP
}
