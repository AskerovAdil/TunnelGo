package web

import (
	"embed"
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Renderer struct {
	templates *template.Template
}

func NewRenderer() (*Renderer, error) {
	tpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tpl}, nil
}

func (r *Renderer) Render(w io.Writer, name string, data any, _ echo.Context) error {
	return r.templates.ExecuteTemplate(w, name, data)
}
