package main

import (
	_ "embed"
	"io"
	"net/http"
	"text/template"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
)

//go:embed files/index.html
var indexhtml []byte

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func (o *Opt) handleJSON(c echo.Context) error {
	return c.JSON(http.StatusOK, o.config)
}

func (o *Opt) handleIndex(c echo.Context) error {
	return c.Render(http.StatusOK, "index", o.config)
}

func (o *Opt) httpserver() error {
	e := echo.New()
	e.Debug = false

	renderer := &Template{
		templates: template.Must(template.New("index").Parse(string(indexhtml))),
	}
	e.Renderer = renderer

	skipper := func(c echo.Context) bool {
		switch c.Path() {
		case "/", "/live":
			return true
		default:
			return false
		}
	}

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: skipper,
	}))
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", o.handleIndex)
	e.GET("/_json", o.handleJSON)

	// Start server
	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
