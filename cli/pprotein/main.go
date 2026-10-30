package main

import (
	"net/http"
	"os"

	"github.com/hmdyt/pprotein/integration/echov4"
	"github.com/hmdyt/pprotein/internal/ai"
	"github.com/hmdyt/pprotein/internal/collect"
	"github.com/hmdyt/pprotein/internal/collect/group"
	"github.com/hmdyt/pprotein/internal/event"
	"github.com/hmdyt/pprotein/internal/extproc/alp"
	"github.com/hmdyt/pprotein/internal/extproc/slp"
	"github.com/hmdyt/pprotein/internal/memo"
	"github.com/hmdyt/pprotein/internal/pprof"
	"github.com/hmdyt/pprotein/internal/storage"
	"github.com/hmdyt/pprotein/view"
	"github.com/labstack/echo/v4"
)

func start() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	store, err := storage.New("data")
	if err != nil {
		return err
	}

	e := echo.New()
	echov4.Integrate(e)

	fs, err := view.FS()
	if err != nil {
		return err
	}
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(fs))))

	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	})

	hub := event.NewHub()
	hub.RegisterHandlers(api.Group("/event"))

	pprofOpts := &collect.Options{
		Type:     "pprof",
		Ext:      "-pprof.pb.gz",
		Store:    store,
		EventHub: hub,
	}
	pprofHandler := pprof.NewHandler(pprofOpts)
	if err := pprofHandler.Register(api.Group("/pprof")); err != nil {
		return err
	}

	alpOpts := &collect.Options{
		Type:     "httplog",
		Ext:      "-httplog.log",
		Store:    store,
		EventHub: hub,
	}
	alpHandler, err := alp.NewHandler(alpOpts, store)
	if err != nil {
		return err
	}
	if err := alpHandler.Register(api.Group("/httplog")); err != nil {
		return err
	}

	slpOpts := &collect.Options{
		Type:     "slowlog",
		Ext:      "-slowlog.log",
		Store:    store,
		EventHub: hub,
	}
	slpHandler, err := slp.NewHandler(slpOpts, store)
	if err != nil {
		return err
	}
	if err := slpHandler.Register(api.Group("/slowlog")); err != nil {
		return err
	}

	memoOpts := &collect.Options{
		Type:     "memo",
		Ext:      "-memo.log",
		Store:    store,
		EventHub: hub,
	}
	memoHandler := memo.NewHandler(memoOpts)
	if err := memoHandler.Register(api.Group("/memo")); err != nil {
		return err
	}

	ai.Register(e.Group("/ai"), ai.Sources{
		PProf:   pprofHandler.Collector(),
		HTTPLog: alpHandler.Collector(),
		SlowLog: slpHandler.Collector(),
		Memo:    memoHandler.Collector(),
	})

	grp, err := group.NewCollector(store, port)
	if err != nil {
		return err
	}
	grp.RegisterHandlers(api.Group("/group"))

	return e.Start(":" + port)
}

func main() {
	if err := start(); err != nil {
		panic(err)
	}
}
