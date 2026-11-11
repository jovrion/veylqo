package ports

import "net/http"

// route is one ServeMux registration. pattern is the string Handler
// reports, "METHOD /path". Paths use the same {name} wildcards as the
// OpenAPI document so the contract test can compare them directly.
type route struct {
	method string
	path   string
}

func (r route) pattern() string {
	return r.method + " " + r.path
}

// apiRoutes are the only patterns NewMux registers.
var apiRoutes = []route{
	{method: http.MethodPost, path: "/api/v1/links"},
	{method: http.MethodGet, path: "/api/v1/links/{code}"},
	{method: http.MethodDelete, path: "/api/v1/links/{code}"},
	{method: http.MethodGet, path: "/api/v1/links/{code}/stats"},
}

// redirectPath is the public resolve pattern (ADR-0007). It is 302, never 301.
const redirectPath = "/{code}"

// redirectRoutes are the only patterns NewRedirectMux registers.
// Reserved names are registered so they do not fall through to GET /{code}.
func redirectRoutes() []route {
	routes := make([]route, 0, len(reservedRedirectPaths)+1)
	for _, path := range reservedRedirectPaths {
		routes = append(routes, route{method: http.MethodGet, path: path})
	}
	return append(routes, route{method: http.MethodGet, path: redirectPath})
}

// registeredRoutes is every pattern the link muxes register. cmd composes
// the two muxes; this list is their union. Do not add a Handle or
// HandleFunc that is not in apiRoutes or redirectRoutes.
func registeredRoutes() []route {
	routes := make([]route, 0, len(apiRoutes)+len(reservedRedirectPaths)+1)
	routes = append(routes, apiRoutes...)
	return append(routes, redirectRoutes()...)
}
