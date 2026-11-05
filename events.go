package main

import (
	"AuOvIP/wcatools"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// DeviceChangedData is the payload of the "deviceChanged" event.
type DeviceChangedData struct {
	Event wcatools.DeviceEvent `json:"event"`
	ID    string               `json:"id"`
}

// init registers every custom event emitted by the application together with
// the exact Go type of its payload. The binding generator emits TypeScript
// typing for these events and validates payloads at emit time, so the types
// here must match what is passed to App.emitEvent.
//
// The generator only discovers registrations that are written out directly
// with constant arguments, so all of them live in this one init function.
func init() {
	application.RegisterEvent[[]Server]("serversUpdated")
	application.RegisterEvent[ConnStatusMessage]("updateConnStatus")
	application.RegisterEvent[ServerStats]("updateServerStats")
	application.RegisterEvent[bool]("updateServerStatus")
	application.RegisterEvent[string]("updateServerPagePortDisplay")
	application.RegisterEvent[DeviceChangedData]("deviceChanged")
}
