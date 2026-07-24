// Package ui contains all Fyne GUI code for SearchBoar. It is the only
// package in this module that imports fyne.io/fyne/v2; everything it wires
// together (search engine, config, favorites, network search) is UI-agnostic
// and independently testable.
package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/cassiusamicus/SearchBoar/assets"
	"github.com/cassiusamicus/SearchBoar/internal/cache"
	"github.com/cassiusamicus/SearchBoar/internal/config"
	"github.com/cassiusamicus/SearchBoar/internal/favorites"
	"github.com/cassiusamicus/SearchBoar/internal/netsearch"
	"github.com/cassiusamicus/SearchBoar/internal/search"
	"github.com/cassiusamicus/SearchBoar/internal/storedsearches"
	"github.com/cassiusamicus/SearchBoar/internal/workspaces"
)

// Run builds and shows the main SearchBoar window, then blocks until the
// window is closed.
func Run() {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		// A corrupt config shouldn't prevent the app from starting; fall
		// back to defaults rather than crashing on launch.
		cfg = config.New(config.DefaultPath())
	}

	searchEng := search.NewEngine()
	// A cache that fails to open (e.g. a read-only home directory) just
	// means repeat searches stay as fast/slow as they've always been --
	// searchEng.Cache is nil-safe, so this never blocks startup.
	resultCache, err := cache.Open(cache.DefaultPath())
	if err == nil {
		searchEng.Cache = resultCache
	}

	appTheme := newNordTheme(cfg.AccentColor, cfg.ThemeMode != "light")

	a := &App{
		fyneApp:   app.NewWithID("com.epicureanfriends.searchboar"),
		cfg:       cfg,
		searchEng: searchEng,
		netEng:    netsearch.NewEngine(),
		favStore:  favorites.LoadStore(cfg),
		ssStore:   storedsearches.LoadStore(cfg),
		wsStore:   workspaces.LoadStore(cfg),
		cache:     resultCache,
		theme:     appTheme,
	}
	a.fyneApp.SetIcon(assets.Icon())
	a.fyneApp.Settings().SetTheme(appTheme)

	a.win = a.fyneApp.NewWindow("SearchBoar")
	a.win.SetIcon(assets.Icon())
	a.win.SetMaster()

	a.buildMainWindow()
	a.restoreWindowGeometry()
	a.restoreLastSearch()
	go a.checkDependenciesOnStartup()

	a.win.SetCloseIntercept(func() {
		if a.cancelSearch != nil {
			a.cancelSearch()
		}
		a.netEng.Mounts.UnmountAll(context.Background())
		a.saveWindowGeometry()
		a.cfg.Save()
		a.cache.Close()
		a.win.Close()
	})

	a.win.ShowAndRun()
}

func defaultWindowSize() fyne.Size {
	return fyne.NewSize(900, 600)
}
