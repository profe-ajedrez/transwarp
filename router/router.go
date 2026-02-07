package router

import "net/http"

type Router interface {
	http.Handler
	GET(path string, handler http.HandlerFunc)
	POST(path string, handler http.HandlerFunc)
	PUT(path string, handler http.HandlerFunc)
	HEAD(path string, handler http.HandlerFunc)
	DELETE(path string, handler http.HandlerFunc)
	Use(mw Middleware)
	Param(r *http.Request, key string) string
	Group(prefix string) Router
	Serve(port string) error
	Handle(pattern string, h http.Handler)
	HandleFunc(pattern string, h http.HandlerFunc)
}
