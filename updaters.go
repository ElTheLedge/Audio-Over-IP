package main

import "context"

// Per-server status update channel; carries server ID and status text as JSON
var connStatus = make(chan ConnStatusMessage, 100)

type ConnStatusMessage struct {
	ServerID   string `json:"serverId"`
	StatusText string `json:"statusText"`
}

type ConnStatuses struct {
	Idle                       string
	Connected                  string
	ConnectionFailed           string
	Connecting                 string
	ConnectionLost             string
	InvalidIpOrPort            string
	ResamplingFailed           string
	AudioConfigRetrievalFailed string
	CaptureDeviceUnavailable   string
}

var connStatuses = ConnStatuses{
	Idle:                       "Idle",
	Connected:                  "Connected",
	ConnectionFailed:           "Connection failed",
	Connecting:                 "Connecting...",
	ConnectionLost:             "Connection lost",
	InvalidIpOrPort:            "Invalid IP or Port",
	ResamplingFailed:           "Resampling failed",
	AudioConfigRetrievalFailed: "Could not retrieve server audio config",
	CaptureDeviceUnavailable:   "Capture Device Unavailable",
}

func connStatusUpdater(ctx context.Context) {
	for {
		select {
		case msg := <-connStatus:
			// Update server registry status under lock so snapshots reflect current state
			serversMu.Lock()
			if s, ok := servers[msg.ServerID]; ok {
				s.Status = msg.StatusText
			}
			serversMu.Unlock()

			// Emit the per-server status update
			app.emitEvent("updateConnStatus", msg)

			app.emitEvent("serversUpdated", ListServers())

		case <-ctx.Done():
			return
		}
	}
}

// ServerStats holds statistics about the server.
type ServerStats struct {
	ConnectedClients int `json:"connectedClients"`
	Bandwidth        int `json:"bandwidth"` // in kbps
}

// serverStats is a channel for ServerStats updates.
var serverStats = make(chan ServerStats, 10)

// serverStatsUpdater listens for server stats updates and emits them to the frontend.
// It returns when the application context is cancelled.
func serverStatsUpdater(ctx context.Context) {
	for {
		select {
		case stats := <-serverStats:
			app.emitEvent("updateServerStats", stats)

		case <-ctx.Done():
			return
		}
	}
}
