package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Cn8001/svnch/internal/appinfo"
	"github.com/Cn8001/svnch/internal/explorer"
	"github.com/Cn8001/svnch/internal/servers"
	"github.com/Cn8001/svnch/internal/service"
	"github.com/Cn8001/svnch/internal/svn"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/config.yml
var buildConfig []byte

//go:embed THIRD_PARTY_NOTICES.md
var notices string

//go:embed THIRD_PARTY_LICENSES.json
var licenseTexts []byte

const releaseRepo = "Cn8001/svnch"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A missing svn is shown in the window with install help rather than stopping the app; until then commands run
	// the bare name, so an svn installed by hand later is found on PATH without a restart.
	binary, installErr := svn.EnsureInstalled(ctx)
	if installErr != nil {
		slog.Warn("svn is not available", "error", installErr)
	}
	runBinary := binary
	if runBinary == "" {
		runBinary = "svn"
	}

	about := appinfo.FromConfig(buildConfig)
	about.Notices = notices
	if err := json.Unmarshal(licenseTexts, &about.Licenses); err != nil {
		slog.Error("licences", "error", err)
	}
	updates, enableUpdates := service.NewUpdateService(ctx)
	runner := svn.NewExecRunner(runBinary)
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
		slog.Warn("no user config folder, saved servers last until reboot", "error", err)
	}
	store := servers.NewStore(filepath.Join(configDir, "svnch", "servers.json"))

	exe, err := os.Executable()
	if err != nil {
		slog.Warn("own path unknown, explorer menu uses the bare name", "error", err)
		exe = "svnch.exe"
	}
	menu := explorer.New(exe, filepath.Join(configDir, "svnch", "icons"))
	if err := menu.Repair(ctx); err != nil {
		slog.Warn("explorer menu", "error", err)
	}
	svnConfig, err := svn.ConfigDir()
	if err != nil {
		slog.Warn("svn config folder unknown, certificates cannot be trusted from svnch", "error", err)
	}
	web := &http.Client{Timeout: 30 * time.Second, Transport: svn.TrustingTransport(svnConfig)}

	// Wails 3.0.0 only applies a service's own MarshalError to its bound methods and falls back to its default, which
	// sends Go errors as {}, not to the application option; so each service carries it, or the webview never sees
	// the AUTH and CERT codes it branches on.
	failures := application.ServiceOptions{MarshalError: service.MarshalError}
	app := application.New(application.Options{
		Name:         "svnch",
		Description:  "A Subversion client",
		MarshalError: service.MarshalError,
		Services: []application.Service{
			application.NewServiceWithOptions(service.New(runner, runner, web), failures),
			application.NewServiceWithOptions(service.NewServerService(store), failures),
			application.NewServiceWithOptions(service.NewTrustService(svnConfig), failures),
			application.NewServiceWithOptions(service.NewSetupService(binary, installErr), failures),
			application.NewServiceWithOptions(service.NewAppService(about, explorer.ParseLaunch(os.Args[1:], os.Stat)), failures),
			application.NewServiceWithOptions(service.NewExplorerService(menu), failures),
			application.NewServiceWithOptions(updates, failures),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	runner.OnProgress(func(p svn.Progress) { app.Event.Emit("svn:progress", p) })

	// An updater that cannot start is not a reason to stop the app.
	if err := initUpdater(app, about.Version); err != nil {
		slog.Error("updater", "error", err)
	} else {
		enableUpdates(app.Updater)
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go checkForUpdate(ctx, app)
		})
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "svnch Subversion",
		Width:     1200,
		Height:    760,
		MinWidth:  860, // file list and diff still fit side by side
		MinHeight: 540,
		// The dark theme's background, so the window does not flash a different colour before the frontend paints.
		BackgroundColour: application.NewRGB(22, 24, 28),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func initUpdater(app *application.App, version string) error {
	gh, err := github.New(github.Config{Repository: releaseRepo, ChecksumAsset: "SHA256SUMS"})
	if err != nil {
		return fmt.Errorf("release source: %w", err)
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: version,
		Providers:      []updater.Provider{gh},
	}); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	return nil
}

func checkForUpdate(ctx context.Context, app *application.App) {
	rel, err := app.Updater.Check(ctx)
	if err != nil {
		slog.Warn("update check", "error", err)
		return
	}
	if rel == nil {
		return
	}
	if err := app.Updater.CheckAndInstall(ctx); err != nil {
		slog.Error("update", "error", err)
	}
}
