package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed all:templates
var TemplatesFS embed.FS

//go:embed all:static
var StaticFS embed.FS

func LoadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"dict": dict,
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
	}
	return template.New("").Funcs(funcMap).ParseFS(TemplatesFS, "templates/*.html", "templates/**/*.html")
}

func StaticHandler() http.Handler {
	sub, err := fs.Sub(StaticFS, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}

func dict(values ...any) map[string]any {
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		if i+1 < len(values) {
			m[values[i].(string)] = values[i+1]
		}
	}
	return m
}
