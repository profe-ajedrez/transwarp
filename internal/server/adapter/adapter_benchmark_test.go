package adapter_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v3"
	echo "github.com/labstack/echo/v5"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/chiadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/echoadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/fiberadapter"
	"github.com/profe-ajedrez/transwarp/internal/server/adapter/ginadapter"
)

// Escenarios de prueba basados en setupUniversalRoutes
var scenarios = []struct {
	name   string
	method string
	path   string
	body   []byte
}{
	{"Echo_Simple", "GET", "/api/echo/benchmark", nil},
	{"Query_Params", "GET", "/api/search?q=golang&page=1", nil},
	{"Deep_Param", "GET", "/api/shop/category/books/item/12345", nil},
	{"Static_Route", "GET", "/api/admin/settings", nil},
	{"JSON_Body", "POST", "/api/users", []byte(`{"name":"Bench","role":"user"}`)},
	{"Handle_All", "PUT", "/universal", nil}, // Prueba del HandleFunc genérico
}

func BenchmarkAdapters(b *testing.B) {
	// 1. GIN
	b.Run("Gin", func(b *testing.B) {
		gin.SetMode(gin.ReleaseMode) // Importante para benchmark real
		g := gin.New()
		r := &ginadapter.GinAdapter{Router: g}
		setupUniversalRoutes(r)
		runBenchmarkStandard(b, g)
	})

	// 2. FIBER v3
	b.Run("Fiber", func(b *testing.B) {
		app := fiber.New()
		r := &fiberadapter.FiberAdapter{App: app, Router: app}
		setupUniversalRoutes(r)

		// Fiber usa una lógica distinta (app.Test en lugar de ServeHTTP)
		runBenchmarkFiber(b, app)
	})

	// 3. ECHO
	b.Run("Echo", func(b *testing.B) {
		e := echo.New()
		// e.Logger.SetLevel(log.OFF)
		r := &echoadapter.EchoAdapter{Instance: e}
		setupUniversalRoutes(r)
		runBenchmarkStandard(b, e)
	})

	// 4. CHI
	b.Run("Chi", func(b *testing.B) {
		c := chi.NewRouter()
		r := &chiadapter.ChiAdapter{Router: c}
		setupUniversalRoutes(r)
		runBenchmarkStandard(b, c)
	})

	// 5. MOCK (Referencia base)
	b.Run("Mock", func(b *testing.B) {
		m := adapter.NewMockRouter()
		setupUniversalRoutes(m)
		// El mock también implementa ServeHTTP
		runBenchmarkStandard(b, m)
	})
}

// runBenchmarkStandard ejecuta el loop para routers compatibles con net/http
func runBenchmarkStandard(b *testing.B, handler http.Handler) {
	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			// Preparación (fuera del timer si es posible, pero NewRequest es rápido)
			req, _ := http.NewRequest(sc.method, sc.path, bytes.NewReader(sc.body))
			if sc.method == "POST" {
				req.Header.Set("Content-Type", "application/json")
			}

			// ResponseRecorder reutilizable (opcional, aquí creamos uno nuevo por iteración)
			// para simular carga real de allocs
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				// Clonamos el request o creamos uno nuevo si el router ensucia el contexto
				// Para routers estándar, reutilizar el mismo struct request a veces es peligroso
				// si modifican el ctx. Lo más seguro para bench es crear request en loop
				// o resetear el contexto, pero NewRequest añade overhead constante a TODOS.

				// handler.ServeHTTP(w, req) <--- Reutilizar req puede fallar en Gin/Echo

				// Creamos request ligero dentro del loop para ser justos con Fiber
				r, _ := http.NewRequest(sc.method, sc.path, bytes.NewReader(sc.body))
				if sc.body != nil {
					r.Header.Set("Content-Type", "application/json")
				}
				handler.ServeHTTP(w, r)
			}
		})
	}
}

// runBenchmarkFiber ejecuta el loop específico para Fiber
func runBenchmarkFiber(b *testing.B, app *fiber.App) {
	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Creamos el request estándar
				req, _ := http.NewRequest(sc.method, sc.path, bytes.NewReader(sc.body))
				if sc.body != nil {
					req.Header.Set("Content-Type", "application/json")
				}

				// app.Test inyecta el http.Request directamente en Fasthttp
				resp, _ := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
				if err := resp.Body.Close(); err != nil {
					panic(err)
				}
			}
		})
	}
}
