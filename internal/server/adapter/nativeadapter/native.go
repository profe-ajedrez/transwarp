package nativeadapter

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/profe-ajedrez/transwarp/internal"
)

var _ internal.Router = New()

// Regex para detectar parámetros estilo Gin (:id) y convertirlos a Go 1.22 ({id})
var paramRegex = regexp.MustCompile(`:([a-zA-Z0-9_]+)`)

// suffixCleaner Clean stuff after {} (ej: }.json)
var suffixCleaner = regexp.MustCompile(`\}[^/]+`)

type NativeAdapter struct {
	mux         *http.ServeMux
	prefix      string
	middlewares []internal.Middleware
}

// New crea una nueva instancia del adaptador
func New() *NativeAdapter {
	return &NativeAdapter{
		mux:         http.NewServeMux(),
		prefix:      "",
		middlewares: []internal.Middleware{},
	}
}

// ServeHTTP implementa la interfaz http.Handler
func (a *NativeAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// Group crea un sub-grupo de rutas con prefijo y middlewares heredados
func (a *NativeAdapter) Group(prefix string) internal.Router {
	newMws := make([]internal.Middleware, len(a.middlewares))
	copy(newMws, a.middlewares)

	return &NativeAdapter{
		mux:         a.mux,
		prefix:      a.joinPath(a.prefix, prefix),
		middlewares: newMws,
	}
}

// Use registra un middleware en la cadena actual
func (a *NativeAdapter) Use(mw internal.Middleware) {
	a.middlewares = append(a.middlewares, mw)
}

// HandleFunc registra un handler (Catch-All o Path específico)
func (a *NativeAdapter) HandleFunc(pattern string, h http.HandlerFunc) {
	// 1. Limpiamos y aplicamos prefijo
	cleanPath := a.transformPattern(pattern)
	fullPattern := a.joinPath(a.prefix, cleanPath)

	// 2. Delegamos al registro final (sin agregar método, actúa como match para todos)
	a.registerToMux(fullPattern, h)
}

// Handle implementa la interfaz http.Handler wrapper
func (a *NativeAdapter) Handle(pattern string, h http.Handler) {
	a.HandleFunc(pattern, h.ServeHTTP)
}

// Métodos para verbos específicos
func (a *NativeAdapter) GET(p string, h http.HandlerFunc)     { a.register("GET", p, h) }
func (a *NativeAdapter) POST(p string, h http.HandlerFunc)    { a.register("POST", p, h) }
func (a *NativeAdapter) PUT(p string, h http.HandlerFunc)     { a.register("PUT", p, h) }
func (a *NativeAdapter) DELETE(p string, h http.HandlerFunc)  { a.register("DELETE", p, h) }
func (a *NativeAdapter) PATCH(p string, h http.HandlerFunc)   { a.register("PATCH", p, h) }
func (a *NativeAdapter) HEAD(p string, h http.HandlerFunc)    { a.register("HEAD", p, h) }
func (a *NativeAdapter) OPTIONS(p string, h http.HandlerFunc) { a.register("OPTIONS", p, h) }

// register calcula el path y antepone el verbo HTTP correcto
func (a *NativeAdapter) register(method, pattern string, h http.HandlerFunc) {
	// 1. Limpieza de params (:id -> {id})
	cleanPath := a.transformPattern(pattern)

	// 2. Unión con el prefijo del grupo
	fullPath := a.joinPath(a.prefix, cleanPath)

	// 3. Formato Go 1.22: "METHOD /path"
	fullPattern := method + " " + fullPath

	// 4. Delegar al registro final
	a.registerToMux(fullPattern, h)
}

// registerToMux es el encargado final de aplicar middlewares y llamar al Mux nativo.
// NO modifica el patrón (asume que ya viene con prefijos y métodos correctos).
func (a *NativeAdapter) registerToMux(finalPattern string, h http.HandlerFunc) {
	finalHandler := h

	// Aplicamos middlewares (Onion pattern: último entra primero)
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		finalHandler = a.middlewares[i](finalHandler).ServeHTTP
	}

	a.mux.HandleFunc(finalPattern, finalHandler)
}

// Param recupera parámetros usando la nueva funcionalidad r.PathValue
func (a *NativeAdapter) Param(r *http.Request, key string) string {
	return r.PathValue(key)
}

// Serve inicia el servidor
func (a *NativeAdapter) Serve(port string) error {
	return http.ListenAndServe(port, a.mux)
}

// --- Helpers Privados ---

func (a *NativeAdapter) transformPattern(p string) string {
	// 1. Paso 1: Convertir estilo Gin (:id) a estilo Go ({id})
	// Entrada: "/v1/product/:id.json"
	// Salida:  "/v1/product/{id}.json"
	converted := paramRegex.ReplaceAllString(p, "{$1}")

	// 2. Paso 2: Limpieza agresiva de sufijos para evitar el Panic
	// El ServeMux de Go ODIA "{id}.json".
	// Esta línea busca "}.json" (o cualquier cosa pegada) y la reemplaza solo por "}"

	if strings.Contains(converted, "}") {
		// Entrada: "/v1/product/{id}.json"
		// Match del regex: "}.json"
		// Reemplazo: "}"
		// Resultado final: "/v1/product/{id}"
		converted = suffixCleaner.ReplaceAllString(converted, "}")
	}

	return converted
}

func (a *NativeAdapter) joinPath(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}

	cleanPrefix := strings.TrimSuffix(prefix, "/")
	cleanPath := path
	if !strings.HasPrefix(path, "/") {
		cleanPath = "/" + path
	}

	return cleanPrefix + cleanPath
}
