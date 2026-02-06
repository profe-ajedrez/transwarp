package echoadapter

import (
	"context"
	"net/http"

	// Using Echo v5 as defined in the project requirements.
	echo "github.com/labstack/echo/v5"
	"github.com/profe-ajedrez/transwarp/internal"
)

// EchoAdapter is the implementation of the Transwarp interface for the Echo framework.
//
// It serves as a bridge, translating standard net/http calls into Echo's specific
// context-based architecture.
type EchoAdapter struct {
	// Instance holds the reference to the main Echo engine.
	// This is used when registering routes at the root level.
	Instance *echo.Echo

	// group holds the reference to a specific Echo Route Group.
	// If this field is not nil, routes will be registered to this group
	// instead of the main Instance.
	group *echo.Group
}

// ctxKey is a private type used for context keys to prevent collisions
// with other packages using context.WithValue.
type ctxKey string

// paramsKey is the specific key used to store the original *echo.Context
// inside the standard net/http request context.
const paramsKey ctxKey = "params"

// handle is a helper function that registers a standard http.HandlerFunc
// into the Echo router.
//
// The Magic:
// Since Echo uses a custom Context struct for everything (parameters, responses),
// and Transwarp uses standard http.Request, we must "inject" the Echo context
// into the standard Request context.
//
// This allows the Param() method to later retrieve the Echo context and
// extract URL parameters.
func (a *EchoAdapter) handle(method, path string, h http.HandlerFunc) {
	// We wrap the standard handler in an Echo-compatible handler.
	handler := func(c *echo.Context) error {
		// INJECTION STEP:
		// We take the current Echo context 'c' and save it inside the
		// standard request's context under a private key.
		ctx := context.WithValue(c.Request().Context(), paramsKey, c)

		// We execute the standard handler, passing the modified request
		// that now carries the hidden Echo context.
		h(c.Response(), c.Request().WithContext(ctx))
		return nil
	}

	// Register the wrapped handler to either the group or the main instance.
	if a.group != nil {
		a.group.Add(method, path, handler)
	} else {
		a.Instance.Add(method, path, handler)
	}
}

// Param retrieves a URL parameter value (e.g., /users/:id).
//
// How it works:
// It looks into the http.Request context for the hidden *echo.Context
// (injected previously by 'handle' or 'Use'). If found, it delegates
// the parameter lookup to Echo.
func (a *EchoAdapter) Param(r *http.Request, key string) string {
	// Attempt to retrieve the Echo context stored in the request context.
	if c, ok := r.Context().Value(paramsKey).(*echo.Context); ok {
		return c.Param(key)
	}
	return ""
}

// HTTP Verb Implementations
// These methods simply delegate the registration to the internal 'handle' helper.

func (a *EchoAdapter) GET(p string, h http.HandlerFunc)    { a.handle(http.MethodGet, p, h) }
func (a *EchoAdapter) POST(p string, h http.HandlerFunc)   { a.handle(http.MethodPost, p, h) }
func (a *EchoAdapter) PUT(p string, h http.HandlerFunc)    { a.handle(http.MethodPut, p, h) }
func (a *EchoAdapter) PATCH(p string, h http.HandlerFunc)  { a.handle(http.MethodPatch, p, h) }
func (a *EchoAdapter) DELETE(p string, h http.HandlerFunc) { a.handle(http.MethodDelete, p, h) }
func (a *EchoAdapter) HEAD(p string, h http.HandlerFunc)   { a.handle(http.MethodHead, p, h) }

// Group creates a new sub-router with a specific path prefix.
//
// It returns a new *EchoAdapter instance that points to the newly created
// Echo group, ensuring that subsequent calls to GET/POST/Use on the new
// instance are scoped to that group.
func (a *EchoAdapter) Group(prefix string) internal.Router {
	if a.group != nil {
		// Nested group: Create a group inside the existing group.
		return &EchoAdapter{Instance: a.Instance, group: a.group.Group(prefix)}
	}
	// Root group: Create a group directly from the main instance.
	return &EchoAdapter{Instance: a.Instance, group: a.Instance.Group(prefix)}
}

// Use registers a global or group-level middleware.
//
// It adapts the standard 'func(http.Handler) http.Handler' signature
// into Echo's 'func(echo.HandlerFunc) echo.HandlerFunc' signature.
func (a *EchoAdapter) Use(mw internal.Middleware) {
	mwFunc := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			var nextErr error

			// Create a standard http.Handler that wraps the 'next' Echo handler.
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Update Echo's internal request reference (crucial for valid context).
				c.SetRequest(r)
				nextErr = next(c)
			})

			// INJECTION STEP (Middleware Level):
			// We must inject the Echo context here as well. This ensures that
			// if a user calls Param() inside a middleware, it works correctly.
			ctx := context.WithValue(c.Request().Context(), paramsKey, c)

			// Execute the Transwarp middleware, passing the context-aware request.
			mw(finalHandler).ServeHTTP(c.Response(), c.Request().WithContext(ctx))

			return nextErr
		}
	}

	if a.group != nil {
		a.group.Use(mwFunc)
	} else {
		a.Instance.Use(mwFunc)
	}
}

// Serve starts the HTTP server on the specified port.
func (a *EchoAdapter) Serve(port string) error {
	return a.Instance.Start(port)
}
