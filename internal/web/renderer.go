package web

import (
	"embed"
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

type Renderer struct {
	templates *template.Template
}

func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"formatBytes":    FormatBytes,
		"formatTime":     FormatTime,
		"formatRelative": FormatRelative,
	}
	tpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tpl}, nil
}

func (r *Renderer) Render(w io.Writer, name string, data any, _ echo.Context) error {
	return r.templates.ExecuteTemplate(w, name, data)
}
