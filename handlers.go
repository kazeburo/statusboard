package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (o *Opt) handleJSON(c echo.Context) error {
	return c.JSON(http.StatusOK, o.config)
}

func (o *Opt) handleIndex(c echo.Context) error {
	return c.HTMLBlob(http.StatusOK, o.htmlBlob)
}

func (o *Opt) startServer(ctx context.Context) error {
	e := echo.New()
	e.HideBanner = true
	e.Debug = false

	skipper := func(c echo.Context) bool {
		switch c.Path() {
		case "/favicon.ico", "/live":
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

	c := make(chan error, 1)
	go func() {
		c <- e.Start(o.Listen)
	}()
	var err error
	select {
	case <-ctx.Done():
		bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = e.Shutdown(bg)
	case err = <-c:
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
