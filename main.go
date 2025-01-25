package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
)

var version string

type Opt struct {
	Toml    string `long:"toml" description:"file path to toml file" required:"true"`
	Version bool   `short:"v" long:"version" description:"Show version"`
	config  *Config
}

func printVersion() {
	fmt.Printf(`%s %s
Compiler: %s %s
`,
		os.Args[0],
		version,
		runtime.Compiler,
		runtime.Version())
}

type Config struct {
	Title      string     `toml:"title"`
	Categories []Category `toml:"category"`
}

type Category struct {
	Name     string    `toml:"name"`
	Comment  string    `toml:"comment"`
	Services []Service `toml:"service"`
}

type Service struct {
	Name    string   `toml:"name"`
	Command []string `toml:"command"`
}

func (o *Opt) httpserver() error {
	e := echo.New()
	e.Use(middleware.Logger())

	// Routes
	e.GET("/", o.handle_index)

	// Start server
	if err := e.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func loadTomp(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not open toml")
	}
	defer file.Close()

	var conf Config
	if _, err := toml.NewDecoder(file).Decode(&conf); err != nil {
		return nil, errors.Wrap(err, "failed to decode toml")
	}

	return &conf, nil
}

func _main() int {
	opt := Opt{}
	psr := flags.NewParser(&opt, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opt.Version {
		printVersion()
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	conf, err := loadTomp(opt.Toml)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	opt.config = conf

	return 0
}

func main() {
	os.Exit(_main())
}
