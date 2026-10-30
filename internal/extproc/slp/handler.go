package slp

import (
	_ "embed"
	"fmt"

	"github.com/hmdyt/pprotein/internal/collect"
	"github.com/hmdyt/pprotein/internal/extproc"
	"github.com/hmdyt/pprotein/internal/persistent"
	"github.com/hmdyt/pprotein/internal/storage"
	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
)

type (
	handler struct {
		opts      *collect.Options
		config    *persistent.Handler
		collector *collect.Collector
	}
)

//go:embed slp.yml
var defaultConfig []byte

func NewHandler(opts *collect.Options, store storage.Storage) (*handler, error) {
	h := &handler{
		opts: opts,
	}

	config, err := persistent.New(store, "slp.yml", defaultConfig, h.sanitize)
	if err != nil {
		return nil, fmt.Errorf("failed to create targets: %w", err)
	}
	h.config = config

	return h, nil
}

func (h *handler) Register(g *echo.Group) error {
	h.config.RegisterHandlers(g.Group("/config"))

	inner := extproc.NewHandler(&processor{confPath: h.config.GetPath()}, h.opts)
	if err := inner.Register(g); err != nil {
		return fmt.Errorf("failed to register extproc handlers: %w", err)
	}
	h.collector = inner.Collector()
	return nil
}

// Collector returns the collector initialized by Register.
func (h *handler) Collector() *collect.Collector {
	return h.collector
}

func (h *handler) sanitize(raw []byte) ([]byte, error) {
	var config interface{}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}

	res, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	return res, nil
}
