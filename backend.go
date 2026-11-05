package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"AuOvIP/pcmresample"
	"AuOvIP/wcatools"

	"github.com/gorilla/websocket"
	wRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var aconf wcatools.AudioConfig

// Server represents a remote audio server to connect to.
type Server struct {
	ID              string
	Addr            string // IP:Port
	Hostname        string
	Online          bool
	Status          string
	ResampleOpts    pcmresample.Options
	PlaybackDevices []PlaybackDevice
	AudioConfig     wcatools.AudioConfig

	RemoteDeviceID string
	RemoteDevices  []wcatools.AudioConfig

	settingsCh chan settingsRequest

	cancel context.CancelFunc

	ctx context.Context

	infoCancel context.CancelFunc

	// Runtime state. Guarded by the package-level serversMu, not by a
	// per-Server mutex: ListServers hands out Server values by copy, so any
	// lock stored here would be copied along with them and protect nothing.
	players     map[string]*DevicePlayer // Map Key: DeviceID (or "default")
	distributor *AudioDistributor
}

type settingsRequest struct {
	opts pcmresample.Options
	done chan error
}

var (
	servers     = make(map[string]*Server)
	serverOrder []string
	serversMu   sync.Mutex
)

func ReorderServers(newOrder []string) {
	serversMu.Lock()
	defer serversMu.Unlock()

	if len(newOrder) != len(servers) {
		return
	}

	seen := make(map[string]bool)
	for _, id := range newOrder {
		if _, ok := servers[id]; !ok {
			return
		}
		seen[id] = true
	}

	if len(seen) != len(servers) {
		return
	}

	serverOrder = newOrder

	go func() { _ = SaveServerListConfig() }()
}

func audioStartup(s *Server) {
	connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.Connecting}

	serversMu.Lock()
	remoteDeviceID := s.RemoteDeviceID
	if remoteDeviceID == "" {
		remoteDeviceID = "default"
	}

	// Check availability before trying to connect
	if remoteDeviceID != "default" && len(s.RemoteDevices) > 0 {
		found := false
		for _, d := range s.RemoteDevices {
			if d.ID == remoteDeviceID {
				found = true
				break
			}
		}
		if !found {
			serversMu.Unlock()
			connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.CaptureDeviceUnavailable}
			return
		}
	}
	serversMu.Unlock()

	clientLogger.Infof("retrieving audio config from %s for device %s", s.Addr, remoteDeviceID)
	conf, err := getAudioConfig(s.Addr, remoteDeviceID)
	if err != nil {
		connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.AudioConfigRetrievalFailed}
		clientLogger.Errorf("getAudioConfig failed for %s: %v", s.Addr, err)
		return
	}

	// Store in server
	serversMu.Lock()
	s.AudioConfig = conf
	serversMu.Unlock()

	// log the retrieved config at debug level (full JSON)
	if cfgBytes, jerr := json.MarshalIndent(conf, "", "    "); jerr == nil {
		clientLogger.Debugf("retrieved audio config from %s: %s", s.Addr, string(cfgBytes))
	}

	// create a cancellable context for this run
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cleanup on exit

	// store cancel so DisconnectServer can cancel
	serversMu.Lock()
	s.cancel = cancel
	s.ctx = ctx
	s.distributor = NewAudioDistributor()
	// Copy devices to avoid holding lock during init
	devices := make([]PlaybackDevice, len(s.PlaybackDevices))
	copy(devices, s.PlaybackDevices)
	serversMu.Unlock()

	// PREPARE PLAYERS (Parallel)
	// Initialize players before connecting to ensure no latency on start
	srcSettings := pcmresample.Settings{
		SampleRate: int(conf.SamplesPerSec),
		Channels:   int(conf.Channels),
		Format:     pcmresample.PCM32Float,
	}

	var prepared []*DevicePlayer
	var preparedMu sync.Mutex
	var wg sync.WaitGroup

	for _, devCfg := range devices {
		if !devCfg.Enabled {
			continue
		}
		wg.Add(1)
		go func(cfg PlaybackDevice) {
			defer wg.Done()
			p, err := NewDevicePlayer(ctx, cfg, s, srcSettings)
			if err == nil {
				preparedMu.Lock()
				prepared = append(prepared, p)
				preparedMu.Unlock()
			} else {
				clientLogger.Errorf("Failed to init player for %s: %v", cfg.Name, err)
			}
		}(devCfg)
	}
	wg.Wait()

	// Check if context was cancelled during init (user clicked disconnect)
	if ctx.Err() != nil {
		return
	}

	// Connect Websocket
	u := url.URL{Scheme: "ws", Host: s.Addr, Path: "/audio"}
	q := u.Query()
	q.Set("deviceID", remoteDeviceID)
	u.RawQuery = q.Encode()

	var wsConn *websocket.Conn
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wsConn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		clientLogger.Errorf("websocket dial failed for %s: %v", s.Addr, err)
		connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.ConnectionFailed}
		return
	}

	// Close websocket on ctx cancellation to unblock reader goroutine
	go func() {
		<-ctx.Done()
		_ = wsConn.Close()
	}()

	// Start Players
	serversMu.Lock()
	for _, p := range prepared {
		ch := s.distributor.Subscribe(p)
		s.players[p.config.DeviceID] = p

		go func(dp *DevicePlayer, c <-chan []byte) {
			if err := dp.Run(c); err != nil {
				clientLogger.Errorf("Player %s stopped: %v", dp.config.Name, err)
			}
			s.distributor.Unsubscribe(dp)
		}(p, ch)
	}
	serversMu.Unlock()

	connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.Connected}

	readErrCh := make(chan error, 1)

	go func() {
		defer wsConn.Close()

		wsConn.SetPingHandler(func(appData string) error {
			_ = wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			return wsConn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})

		for {
			_ = wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, m, err := wsConn.ReadMessage()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				readErrCh <- err
				return
			}

			// Broadcast audio data to all players
			s.distributor.Broadcast(m)
		}
	}()

	select {
	case <-ctx.Done():
		//Seems unnecessary for now, keeping just in case
		//connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.Idle}
	case err := <-readErrCh:
		clientLogger.Errorf("Websocket read error: %v", err)
		connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.ConnectionLost}
	case req := <-s.settingsCh:
		go func(req settingsRequest) {
			serversMu.Lock()
			// Update stored options so new players get them
			s.ResampleOpts = req.opts

			// Collect done channels
			var chans []<-chan error
			for _, p := range s.players {
				chans = append(chans, p.UpdateResamplerAsync(req.opts))
			}
			serversMu.Unlock()

			// Wait for all
			var finalErr error
			for _, c := range chans {
				if err := <-c; err != nil {
					finalErr = err
				}
			}

			select {
			case req.done <- finalErr:
			default:
			}
		}(req)
		// Continue loop
		goto loop
	}
	return

loop:
	for {
		select {
		case <-ctx.Done():
			connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.Idle}
			return
		case err := <-readErrCh:
			clientLogger.Errorf("Websocket read error: %v", err)
			connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.ConnectionLost}
			return
		case req := <-s.settingsCh:
			go func(req settingsRequest) {
				serversMu.Lock()
				s.ResampleOpts = req.opts
				var chans []<-chan error
				for _, p := range s.players {
					chans = append(chans, p.UpdateResamplerAsync(req.opts))
				}
				serversMu.Unlock()

				var finalErr error
				for _, c := range chans {
					if err := <-c; err != nil {
						finalErr = err
					}
				}
				select {
				case req.done <- finalErr:
				default:
				}
			}(req)
		}
	}
}

func getAudioConfig(addr string, deviceID string) (wcatools.AudioConfig, error) {
	// Use HTTP GET to the control endpoint to fetch audio config.
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get("http://" + addr + "/getAudioConfig?deviceID=" + deviceID)
	if err != nil {
		return wcatools.AudioConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wcatools.AudioConfig{}, fmt.Errorf("getAudioConfig returned status %d", resp.StatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return wcatools.AudioConfig{}, err
	}
	var aconf wcatools.AudioConfig
	if err := json.Unmarshal(respBytes, &aconf); err != nil {
		return wcatools.AudioConfig{}, err
	}
	return aconf, nil
}

func AddServer(ip, port string) (*Server, error) {
	serversMu.Lock()
	defer serversMu.Unlock()
	//Ensures that a server can only be added once
	id := fmt.Sprintf("%s:%s", ip, port)
	if _, exists := servers[id]; exists {
		return servers[id], nil
	}

	// Initial connection check
	serverInfo, conn, err := checkConnection(id)
	if err != nil {
		return nil, err
	}

	infoCtx, infoCancel := context.WithCancel(context.Background())

	// Default to system default device if new
	defID, _ := wcatools.GetDefaultDeviceID()
	defName := "Default Output"
	if defInfo, err := wcatools.GetDefaultDeviceInfo(); err == nil {
		defName = defInfo.Name
	}

	initialDevices := []PlaybackDevice{
		{
			DeviceID:  defID,
			Name:      defName,
			Volume:    1.0,
			Enabled:   true,
			IsDefault: true,
		},
	}

	s := &Server{
		ID:              id,
		Addr:            id,
		Hostname:        serverInfo.Hostname,
		RemoteDevices:   serverInfo.AudioDevices,
		Online:          true,
		Status:          connStatuses.Idle,
		ResampleOpts:    clientConfig.DefaultResampleOpts,
		PlaybackDevices: initialDevices,
		settingsCh:      make(chan settingsRequest, 1),
		cancel:          nil,
		infoCancel:      infoCancel,
		players:         make(map[string]*DevicePlayer),
	}
	servers[id] = s
	serverOrder = append(serverOrder, id)

	go maintainInfoConnection(infoCtx, s, conn)
	go func() { _ = SaveServerListConfig() }()

	return s, nil
}

func checkConnection(addr string) (InfoData, *websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/info"}
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return InfoData{}, nil, err
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var info InfoData
	if err := conn.ReadJSON(&info); err != nil {
		conn.Close()
		return InfoData{}, nil, err
	}

	return info, conn, nil
}

func maintainInfoConnection(ctx context.Context, s *Server, conn *websocket.Conn) {
	defer func() {
		conn.Close()
		serversMu.Lock()
		if curr, ok := servers[s.ID]; ok && curr == s {
			s.Online = false
		}
		serversMu.Unlock()
		wRuntime.EventsEmit(app.ctx, "serversUpdated", ListServers())
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		return nil
	})

	// Start reader loop for Info updates and Pong processing
	go func() {
		defer close(done)
		for {
			var info InfoData
			if err := conn.ReadJSON(&info); err != nil {
				return
			}
			serversMu.Lock()
			s.Hostname = info.Hostname
			s.RemoteDevices = info.AudioDevices

			// Check if currently selected remote device is still available
			// Only if we are currently streaming (s.cancel != nil)
			if s.cancel != nil {
				targetID := s.RemoteDeviceID
				if targetID == "" {
					targetID = "default"
				}

				found := false
				if targetID == "default" {
					found = true // Default is always considered available as it falls back
				} else {
					for _, d := range s.RemoteDevices {
						if d.ID == targetID {
							found = true
							break
						}
					}
				}

				if !found {
					// Device lost!
					go func(serverID string) {
						// Disconnect and update status
						DisconnectServer(serverID)
						connStatus <- ConnStatusMessage{ServerID: serverID, StatusText: connStatuses.CaptureDeviceUnavailable}
					}(s.ID)
				}
			}
			serversMu.Unlock()
			wRuntime.EventsEmit(app.ctx, "serversUpdated", ListServers())
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func UpdateServerSettings(id string, opts pcmresample.Options) error {
	serversMu.Lock()
	s, ok := servers[id]
	if !ok {
		serversMu.Unlock()
		return fmt.Errorf("server not found")
	}
	s.ResampleOpts = opts

	// Create done channel
	done := make(chan error, 1)
	req := settingsRequest{opts: opts, done: done}

	serverStatus := s.Status
	settingsCh := s.settingsCh
	serversMu.Unlock()

	go func() { _ = SaveServerListConfig() }()

	if serverStatus != connStatuses.Connected {
		return nil
	}

	// Try to send
	select {
	case settingsCh <- req:
	default:
		// Channel full, replace existing
		select {
		case oldReq := <-settingsCh:
			// notify old request that it was cancelled
			select {
			case oldReq.done <- fmt.Errorf("cancelled by newer request"):
			default:
			}
		default:
		}
		// Try send again
		select {
		case settingsCh <- req:
		default:
			return fmt.Errorf("failed to queue settings update (server busy)")
		}
	}

	// Wait for completion
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		return fmt.Errorf("timeout waiting for settings application")
	}
}

func RemoveServer(id string) {
	serversMu.Lock()
	defer serversMu.Unlock()
	if s, ok := servers[id]; ok {
		// ensure disconnect
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
		}
		if s.infoCancel != nil {
			s.infoCancel()
			s.infoCancel = nil
		}
		delete(servers, id)

		for i, oid := range serverOrder {
			if oid == id {
				serverOrder = append(serverOrder[:i], serverOrder[i+1:]...)
				break
			}
		}

		go func() { _ = SaveServerListConfig() }()
	}
}

func ListServers() []Server {
	serversMu.Lock()
	defer serversMu.Unlock()
	out := make([]Server, 0, len(servers))
	for _, id := range serverOrder {
		if s, ok := servers[id]; ok {
			out = append(out, *s)
		}
	}
	if len(out) != len(servers) {
		serverOrder = make([]string, 0, len(servers))
		out = make([]Server, 0, len(servers))
		for id, s := range servers {
			serverOrder = append(serverOrder, id)
			out = append(out, *s)
		}
	}
	return out
}

func ReconnectServer(id string) error {
	serversMu.Lock()
	s, ok := servers[id]
	serversMu.Unlock()

	if !ok {
		return fmt.Errorf("server not found")
	}

	serverInfo, conn, err := checkConnection(id)
	if err != nil {
		return err
	}

	infoCtx, infoCancel := context.WithCancel(context.Background())

	serversMu.Lock()
	if s.infoCancel != nil {
		s.infoCancel()
	}
	s.Hostname = serverInfo.Hostname
	s.RemoteDevices = serverInfo.AudioDevices
	s.Online = true
	s.infoCancel = infoCancel
	serversMu.Unlock()

	go maintainInfoConnection(infoCtx, s, conn)
	go func() { _ = SaveServerListConfig() }()
	wRuntime.EventsEmit(app.ctx, "serversUpdated", ListServers())

	return nil
}

func ConnectServer(id string) error {
	serversMu.Lock()
	s, ok := servers[id]
	if !ok {
		serversMu.Unlock()
		return fmt.Errorf("server not found")
	}
	//s.cancel will be set in audioStartup when connection starts
	serversMu.Unlock()
	go audioStartup(s)
	return nil
}

func DisconnectServer(id string) error {
	serversMu.Lock()
	defer serversMu.Unlock()
	s, ok := servers[id]
	if !ok {
		return fmt.Errorf("server not found")
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	connStatus <- ConnStatusMessage{ServerID: s.ID, StatusText: connStatuses.Idle}
	return nil
}

func InitDeviceMonitor() {
	wcatools.StartMonitor(func(event wcatools.DeviceEvent, id string) {
		if app != nil && app.ctx != nil {
			wRuntime.EventsEmit(app.ctx, "deviceChanged", map[string]interface{}{
				"event": event,
				"id":    id,
			})
		}
	})
}

func AddPlaybackDevice(serverID, deviceID string) error {
	serversMu.Lock()
	s, ok := servers[serverID]
	serversMu.Unlock()
	if !ok {
		return fmt.Errorf("server not found")
	}

	serversMu.Lock()
	defer serversMu.Unlock()

	// Check if already exists
	for _, d := range s.PlaybackDevices {
		if d.DeviceID == deviceID {
			return nil
		}
	}

	// Get Device Name
	name := "Unknown Device"
	if info, err := wcatools.GetDeviceByID(deviceID); err == nil {
		name = info.Name
	}

	newDev := PlaybackDevice{
		DeviceID:  deviceID,
		Name:      name,
		Volume:    1.0,
		Enabled:   true,
		IsDefault: false,
	}
	s.PlaybackDevices = append(s.PlaybackDevices, newDev)

	// If connected, start player
	if s.Status == connStatuses.Connected && s.distributor != nil && s.ctx != nil {
		srcSettings := pcmresample.Settings{
			SampleRate: int(s.AudioConfig.SamplesPerSec),
			Channels:   int(s.AudioConfig.Channels),
			Format:     pcmresample.PCM32Float,
		}

		player, err := NewDevicePlayer(s.ctx, newDev, s, srcSettings)
		if err != nil {
			clientLogger.Errorf("Failed to start player for new device: %v", err)
		} else {
			ch := s.distributor.Subscribe(player)
			s.players[deviceID] = player

			go func(p *DevicePlayer, c <-chan []byte) {
				if err := p.Run(c); err != nil {
					clientLogger.Errorf("Player %s stopped: %v", p.config.Name, err)
				}
				s.distributor.Unsubscribe(p)
			}(player, ch)
		}
	}
	go func() { _ = SaveServerListConfig() }()
	return nil
}

func RemovePlaybackDevice(serverID, deviceID string) error {
	serversMu.Lock()
	s, ok := servers[serverID]
	serversMu.Unlock()
	if !ok {
		return fmt.Errorf("server not found")
	}

	serversMu.Lock()
	defer serversMu.Unlock()

	// Remove from config
	found := -1
	for i, d := range s.PlaybackDevices {
		if d.DeviceID == deviceID {
			found = i
			break
		}
	}
	if found == -1 {
		return nil
	}

	// If playing, stop player
	if p, ok := s.players[deviceID]; ok {
		p.cancel()                   // Stop the loop
		s.distributor.Unsubscribe(p) // Ensure unsubscribe
		delete(s.players, deviceID)
	}

	s.PlaybackDevices = append(s.PlaybackDevices[:found], s.PlaybackDevices[found+1:]...)
	go func() { _ = SaveServerListConfig() }()
	return nil
}

func UpdatePlaybackDevice(serverID, deviceID string, enabled bool, volume float32) error {
	serversMu.Lock()
	s, ok := servers[serverID]
	serversMu.Unlock()
	if !ok {
		return fmt.Errorf("server not found")
	}

	serversMu.Lock()
	defer serversMu.Unlock()

	// Update Config
	var targetDev *PlaybackDevice
	for i := range s.PlaybackDevices {
		if s.PlaybackDevices[i].DeviceID == deviceID {
			s.PlaybackDevices[i].Enabled = enabled
			s.PlaybackDevices[i].Volume = volume
			targetDev = &s.PlaybackDevices[i]
			break
		}
	}
	if targetDev == nil {
		return fmt.Errorf("device not found in server config")
	}

	// Update Runtime
	if p, ok := s.players[deviceID]; ok {
		// Update Volume
		p.SetVolume(volume)

		// Handle Enabled/Disabled
		if !enabled {
			p.cancel()
			s.distributor.Unsubscribe(p)
			delete(s.players, deviceID)
		}
	} else if enabled && s.Status == connStatuses.Connected && s.ctx != nil {
		// Enable: Start Player
		srcSettings := pcmresample.Settings{
			SampleRate: int(s.AudioConfig.SamplesPerSec),
			Channels:   int(s.AudioConfig.Channels),
			Format:     pcmresample.PCM32Float,
		}

		player, err := NewDevicePlayer(s.ctx, *targetDev, s, srcSettings)
		if err != nil {
			clientLogger.Errorf("Failed to start player for device %s: %v", targetDev.Name, err)
		} else {
			ch := s.distributor.Subscribe(player)
			s.players[deviceID] = player

			go func(p *DevicePlayer, c <-chan []byte) {
				if err := p.Run(c); err != nil {
					clientLogger.Errorf("Player %s stopped: %v", p.config.Name, err)
				}
				s.distributor.Unsubscribe(p)
			}(player, ch)
		}
	}
	go func() { _ = SaveServerListConfig() }()
	return nil
}

func SetServerRemoteDevice(id string, deviceID string) error {
	serversMu.Lock()
	s, ok := servers[id]
	if !ok {
		serversMu.Unlock()
		return fmt.Errorf("server not found")
	}
	s.RemoteDeviceID = deviceID
	serversMu.Unlock()

	go func() { _ = SaveServerListConfig() }()

	// If connected (audio streaming), restart connection to switch device
	serversMu.Lock()
	isPlaying := s.cancel != nil
	serversMu.Unlock()

	if isPlaying {
		if err := DisconnectServer(id); err != nil {
			clientLogger.Errorf("Failed to disconnect for device switch: %v", err)
		}
		// Small delay to ensure cleanup. Just a safety, might not be needed
		time.Sleep(100 * time.Millisecond)
		if err := ConnectServer(id); err != nil {
			clientLogger.Errorf("Failed to reconnect for device switch: %v", err)
			return err
		}
	}

	return nil
}
