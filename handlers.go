package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (o *Opt) ifModifiedSince(r *http.Request) bool {
	ims := r.Header.Get("If-Modified-Since")
	if ims == "" {
		return true
	}
	t, err := http.ParseTime(ims)
	if err != nil {
		return true
	}
	lm := o.config.LastUpdatedAt.Truncate(time.Second)
	if ret := lm.Compare(t); ret <= 0 {
		return false
	}
	return true
}

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

	// Route level middleware
	conditionalGET := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !o.ifModifiedSince(c.Request()) {
				return c.NoContent(http.StatusNotModified)
			}
			c.Response().Header().Set("Last-Modified", o.config.LastUpdatedAt.UTC().Format(http.TimeFormat))
			return next(c)
		}
	}
	// Routes
	e.GET("/", o.handleIndex, conditionalGET)
	e.GET("/_json", o.handleJSON, conditionalGET)

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
