package main

import (
	"context"
	"os/exec"
	"sync"

	"AuOvIP/pcmresample"
	"AuOvIP/wcatools"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// appVersion is shown in Settings > General. Keep in sync with the version in
// build/config.yml (it also renders into build/windows/info.json and the manifest).
const appVersion = "0.9.0.1"

// App struct
type App struct {
	wails  *application.App
	window application.Window

	// ctx is the application context handed to ServiceStartup. It stays valid
	// while the application is running and is cancelled right before shutdown.
	ctx context.Context

	// updatersOnce makes sure the frontend updater goroutines are started
	// only once, no matter how many times the frontend signals it is ready.
	updatersOnce sync.Once
}

// setWails stores the application instance (call after application.New)
func (a *App) setWails(wailsApp *application.App) {
	a.wails = wailsApp
}

// setMainWindow stores the main window (call after creating the window)
func (a *App) setMainWindow(window application.Window) {
	a.window = window
}

// emitEvent sends an event to the frontend.
// It is a no-op if the application is not initialized yet.
func (a *App) emitEvent(name string, data ...any) {
	if a != nil && a.wails != nil {
		a.wails.Event.Emit(name, data...)
	}
}

// GetAppVersion returns the application version for the settings UI.
func (a *App) GetAppVersion() string {
	return appVersion
}

func (a *App) GetServerConfig() ServerConfig {
	return serverConfig
}

func (a *App) SaveServerConfig(cfg ServerConfig) error {
	configMu.Lock()
	serverConfig = cfg
	configMu.Unlock()
	a.emitEvent("updateServerPagePortDisplay", cfg.Port)
	return SaveServerConfig()
}

func (a *App) GetClientConfig() ClientConfig {
	return clientConfig
}

func (a *App) SaveClientConfig(cfg ClientConfig) error {
	configMu.Lock()
	clientConfig = cfg
	configMu.Unlock()
	return SaveClientConfig()
}

func (a *App) GetConfigDir() string {
	dir, _ := GetConfigDir()
	return dir
}

func (a *App) OpenConfigDir() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	return exec.Command("explorer", dir).Start()
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// ServiceStartup is called at application startup (Wails v3 service initialisation hook).
// The context it receives is valid for as long as the application is running and
// is cancelled right before shutdown.
func (a *App) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	a.ctx = ctx

	_ = LoadConfig()

	InitDeviceMonitor()

	for _, s := range ListServers() {
		go func(id string) {
			_ = ReconnectServer(id)
		}(s.ID)
	}

	return nil
}

// domReady is called after front-end resources have been loaded.
// WindowRuntimeReady fires on every page load or reload (a Vite full reload or a
// manual reload in dev mode fires it again), so the goroutines are started behind
// a sync.Once. Without the guard every reload would add another pair of
// goroutines consuming the same channels, making the event flow non-deterministic.
func (a *App) domReady() {
	a.updatersOnce.Do(func() {
		//Start connection status frontend updater
		go connStatusUpdater(a.ctx)
		go serverStatsUpdater(a.ctx)
	})
}

func (a *App) QuitApp() {
	a.wails.Quit()
}

func (a *App) MinimiseApp() {
	a.window.Minimise()
}

// Server management exposed to frontend
func (a *App) AddServer(ipAddress string, port string) error {
	_, err := AddServer(ipAddress, port)
	if err == nil {
		a.emitEvent("serversUpdated", ListServers())
	}
	return err
}

func (a *App) CheckServerConnection(id string) error {
	return ReconnectServer(id)
}

func (a *App) RemoveServer(id string) {
	RemoveServer(id)
	a.emitEvent("serversUpdated", ListServers())
}

func (a *App) ReorderServers(newOrder []string) {
	ReorderServers(newOrder)
}

func (a *App) GetServerList() []Server {
	return ListServers()
}

func (a *App) UpdateServerSettings(id string, opts pcmresample.Options) {
	err := UpdateServerSettings(id, opts)
	if err != nil {
		clientLogger.Errorf("%s", err.Error())
	}
	a.emitEvent("serversUpdated", ListServers())
}

func (a *App) ConnectToAudioServer(id string) {
	// start connect in background (launches a go routine)
	err := ConnectServer(id)
	if err != nil {
		clientLogger.Errorf("%s", err.Error())
	}
}

func (a *App) DisconnetFromAudioServer(id string) {
	err := DisconnectServer(id)
	if err != nil {
		clientLogger.Errorf("%s", err.Error())
	}
}

var serverIsOn = false

func (a *App) GetServerEnabled() bool {
	return serverIsOn
}

func (a *App) EnableServer() {
	serverIsOn = true
	a.emitEvent("updateServerStatus", serverIsOn)
	startServer(serverConfig.Port)
}

func (a *App) DisableServer() {
	stopServer()
	serverIsOn = false
	a.emitEvent("updateServerStatus", serverIsOn)
}

func (a *App) ListAudioDevices() ([]wcatools.AudioConfig, error) {
	return wcatools.ListDevices()
}

func (a *App) AddPlaybackDevice(serverID, deviceID string) error {
	err := AddPlaybackDevice(serverID, deviceID)
	if err == nil {
		a.emitEvent("serversUpdated", ListServers())
	}
	return err
}

func (a *App) RemovePlaybackDevice(serverID, deviceID string) error {
	err := RemovePlaybackDevice(serverID, deviceID)
	if err == nil {
		a.emitEvent("serversUpdated", ListServers())
	}
	return err
}

func (a *App) UpdatePlaybackDevice(serverID, deviceID string, enabled bool, volume float32) error {
	err := UpdatePlaybackDevice(serverID, deviceID, enabled, volume)
	if err == nil {
		a.emitEvent("serversUpdated", ListServers())
	}
	return err
}

func (a *App) SetServerRemoteDevice(serverID, deviceID string) error {
	err := SetServerRemoteDevice(serverID, deviceID)
	if err == nil {
		a.emitEvent("serversUpdated", ListServers())
	}
	return err
}
