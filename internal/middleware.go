package internal

import "net/http"

// Middleware define la firma estándar de Go para funciones de envoltura

type Middleware func(http.Handler) http.Handler
