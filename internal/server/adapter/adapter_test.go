package adapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v3"
	echo "github.com/labstack/echo/v5"

	"github.com/profe-ajedrez/transwarp/internal"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/chiadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/echoadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/fiberadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/ginadapter"
)

// testKey is used for context keys to avoid collisions during middleware testing.
type testKey int

const (
	Trace = testKey(77777)
)

// Executor defines the signature for a function that executes an HTTP request
// against a specific adapter implementation.
//
// This abstraction allows the test suite to be agnostic of the underlying
// execution model (e.g., ServeHTTP for Gin/Echo/Chi vs. direct client calls for Fiber).
type Executor func(req *http.Request) *http.Response

// TestAllAdapters is the main entry point for the integration test suite.
//
// Strategy:
// 1. It iterates through all supported drivers (Gin, Echo, Fiber, Chi, Mock).
// 2. For each driver, it initializes the specific engine and wraps it in the Transwarp adapter.
// 3. It calls 'setupUniversalRoutes' to register a standardized set of routes.
// 4. It defines a specific 'Executor' closure that knows how to send requests to that engine.
// 5. It calls 'executeUniversalTests', running the exact same battery of assertions against all drivers.
func TestAllAdapters(t *testing.T) {

	// --- GIN ADAPTER TEST ---
	t.Run("Gin", func(t *testing.T) {
		gin.SetMode(gin.TestMode) // Suppress debug logs
		g := gin.New()
		r := &ginadapter.GinAdapter{Router: g}

		setupUniversalRoutes(r)

		// Gin implements http.Handler, so we can use httptest.ResponseRecorder directly.
		executeUniversalTests(t, func(req *http.Request) *http.Response {
			rec := httptest.NewRecorder()
			g.ServeHTTP(rec, req)
			return rec.Result()
		})
	})

	// --- ECHO ADAPTER TEST ---
	t.Run("EchoV5", func(t *testing.T) {
		e := echo.New()
		// e.HideBanner = true // Optional: clean logs
		r := &echoadapter.EchoAdapter{Instance: e}

		setupUniversalRoutes(r)

		// Echo implements http.Handler.
		executeUniversalTests(t, func(req *http.Request) *http.Response {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			return rec.Result()
		})
	})

	// --- FIBER ADAPTER TEST ---
	// Special Case: Fiber (v2/v3) does NOT implement http.Handler.
	// It runs its own fasthttp server. Therefore, we must start a real TCP listener
	// and make actual HTTP client requests.
	t.Run("FiberV3", func(t *testing.T) {
		app := fiber.New(fiber.Config{})
		r := &fiberadapter.FiberAdapter{App: app, Router: app}

		setupUniversalRoutes(r)

		port := ":9988"
		// Start Fiber in a goroutine
		go func() { _ = app.Listen(port) }()
		time.Sleep(100 * time.Millisecond) // Give it time to bind port

		defer func() { _ = app.Shutdown() }()

		executeUniversalTests(t, func(req *http.Request) *http.Response {
			// Transform the request URL to point to the local TCP port
			u := req.URL
			u.Scheme = "http"
			u.Host = "localhost" + port

			// Clone the request to avoid mutating the original test definition
			newReq, _ := http.NewRequest(req.Method, u.String(), req.Body)
			newReq.Header = req.Header

			// Important: Disable Keep-Alive to prevent connection exhaustion during
			// rapid-fire tests (like the concurrency test).
			newReq.Close = true

			resp, err := http.DefaultClient.Do(newReq)
			if err != nil {
				t.Fatalf("Fiber connection error: %v", err)
			}
			return resp
		})
	})

	// --- CHI ADAPTER TEST ---
	t.Run("Chi", func(t *testing.T) {
		c := chi.NewRouter()
		var r internal.Router = &chiadapter.ChiAdapter{Router: c}

		setupUniversalRoutes(r)

		// Chi implements http.Handler.
		executeUniversalTests(t, func(req *http.Request) *http.Response {
			rec := httptest.NewRecorder()
			c.ServeHTTP(rec, req)
			return rec.Result()
		})
	})

	// --- MOCK ROUTER TEST ---
	// This tests the internal Mock implementation used for unit testing.
	// Since the MockRouter is a manual implementation, we must simulate the routing logic manually here
	// to ensure the Mock behaves correctly when users use it.
	t.Run("MockRouter", func(t *testing.T) {
		m := adapter.NewMockRouter()
		setupUniversalRoutes(m)

		executeUniversalTests(t, func(req *http.Request) *http.Response {
			path := req.URL.Path
			method := req.Method
			var key string

			// 1. Thread-safe parameter storage for this request
			currentParams := make(map[string]string)

			// 2. Manual Routing Logic (Simulating what a real router does)
			switch {
			// Basic Echo Route
			case strings.HasPrefix(path, "/api/echo/"):
				key = "GET /api/echo/:data"
				currentParams["data"] = strings.TrimPrefix(path, "/api/echo/")

			// Deeply Nested Param Route
			case strings.HasPrefix(path, "/api/shop/category/"):
				key = "GET /api/shop/category/:cat/item/:id"
				p := strings.Split(path, "/")
				if len(p) >= 7 {
					currentParams["cat"], currentParams["id"] = p[4], p[6]
				}

			// Method Specific Routes
			case path == "/api/users" && method == "POST":
				key = "POST /api/users"
			case path == "/api/update" && method == "PUT":
				key = "PUT /api/update"
			case strings.HasPrefix(path, "/api/remove/") && method == "DELETE":
				key = "DELETE /api/remove/:id"
				currentParams["id"] = strings.TrimPrefix(path, "/api/remove/")

			// Exact Matches
			case path == "/api/secret":
				key = "GET /api/secret"
			case path == "/api/search":
				key = "GET /api/search"
			case path == "/api/admin/settings":
				key = "GET /api/admin/settings"

			// Dynamic Match
			case strings.HasPrefix(path, "/api/admin/"):
				key = "GET /api/admin/:any"
				currentParams["any"] = strings.TrimPrefix(path, "/api/admin/")

			// --- Firewall Test Route ---
			case strings.HasPrefix(path, "/protected/"):
				key = "GET /protected/dashboard"
				// Note: query params are handled by the middleware registered in setupUniversalRoutes

			// --- Collision Tests ---
			// Specific route must be matched before generic prefix
			case path == "/files/config":
				key = "GET /files/config"

			case strings.HasPrefix(path, "/files/"):
				key = "GET /files/:name"
				currentParams["name"] = strings.TrimPrefix(path, "/files/")

			// Default: Exact path match fallback
			default:
				key = method + " " + path
			}

			// Lookup the handler in the Mock's map
			h, ok := m.Handlers[key]
			if !ok {
				// Debugging aid
				fmt.Printf("[Mock 404] Looking for key: '%s'. Original Path: '%s'\n", key, path)
				return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("404"))}
			}

			// 3. Inject Context (Thread-safe)
			// We inject the captured parameters into the request context so the Mock's Param() method can find them.
			ctx := context.WithValue(req.Context(), adapter.MockParamsKey, currentParams)

			rec := httptest.NewRecorder()
			h(rec, req.WithContext(ctx))
			return rec.Result()
		})
	})
}

// setupUniversalRoutes registers a comprehensive set of routes covering common scenarios:
// - Global & Group Middleware
// - URL Parameters & Query Params
// - JSON Body Parsing
// - Header Manipulation
// - Deep Nesting
// - Route Collision (Static vs Dynamic)
// - Middleware Interruption (Firewall)
func setupUniversalRoutes(r internal.Router) {
	// 1. Global Middleware: Injects a header into every response
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), Trace, "root")
			w.Header().Set("X-Powered-By", "Transwarp")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	api := r.Group("/api")

	// Simple Echo Route
	api.GET("/echo/:data", func(w http.ResponseWriter, req *http.Request) {
		data := r.Param(req, "data")
		w.Write([]byte(data))
	})

	// Query Params Test
	api.GET("/search", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query().Get("q")
		page := req.URL.Query().Get("page")
		fmt.Fprintf(w, "q:%s|page:%s", q, page)
	})

	// JSON Body Test
	api.POST("/users", func(w http.ResponseWriter, req *http.Request) {
		type User struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}
		var u User
		if err := json.NewDecoder(req.Body).Decode(&u); err != nil {
			http.Error(w, "Bad JSON", http.StatusBadRequest)
			return
		}
		u.Role = "super_" + u.Role
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	})

	// Headers Auth Test
	api.GET("/secret", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer 123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("granted"))
	})

	// Groups & Params
	admin := api.Group("/admin")
	admin.GET("/settings", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("static_settings")) })
	admin.GET("/:any", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("dynamic_any")) })

	shop := api.Group("/shop")
	shop.GET("/category/:cat/item/:id", func(w http.ResponseWriter, req *http.Request) {
		cat := r.Param(req, "cat")
		id := r.Param(req, "id")
		fmt.Fprintf(w, "cat:%s|id:%s", cat, id)
	})

	api.PUT("/update", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	api.DELETE("/remove/:id", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// --- Firewall Test (Middleware Interruption) ---
	protected := r.Group("/protected")

	// This middleware BLOCKS the request if ?admin=true is missing.
	protected.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Query().Get("admin") != "true" {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("bloqueado"))
				// RETURN here is crucial. In Gin/Fiber adapters, this must trigger an abort.
				return
			}
			next.ServeHTTP(w, req)
		})
	})

	protected.GET("/dashboard", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dashboard_data"))
	})

	// --- Collision Test (Static vs Dynamic) ---
	files := r.Group("/files")

	// Static route (Should have priority)
	files.GET("/config", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("file_config"))
	})

	// Dynamic route (Conflicting with /config)
	files.GET("/:name", func(w http.ResponseWriter, req *http.Request) {
		name := r.Param(req, "name")
		w.WriteHeader(http.StatusCreated) // Expecting 201
		fmt.Fprintf(w, "created_%s", name)
	})
}

// executeUniversalTests runs the battery of assertions using the provided Executor.
func executeUniversalTests(t *testing.T, executor Executor) {

	// Helper to create simple requests
	simpleReq := func(method, path string) *http.Request {
		req, _ := http.NewRequest(method, path, nil)
		return req
	}

	readBody := func(res *http.Response) string {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return string(b)
	}

	// Test 1: Middleware Chain & Header Injection
	t.Run("Middleware_Headers", func(t *testing.T) {
		resp := executor(simpleReq("GET", "/api/echo/hello"))
		if resp.Header.Get("X-Powered-By") != "Transwarp" {
			t.Error("Middleware global header missing")
		}
	})

	// Test 2: Query String Parsing
	t.Run("Query_Params", func(t *testing.T) {
		resp := executor(simpleReq("GET", "/api/search?q=go&page=1"))
		if body := readBody(resp); body != "q:go|page:1" {
			t.Errorf("Query params failed. Got: %s", body)
		}
	})

	// Test 3: JSON Body Parsing & Response
	t.Run("JSON_Body", func(t *testing.T) {
		payload := []byte(`{"name":"Jacob","role":"admin"}`)
		req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")

		resp := executor(req)

		if resp.StatusCode != 200 {
			t.Fatalf("JSON POST status wrong: %d", resp.StatusCode)
		}

		var resMap map[string]string
		json.NewDecoder(resp.Body).Decode(&resMap)

		if resMap["role"] != "super_admin" {
			t.Errorf("JSON logic failed. Got role: %s", resMap["role"])
		}
	})

	// Test 4: Custom Headers & Auth Logic
	t.Run("Custom_Headers", func(t *testing.T) {
		// Failure Case
		req1, _ := http.NewRequest("GET", "/api/secret", nil)
		if resp := executor(req1); resp.StatusCode != 401 {
			t.Errorf("Auth should fail without header")
		}

		// Success Case
		req2, _ := http.NewRequest("GET", "/api/secret", nil)
		req2.Header.Set("Authorization", "Bearer 123")
		if resp := executor(req2); resp.StatusCode != 200 {
			t.Errorf("Auth should pass with header")
		}
	})

	// Test 5: Route Collision (Static should beat Dynamic)
	t.Run("Static_Vs_Dynamic", func(t *testing.T) {
		if b := readBody(executor(simpleReq("GET", "/api/admin/settings"))); b != "static_settings" {
			t.Error("Static route failed")
		}
		if b := readBody(executor(simpleReq("GET", "/api/admin/random"))); b != "dynamic_any" {
			t.Error("Dynamic route failed")
		}
	})

	// Test 6: Deep Nesting & Multiple Parameters
	t.Run("Multi_Param", func(t *testing.T) {
		resp := executor(simpleReq("GET", "/api/shop/category/books/item/42"))
		if b := readBody(resp); b != "cat:books|id:42" {
			t.Errorf("Multi param failed. Got: %s", b)
		}
	})

	// Test 7: Concurrency Safety
	// Critical for engines like Fiber/Echo that recycle request contexts.
	t.Run("Concurrency_Safe", func(t *testing.T) {
		var wg sync.WaitGroup
		count := 20
		errors := make(chan error, count)

		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				target := fmt.Sprintf("val_%d", val)
				resp := executor(simpleReq("GET", "/api/echo/"+target))
				body := readBody(resp)
				if body != target {
					errors <- fmt.Errorf("Race: expected %s got %s", target, body)
				}
			}(i)
		}
		wg.Wait()
		close(errors)
		for err := range errors {
			t.Error(err)
			break
		}
	})

	// Test 8: Middleware Interruption (Firewall)
	// Ensures that if a middleware does NOT call next(), the chain stops.
	t.Run("Middleware_Firewall", func(t *testing.T) {
		// Blocked
		respBlocked := executor(simpleReq("GET", "/protected/dashboard"))
		if respBlocked.StatusCode != http.StatusForbidden {
			t.Errorf("Firewall failed. Expected 403, got %d", respBlocked.StatusCode)
		}
		if b := readBody(respBlocked); b != "bloqueado" {
			t.Errorf("Error body incorrect: %s", b)
		}

		// Allowed
		respOk := executor(simpleReq("GET", "/protected/dashboard?admin=true"))
		if respOk.StatusCode != http.StatusOK {
			t.Errorf("Valid auth failed. Expected 200, got %d", respOk.StatusCode)
		}
		if b := readBody(respOk); b != "dashboard_data" {
			t.Errorf("Protected data incorrect: %s", b)
		}
	})

	// Test 9: Files Collision & Status Codes
	t.Run("Status_And_Collision", func(t *testing.T) {
		// Static Priority
		respConfig := executor(simpleReq("GET", "/files/config"))
		if b := readBody(respConfig); b != "file_config" {
			t.Errorf("Static collision failed. Expected 'file_config', got '%s'", b)
		}

		// Dynamic Fallback
		respFile := executor(simpleReq("GET", "/files/document.pdf"))
		if respFile.StatusCode != http.StatusCreated {
			t.Errorf("Status Code failed. Expected 201, got %d", respFile.StatusCode)
		}
		if b := readBody(respFile); b != "created_document.pdf" {
			t.Errorf("Dynamic param failed: %s", b)
		}
	})
}
