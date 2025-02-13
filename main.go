package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/sync/errgroup"
)

var version string

type Opt struct {
	Listen   string `short:"l" long:"listen" default:":8080" description:"address:port to bind"`
	Toml     string `long:"toml" description:"file path to toml file" required:"true"`
	Data     string `long:"data" description:"file path to data dir" required:"true"`
	Version  bool   `short:"v" long:"version" description:"Show version"`
	config   *Config
	htmlBlob []byte
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

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d *duration) IsZero() bool {
	return d.Duration == 0
}

func (d *duration) ShortString() string {
	s := d.Duration.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func MustDuration(s string) duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("failed to parse duration: %v", err))
	}
	return duration{d}
}

type markdown struct {
	original string
	html     string
}

func MustMarkdown(s string) *markdown {
	m := &markdown{}
	err := m.UnmarshalText([]byte(s))
	if err != nil {
		panic(fmt.Sprintf("failed to convert markdown: %v", err))
	}
	return m
}

func (m *markdown) UnmarshalText(source []byte) error {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return err
	}
	m.html = buf.String()
	m.original = string(source)
	return nil
}

func (m *markdown) HTML() template.HTML {
	return template.HTML(m.html)
}

func (m *markdown) IsEmpty() bool {
	return m.original == ""
}

type statusText struct {
	string string
}

func StatusText(s string) *statusText {
	return &statusText{s}
}

var NoDATA = StatusText("NoData")
var Warning = StatusText("Warning")
var Operational = StatusText("Operational")

func (s *statusText) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.string + `"`), nil
}

func (s *statusText) String() string {
	return s.string
}

func (s *statusText) IsOperational() bool {
	return s == Operational
}

func (s *statusText) IsWarning() bool {
	return s == Warning
}

type Config struct {
	Title            string     `toml:"title" json:"title"`
	NavTitle         *markdown  `toml:"nav_title" json:"-"`
	NavButtonName    string     `toml:"nav_button_name" json:"-"`
	NavButtonLink    string     `toml:"nav_button_link" json:"-"`
	HeaderMessage    *markdown  `toml:"header_message" json:"-"`
	FooterMessage    *markdown  `toml:"footer_message" json:"-"`
	PoweredBy        *markdown  `toml:"powered_by" json:"-"`
	Categories       []Category `toml:"category" json:"categories"`
	WorkerInterval   duration   `toml:"worker_interval" json:"-"`
	WorkerTimeout    duration   `toml:"worker_timeout" json:"-"`
	NumOfWorker      int        `toml:"num_of_worker" json:"-"`
	MaxCheckAttempts int        `toml:"max_check_attempts" json:"-"`
	RetryInterval    duration   `toml:"retry_interval" json:"-"`
	LatestTimeRange  duration   `toml:"latest_time_range" json:"-"`
	Days             []string   `json:"days"`
	LastUpdatedAt    time.Time  `json:"last_updated_at"`
}

type Category struct {
	Name     string     `toml:"name" json:"name"`
	Comment  string     `toml:"comment" json:"comment"`
	Services []*Service `toml:"service" json:"services"`
}

type Service struct {
	categoryName   string
	Name           string        `toml:"name" json:"name"`
	Command        []string      `toml:"command" json:"-"`
	LatestStatus   *statusText   `json:"latest_status"`
	LatestStatusAt time.Time     `json:"latest_status_at"`
	StatusHistory  []*statusText `json:"status_history"`
}

type ServiceLog struct {
	Time         time.Time `json:"time"`
	CategoryName string    `json:"category_name"`
	Name         string    `json:"name"`
	Command      []string  `json:"command"`
	Status       int       `json:"status"`
	Message      string    `json:"message"`
}

func loadToml(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "could not open toml")
	}
	defer file.Close()

	var conf Config
	if _, err := toml.NewDecoder(file).Decode(&conf); err != nil {
		return nil, errors.Wrap(err, "failed to decode toml")
	}

	for _, categeory := range conf.Categories {
		for _, service := range categeory.Services {
			service.categoryName = categeory.Name
		}
	}

	if conf.NumOfWorker == 0 {
		conf.NumOfWorker = 4
	}
	if conf.WorkerInterval.IsZero() {
		conf.WorkerInterval = MustDuration("5m")
	}
	if conf.WorkerTimeout.IsZero() {
		conf.WorkerTimeout = MustDuration("30s")
	}
	if conf.LatestTimeRange.IsZero() {
		conf.LatestTimeRange = MustDuration("1h")
	}

	if conf.MaxCheckAttempts == 0 {
		conf.MaxCheckAttempts = 3
	}
	if conf.RetryInterval.IsZero() {
		conf.RetryInterval = MustDuration("5s")
	}
	if conf.PoweredBy == nil || conf.PoweredBy.IsEmpty() {
		conf.PoweredBy = MustMarkdown("Powered by statusboard.")
	}
	conf.LastUpdatedAt = time.Now()

	return &conf, nil
}

func _main() int {
	opt := &Opt{}
	psr := flags.NewParser(opt, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opt.Version {
		printVersion()
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	conf, err := loadToml(opt.Toml)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	opt.config = conf

	// check open file in data dir
	err = opt.createServiceLog()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// render html
	err = opt.renderStatusPage(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// run
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer done()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return opt.startWorker(ctx)
	})
	g.Go(func() error {
		return opt.startServer(ctx)
	})
	if err := g.Wait(); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Warn("error in service", slog.Any("error", err))
			return 1
		}
	}
	return 0
}

func main() {
	os.Exit(_main())
}
