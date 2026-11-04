package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/Cn8001/svnch/internal/appinfo"
	"github.com/Cn8001/svnch/internal/service"
	"github.com/Cn8001/svnch/internal/svn"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/config.yml
var buildConfig []byte

const releaseRepo = "Cn8001/svnch"

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	binary, err := svn.EnsureInstalled(ctx)

	if err != nil {
		log.Fatalf("snv is not found: %s", err)
	}

	about := appinfo.FromConfig(buildConfig)
	updates, enableUpdates := service.NewUpdateService(ctx)

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "svnch",
		Description: "A Subversion client",
		Services: []application.Service{
			application.NewService(service.New(svn.NewExecRunner(binary))),
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

	// An updater that cannot start is not a reason to stop the editor.
	if err := initUpdater(app, about.Version); err != nil {
		slog.Error("updater", "error", err)
	} else {
		enableUpdates(app.Updater)
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
			go checkForUpdate(ctx, app)
		})
	}

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "svnch",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err = app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
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
