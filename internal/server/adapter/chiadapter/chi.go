package chiadapter

import (
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/profe-ajedrez/transwarp/internal"
)

// ChiAdapter implements the Transwarp interface using the go-chi/chi router.
//
// It acts as a wrapper that translates Transwarp's universal conventions
// (like the ":param" syntax) into Chi's specific requirements (like the "{param}" syntax).
type ChiAdapter struct {
	Router chi.Router
}

// paramRegex matches the universal parameter format used by Transwarp (e.g., ":id", ":user_name").
// It captures the parameter name following the colon.
var paramRegex = regexp.MustCompile(`:([a-zA-Z0-9_]+)`)

// adaptPath translates a universal path string into a Chi-compatible path string.
//
// Transwarp standardizes on the ":param" syntax (e.g., "/users/:id").
// However, Chi uses the "{param}" syntax (e.g., "/users/{id}").
// This function performs a regex replacement to ensure compatibility transparently.
//
// Example:
//
//	Input:  "/category/:cat/item/:id"
//	Output: "/category/{cat}/item/{id}"
func adaptPath(path string) string {
	return paramRegex.ReplaceAllString(path, "{$1}")
}

// register is an internal helper that adapts the path and registers the handler
// to the underlying Chi router based on the HTTP method.
func (a *ChiAdapter) register(method, path string, h http.HandlerFunc) {
	// Crucial Step: Convert ":param" to "{param}" before registering with Chi.
	chiPath := adaptPath(path)

	switch method {
	case http.MethodGet:
		a.Router.Get(chiPath, h)
	case http.MethodPost:
		a.Router.Post(chiPath, h)
	case http.MethodPut:
		a.Router.Put(chiPath, h)
	case http.MethodDelete:
		a.Router.Delete(chiPath, h)
	case http.MethodHead:
		a.Router.Head(chiPath, h)
	}
}

// GET registers a new request handler for the HTTP GET method.
//
// The path provided supports the ":param" syntax, which is automatically
// converted to Chi's "{param}" syntax.
func (a *ChiAdapter) GET(path string, h http.HandlerFunc) { a.register(http.MethodGet, path, h) }

// POST registers a new request handler for the HTTP POST method.
//
// The path provided supports the ":param" syntax, which is automatically
// converted to Chi's "{param}" syntax.
func (a *ChiAdapter) POST(path string, h http.HandlerFunc) { a.register(http.MethodPost, path, h) }

// PUT registers a new request handler for the HTTP PUT method.
//
// The path provided supports the ":param" syntax, which is automatically
// converted to Chi's "{param}" syntax.
func (a *ChiAdapter) PUT(path string, h http.HandlerFunc) { a.register(http.MethodPut, path, h) }

// DELETE registers a new request handler for the HTTP DELETE method.
//
// The path provided supports the ":param" syntax, which is automatically
// converted to Chi's "{param}" syntax.
func (a *ChiAdapter) DELETE(path string, h http.HandlerFunc) {
	a.register(http.MethodDelete, path, h)
}

// HEAD registers a new request handler for the HTTP HEAD method.
//
// The path provided supports the ":param" syntax, which is automatically
// converted to Chi's "{param}" syntax.
func (a *ChiAdapter) HEAD(path string, h http.HandlerFunc) { a.register(http.MethodHead, path, h) }

// Use appends a middleware handler to the Chi middleware stack.
//
// The middleware will be applied to all handlers registered on this router
// and its sub-groups.
func (a *ChiAdapter) Use(mw internal.Middleware) {
	a.Router.Use(mw)
}

// Group creates a new sub-router mounted at the specified path.
//
// This allows for hierarchical routing. The new group inherits middleware
// from its parent.
//
// Note: The 'path' argument is also subject to syntax translation (adapting ":param" to "{param}")
// before the group is mounted.
func (a *ChiAdapter) Group(path string) internal.Router {
	r := chi.NewRouter()
	// In Chi, 'Mount' attaches a sub-router to a specific path pattern.
	a.Router.Mount(adaptPath(path), r)
	return &ChiAdapter{Router: r}
}

// Param retrieves the value of a URL path parameter from the request.
//
// It delegates to 'chi.URLParam'.
// Example:
//
//	Route: /user/:id
//	URL:   /user/42
//	Call:  Param(r, "id") -> "42"
func (a *ChiAdapter) Param(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// Serve starts a standard HTTP server using net/http.
//
// Since Chi is fully compatible with net/http.Handler, we can simply
// pass the router to ListenAndServe.
func (a *ChiAdapter) Serve(port string) error {
	return http.ListenAndServe(port, a.Router)
}
