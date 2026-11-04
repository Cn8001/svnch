package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Cn8001/svnch/internal/appinfo"
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

	app := application.New(application.Options{
		Name:         "svnch",
		Description:  "A Subversion client",
		MarshalError: service.MarshalError,
		Services: []application.Service{
			application.NewService(service.New(runner, runner)),
			application.NewService(service.NewServerService(store)),
			application.NewService(service.NewSetupService(binary, installErr)),
			application.NewService(service.NewAppService(about)),
			application.NewService(updates),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

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
		Title:     "svnch",
		Width:     1200,
		Height:    760,
		MinWidth:  860, // file list and diff still fit side by side
		MinHeight: 540,
		// The page's own paper colour, so the window does not flash dark before the frontend paints.
		BackgroundColour: application.NewRGB(239, 230, 207),
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
