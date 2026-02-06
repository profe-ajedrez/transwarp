package ginadapter

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/profe-ajedrez/transwarp/internal"
)

// ginCtxKey is a private type used for context keys.
// It prevents key collisions with other packages that might be using string keys
// within the standard context.
type ginCtxKey string

// ginParamsKey is the specific key used to store the original *gin.Context
// inside the standard net/http request context.
//
// This allows retrieval of Gin-specific features (like URL parameters via c.Param)
// from within standard http.Handlers that otherwise wouldn't know they are running inside Gin.
const ginParamsKey ginCtxKey = "gin_params"

// GinAdapter implements the Transwarp interface for the Gin Gonic framework.
//
// It wraps Gin's specific routing and middleware capabilities to expose
// a unified, standard net/http interface.
type GinAdapter struct {
	// Router holds the underlying Gin interface.
	// We use gin.IRouter (interface) instead of *gin.Engine so that this
	// struct can represent both the main application (*gin.Engine) and
	// nested route groups (*gin.RouterGroup) uniformly.
	Router gin.IRouter
}

// Group creates a new sub-router with a specific path prefix.
//
// It utilizes Gin's native Group() functionality. The returned adapter
// wraps the new *gin.RouterGroup, ensuring recursive compatibility with
// the Transwarp interface.
func (a *GinAdapter) Group(prefix string) internal.Router {
	return &GinAdapter{Router: a.Router.Group(prefix)}
}

// handle is the central helper function for route registration.
//
// It performs the critical task of bridging the gap between Gin's
// func(*gin.Context) signature and the standard http.HandlerFunc.
//
// Mechanism:
// 1. It wraps the standard 'h' handler inside a Gin handler.
// 2. It injects the *gin.Context into the standard request Context (Context Injection).
// 3. It executes the standard handler using the Gin response writer.
func (a *GinAdapter) handle(method, path string, h http.HandlerFunc) {
	fn := func(c *gin.Context) {
		// INJECTION STEP:
		// Save the Gin context into the request context under a private key.
		// This is required for a.Param() to work later inside the handler.
		ctx := context.WithValue(c.Request.Context(), ginParamsKey, c)

		// Execute the standard handler with the enriched context.
		h(c.Writer, c.Request.WithContext(ctx))
	}

	// Register the wrapped function with the underlying Gin router.
	a.Router.Handle(method, path, fn)
}

// GET registers a new request handler for the HTTP GET method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) GET(p string, h http.HandlerFunc) { a.handle(http.MethodGet, p, h) }

// POST registers a new request handler for the HTTP POST method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) POST(p string, h http.HandlerFunc) { a.handle(http.MethodPost, p, h) }

// PUT registers a new request handler for the HTTP PUT method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) PUT(p string, h http.HandlerFunc) { a.handle(http.MethodPut, p, h) }

// PATCH registers a new request handler for the HTTP PATCH method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) PATCH(p string, h http.HandlerFunc) { a.handle(http.MethodPatch, p, h) }

// DELETE registers a new request handler for the HTTP DELETE method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) DELETE(p string, h http.HandlerFunc) { a.handle(http.MethodDelete, p, h) }

// HEAD registers a new request handler for the HTTP HEAD method.
// Delegates to the internal 'handle' helper.
func (a *GinAdapter) HEAD(p string, h http.HandlerFunc) { a.handle(http.MethodHead, p, h) }

// Use registers a global or group-level middleware.
//
// This method bridges the gap between standard Go middleware (which controls execution via function calls)
// and Gin middleware (which controls execution via c.Next() / c.Abort()).
//
// Critical Logic:
// It implements a mechanism to detect if the standard middleware decided to stop
// the request chain (e.g., Authentication failure). If the standard middleware
// returns without calling 'next', this adapter explicitly calls c.Abort() to ensure
// Gin respects the interruption.
func (a *GinAdapter) Use(mw internal.Middleware) {
	a.Router.Use(func(c *gin.Context) {
		// 'called' tracks whether the middleware invoked the 'next' handler.
		called := false

		// This 'finalHandler' represents the "next" link in the chain from the
		// perspective of the standard middleware.
		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true // Mark that the chain continued successfully.

			// Update Gin's request reference in case the middleware modified it.
			c.Request = r
			// Hand over control back to Gin's chain.
			c.Next()
		})

		// INJECTION STEP (Middleware Level):
		// We must also inject the context here, so middlewares can access params via a.Param().
		ctx := context.WithValue(c.Request.Context(), ginParamsKey, c)

		// Execute the Transwarp middleware.
		mw(finalHandler).ServeHTTP(c.Writer, c.Request.WithContext(ctx))

		// GIN SPECIFIC CONTROL FLOW:
		// If 'called' is false, it means the middleware returned WITHOUT calling next.ServeHTTP.
		// In standard net/http, this stops the chain.
		// In Gin, however, we must explicitly call c.Abort(), otherwise Gin might continue
		// to the next handler in its internal index.
		if !called {
			c.Abort()
		}
	})
}

// Param retrieves a URL parameter value (e.g., "id" from "/user/:id").
//
// It retrieves the *gin.Context hidden inside the request context (injected by handle/Use)
// and calls Gin's native .Param() method.
func (a *GinAdapter) Param(r *http.Request, key string) string {
	// Attempt to retrieve the stored Gin context.
	if c, ok := r.Context().Value(ginParamsKey).(*gin.Context); ok {
		return c.Param(key)
	}
	return ""
}

// Serve starts the HTTP server.
//
// It attempts to cast the internal Router to *gin.Engine to call Run().
// If the adapter is wrapping a RouteGroup (which cannot run directly),
// it falls back to http.ListenAndServe using nil handler (standard fallback),
// though this scenario is rare in practice.
func (a *GinAdapter) Serve(port string) error {
	// Check if the router is the main Engine
	if engine, ok := a.Router.(*gin.Engine); ok {
		return engine.Run(port)
	}
	// Fallback if we are somehow trying to serve a Group directly
	return http.ListenAndServe(port, nil)
}
