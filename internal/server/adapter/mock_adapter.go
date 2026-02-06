package adapter

import (
	"net/http"
	"strings"

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

// Lista de métodos soportados por el Mock
var mockMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

func (m *MockRouter) Handle(pattern string, h http.Handler) {
	m.HandleFunc(pattern, h.ServeHTTP)
}

func (m *MockRouter) HandleFunc(pattern string, h http.HandlerFunc) {
	// Registramos la ruta para cada verbo HTTP
	for _, method := range mockMethods {
		// Usamos el método register interno que creamos antes
		// para que aplique middlewares y prefijos correctamente
		m.register(method, pattern, h)
	}
}

// ServeHTTP permite que el MockRouter cumpla con la interfaz http.Handler.
// Esto es vital para usarlo con httptest.NewRecorder() y en benchmarks.
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Construimos la clave de búsqueda: "METODO /ruta"
	// Nota: Esta es una implementación simplificada para tests.
	// No maneja patrones complejos como :params en la búsqueda (matching),
	// solo coincidencia exacta de rutas registradas o la lógica específica que definas.

	// Si tu Mock soporta parámetros (ej. /users/:id), aquí deberías tener
	// una lógica básica para resolverlos. Para benchmarks exactos,
	// asumimos que registraste la ruta exacta o tienes una lógica de "best match".

	key := r.Method + " " + r.URL.Path

	// 2. Buscamos el handler
	if handler, exists := m.Handlers[key]; exists {
		// Ejecutamos el handler encontrado
		handler(w, r)
		return
	}

	// 3. Fallback: Intentar buscar rutas con parámetros (Lógica simple para Mock)
	// Si no encuentras la ruta exacta, iteras para ver si alguna coincide con patrón
	for routeKey, h := range m.Handlers {
		// routeKey es ej: "GET /users/:id"
		// r.Method + r.URL.Path es ej: "GET /users/123"

		// Separamos método y path
		parts := strings.SplitN(routeKey, " ", 2)
		if len(parts) != 2 || parts[0] != r.Method {
			continue
		}

		pattern := parts[1] // "/users/:id"

		// Chequeo muy básico de prefijo para simular match dinámico
		// (Para un mock robusto, podrías usar regex, pero esto suele bastar para tests)
		if strings.Contains(pattern, ":") {
			base := strings.Split(pattern, ":")[0] // "/users/"
			if strings.HasPrefix(r.URL.Path, base) {
				// Encontramos un candidato "parecido"
				h(w, r)
				return
			}
		}
	}

	// 4. Si no existe, 404
	http.NotFound(w, r)
}
