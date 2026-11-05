package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// Create an instance of the app structure
var app = NewApp()

var serverLogger *Logger
var clientLogger *Logger

func main() {
	var err error

	// Initialize loggers (1MB size limit)
	serverLogger, err = InitLogger("logs/server.log", LevelDebug, 1024*1024)
	if err != nil {
		log.Println("Could not init serverLogger:", err)
	}
	clientLogger, err = InitLogger("logs/client.log", LevelDebug, 1024*1024)
	if err != nil {
		log.Println("Could not init clientLogger:", err)
	}

	// Create application with options. Name is the internal application identifier (menu
	// labels, About dialog, autostart slug); the user-visible name is the window Title below.
	wailsApp := application.New(application.Options{
		Name:        "AudioOverIP",
		Description: "Stream audio over the network",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.setWails(wailsApp)

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Audio Over IP",
		Width:            1280,
		Height:           720,
		MinWidth:         384,
		MinHeight:        216,
		Frameless:        true,
		BackgroundColour: application.NewRGBA(27, 38, 54, 1),
		URL:              "/",
	})

	app.setMainWindow(mainWindow)

	// Start the frontend updaters once the frontend runtime is ready (replaces v2 OnDomReady)
	mainWindow.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		app.domReady()
	})

	err = wailsApp.Run()

	if err != nil {
		clientLogger.Errorf("wails run failed: %v", err)
		os.Exit(1)
	}
}
