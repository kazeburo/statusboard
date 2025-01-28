package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var version string

type Opt struct {
	Toml          string `long:"toml" description:"file path to toml file" required:"true"`
	Data          string `long:"data" description:"file path to data dir" required:"true"`
	Version       bool   `short:"v" long:"version" description:"Show version"`
	config        *Config
	workerChannel chan Service
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

type markdown struct {
	original string
	html     string
}

func (m *markdown) UnmarshalText(source []byte) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
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

var NoDATA = StatusText("no data")
var Warning = StatusText("Warning")
var Operational = StatusText("Operational")

type statusText struct {
	string
}

func StatusText(s string) *statusText {
	return &statusText{s}
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
func (m *markdown) HTML() template.HTML {
	return template.HTML(m.html)
}

func (m *markdown) Plain() string {
	return m.original
}

type Config struct {
	Title            string     `toml:"title" json:"title"`
	NavTitle         string     `toml:"nav_title" json:"nav_title"`
	NavButtonName    string     `toml:"nav_button_name" json:"nav_button_name"`
	NavButtonLink    string     `toml:"nav_button_link" json:"nav_button_link"`
	HeaderMessage    markdown   `toml:"header_message" json:"-"`
	FooterMessage    markdown   `toml:"footer_message" json:"-"`
	Categories       []Category `toml:"category" json:"categories"`
	WorkerInterval   duration   `toml:"worker_interval" json:"-"`
	WorkerTimeout    duration   `toml:"worker_timeout" json:"-"`
	NumOfWorker      int        `toml:"num_of_worker" json:"-"`
	MaxCheckAttempts int        `toml:"max_check_attempts" json:"-"`
	RetryInterval    duration   `toml:"retry_interval" json:"-"`
	LatestTimeRange  duration   `toml:"latest_time_range" json:"-"`
	Days             []string   `json:"days"`
}

type Category struct {
	Name     string     `toml:"name" json:"name"`
	Comment  string     `toml:"comment" json:"comment"`
	Services []*Service `toml:"service" json:"services"`
}

type Service struct {
	categoryName   string
	Name           string        `toml:"name" json:"name"`
	Command        []string      `toml:"command" json:"command"`
	LatestStatus   *statusText   `json:"latest_status"`
	LatestStatusAt time.Time     `json:"latest_status_at"`
	Statuses       []*statusText `json:"statuses"`
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
	if conf.WorkerInterval.Duration == 0 {
		// 5min
		d, _ := time.ParseDuration("5m")
		conf.WorkerInterval.Duration = d
	}
	if conf.WorkerTimeout.Duration == 0 {
		// 30sec
		d, _ := time.ParseDuration("30s")
		conf.WorkerTimeout.Duration = d
	}
	if conf.LatestTimeRange.Duration == 0 {
		// 30sec
		d, _ := time.ParseDuration("1h")
		conf.LatestTimeRange.Duration = d
	}

	if conf.MaxCheckAttempts == 0 {
		conf.MaxCheckAttempts = 3
	}
	if conf.RetryInterval.Duration == 0 {
		// 10sec
		d, _ := time.ParseDuration("5s")
		conf.RetryInterval.Duration = d
	}

	return &conf, nil
}

func joinName(category, name string) string {
	return fmt.Sprintf("%s&&&&%s", category, name)
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

	// initialize
	opt.config = conf

	opt.workerChannel = make(chan Service)

	// run
	ctxWorker := context.Background()
	ctxWorker, cancelWorker := context.WithCancel(ctxWorker)

	defer cancelWorker()
	go opt.startWorker(ctxWorker)
	go opt.httpserver()

	chWorker := make(chan struct{})
	<-chWorker
	return 0
}

func main() {
	os.Exit(_main())
}
