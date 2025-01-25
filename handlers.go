package main

import (
	_ "embed"

	"github.com/labstack/echo/v4"
)

//go:embed files/index.html
var indexhtml []byte

func (o *Opt) handle_index(c echo.Context) error {
	return nil
}
