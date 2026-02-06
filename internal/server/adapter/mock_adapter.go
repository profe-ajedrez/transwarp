package adapter

import (
	"net/http"

	"github.com/profe-ajedrez/transwarp/internal"
)

// Exportamos la clave y el tipo para usarlos en los tests
type MockKey string

const MockParamsKey MockKey = "mock_params"

type MockRouter struct {
	// Handlers: Mapa compartido entre grupos (por eso es un puntero a mapa si lo reinicias,
	// pero como maps son referencia, basta con pasarlo)
	Handlers map[string]http.HandlerFunc

	// Estado interno para manejar grupos
	prefix      string
	middlewares []internal.Middleware
}

func NewMockRouter() *MockRouter {
	return &MockRouter{
		Handlers:    make(map[string]http.HandlerFunc),
		prefix:      "",
		middlewares: []internal.Middleware{},
	}
}

// Group: Ahora concatena el prefijo y COPIA los middlewares heredados
func (m *MockRouter) Group(path string) internal.Router {
	// Copiamos los middlewares para que los nuevos del grupo no afecten al padre
	newMws := make([]internal.Middleware, len(m.middlewares))
	copy(newMws, m.middlewares)

	return &MockRouter{
		Handlers:    m.Handlers,      // Compartimos el MISMO mapa de rutas
		prefix:      m.prefix + path, // Acumulamos el prefijo (ej: "" -> "/api" -> "/api/admin")
		middlewares: newMws,
	}
}

// Use: Agrega middlewares a la pila actual
func (m *MockRouter) Use(mw internal.Middleware) {
	m.middlewares = append(m.middlewares, mw)
}

// Helper interno para registrar aplicando middlewares y prefijos
func (m *MockRouter) register(method, path string, h http.HandlerFunc) {
	fullPath := m.prefix + path
	key := method + " " + fullPath

	// Composición de Middlewares (Onion Layering)
	// Envolvemos el handler original con los middlewares acumulados
	finalHandler := http.Handler(h)

	// Iteramos al revés para que el primer Use sea el más externo
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		finalHandler = m.middlewares[i](finalHandler)
	}

	// Guardamos el handler final (ya envuelto) en el mapa
	m.Handlers[key] = finalHandler.ServeHTTP
}

// Verbos HTTP
func (m *MockRouter) GET(path string, h http.HandlerFunc)    { m.register("GET", path, h) }
func (m *MockRouter) POST(path string, h http.HandlerFunc)   { m.register("POST", path, h) }
func (m *MockRouter) PUT(path string, h http.HandlerFunc)    { m.register("PUT", path, h) }
func (m *MockRouter) DELETE(path string, h http.HandlerFunc) { m.register("DELETE", path, h) }
func (m *MockRouter) PATCH(path string, h http.HandlerFunc)  { m.register("PATCH", path, h) }
func (m *MockRouter) HEAD(path string, h http.HandlerFunc)   { m.register("HEAD", path, h) }

func (m *MockRouter) Serve(port string) error { return nil }

// Param: Lee del contexto inyectado
func (m *MockRouter) Param(r *http.Request, key string) string {
	if params, ok := r.Context().Value(MockParamsKey).(map[string]string); ok {
		return params[key]
	}
	return ""
}
