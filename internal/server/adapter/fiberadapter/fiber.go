package fiberadapter

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/profe-ajedrez/transwarp/internal"
)

// fiberCtxKey is a private type used for storing Fiber path parameters
// inside the standard net/http request context.
//
// Since Fiber uses a custom context (fasthttp) and Transwarp handlers expect
// a standard context, we must manually copy the parameters into the standard context
// during the request lifecycle.
type fiberCtxKey string

// FiberAdapter implements the Transwarp interface for the Fiber v3 framework.
//
// Architecture Note:
// Fiber is built on top of 'fasthttp', which is not directly compatible with 'net/http'.
// This adapter utilizes the 'fiber/middleware/adaptor' package to convert standard
// handlers into Fiber handlers.
//
// Additionally, this adapter manages its own middleware stack manually to ensure
// standard 'net/http' middlewares function correctly within the Fiber ecosystem.
type FiberAdapter struct {
	// App is the main Fiber engine instance.
	App *fiber.App

	// Router is the specific router interface (Group or App) where routes
	// will be registered. This allows for nesting.
	Router fiber.Router

	// Middlewares stores the list of standard HTTP middlewares that need to be
	// composed into the handler chain before registration.
	Middlewares []internal.Middleware
}

// Group creates a new sub-router with a specific path prefix.
//
// It performs a shallow copy of the existing middleware stack to ensure that
// the new group inherits middlewares defined in the parent, but subsequent
// middlewares added to this group do not affect the parent.
func (a *FiberAdapter) Group(prefix string) internal.Router {
	// Create a copy of the middleware slice to prevent side effects.
	newMws := make([]internal.Middleware, len(a.Middlewares))
	copy(newMws, a.Middlewares)

	return &FiberAdapter{
		App:         a.App,
		Router:      a.Router.Group(prefix),
		Middlewares: newMws,
	}
}

// Use adds a middleware to the internal stack.
//
// unlike native Fiber middleware, these are standard 'func(http.Handler) http.Handler'
// functions. They are not applied immediately but are composed during the
// 'handle' phase (Lazily evaluated).
func (a *FiberAdapter) Use(mw internal.Middleware) {
	if mw != nil {
		a.Middlewares = append(a.Middlewares, mw)
	}
}

// handle is the core bridging function. It performs three critical tasks:
// 1. Middleware Composition: It wraps the user's handler with the stored middlewares.
// 2. Panic Recovery: It adds a safety net to catch panics within the handler.
// 3. Context Injection: It extracts Fiber URL params and injects them into the standard Context.
func (a *FiberAdapter) handle(method string, path string, h http.HandlerFunc) {
	var composedHandler http.Handler = h

	// 1. Middleware Composition
	// Wrap the handler in reverse order (standard Onion model) so that the
	// first registered middleware is the first to execute.
	for i := len(a.Middlewares) - 1; i >= 0; i-- {
		composedHandler = a.Middlewares[i](composedHandler)
	}

	// 2. Register the Fiber Handler
	a.Router.Add([]string{method}, path, func(c fiber.Ctx) error {
		// Internal Recovery Mechanism
		// Ensures that the server doesn't crash if a standard handler panics.
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("🔥 PANIC IN HANDLER: %v\n", r)
				err := c.Status(http.StatusInternalServerError).SendString("Panic Detected")
				if err != nil {
					log.Warn(err)
				}
			}
		}()

		// 3. Parameter Extraction & Injection
		// We extract parameters from Fiber (e.g., :id) and prepare them
		// to be injected into the net/http context.
		params := make(map[string]string)
		if r := c.Route(); r != nil {
			for _, name := range r.Params {
				// We copy the string to ensure it's safe to use after the fasthttp context is recycled.
				params[name] = string([]byte(c.Params(name)))
			}
		}

		// 4. Adapt and Serve
		// Use the official adaptor to convert the composed http.Handler into a fiber.Handler.
		return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Inject the parameters into the request context.
			ctx := r.Context()
			for k, v := range params {
				ctx = context.WithValue(ctx, fiberCtxKey(k), v)
			}
			// Execute the chain with the enriched context.
			composedHandler.ServeHTTP(w, r.WithContext(ctx))
		})(c)
	})
}

// Param retrieves a URL parameter value (e.g., "id" from "/users/:id").
//
// It looks up the value in the standard http.Request context, using the
// specific key populated by the 'handle' method.
func (a *FiberAdapter) Param(r *http.Request, key string) string {
	if r == nil || r.Context() == nil {
		return ""
	}
	// Retrieve the value stored under the private fiberCtxKey.
	if val, ok := r.Context().Value(fiberCtxKey(key)).(string); ok {
		return val
	}
	return ""
}

// GET registers a new request handler for the HTTP GET method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) GET(p string, h http.HandlerFunc) { a.handle(http.MethodGet, p, h) }

// POST registers a new request handler for the HTTP POST method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) POST(p string, h http.HandlerFunc) { a.handle(http.MethodPost, p, h) }

// PUT registers a new request handler for the HTTP PUT method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) PUT(p string, h http.HandlerFunc) { a.handle(http.MethodPut, p, h) }

// DELETE registers a new request handler for the HTTP DELETE method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) DELETE(p string, h http.HandlerFunc) { a.handle(http.MethodDelete, p, h) }

// HEAD registers a new request handler for the HTTP HEAD method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) HEAD(p string, h http.HandlerFunc) { a.handle(http.MethodHead, p, h) }

// PATCH registers a new request handler for the HTTP PATCH method.
// Delegates to the internal 'handle' method.
func (a *FiberAdapter) PATCH(p string, h http.HandlerFunc) { a.handle(http.MethodPatch, p, h) }

// Serve starts the Fiber HTTP server on the specified port.
//
// It is a wrapper around Fiber's App.Listen().
func (a *FiberAdapter) Serve(port string) error {
	return a.App.Listen(port)
}
