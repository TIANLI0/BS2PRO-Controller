// Package ipc provides inter-process communication between core service and GUI
package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/TIANLI0/BS2PRO-Controller/internal/types"
)

const (
	// PipeName named pipe name
	PipeName = "BS2PRO-Controller-IPC"
	// PipePath named pipe full path
	PipePath = `\\.\pipe\` + PipeName
)

// RequestType request type
type RequestType string

const (
	// Device related
	ReqConnect           RequestType = "Connect"
	ReqDisconnect        RequestType = "Disconnect"
	ReqGetDeviceStatus   RequestType = "GetDeviceStatus"
	ReqGetCurrentFanData RequestType = "GetCurrentFanData"

	// Config related
	ReqGetConfig                RequestType = "GetConfig"
	ReqUpdateConfig             RequestType = "UpdateConfig"
	ReqSetFanCurve              RequestType = "SetFanCurve"
	ReqGetFanCurve              RequestType = "GetFanCurve"
	ReqGetFanCurveProfiles      RequestType = "GetFanCurveProfiles"
	ReqSetActiveFanCurveProfile RequestType = "SetActiveFanCurveProfile"
	ReqSaveFanCurveProfile      RequestType = "SaveFanCurveProfile"
	ReqDeleteFanCurveProfile    RequestType = "DeleteFanCurveProfile"
	ReqExportFanCurveProfiles   RequestType = "ExportFanCurveProfiles"
	ReqImportFanCurveProfiles   RequestType = "ImportFanCurveProfiles"

	// Control related
	ReqSetAutoControl    RequestType = "SetAutoControl"
	ReqSetManualGear     RequestType = "SetManualGear"
	ReqGetAvailableGears RequestType = "GetAvailableGears"
	ReqSetCustomSpeed    RequestType = "SetCustomSpeed"
	ReqSetGearLight      RequestType = "SetGearLight"
	ReqSetPowerOnStart   RequestType = "SetPowerOnStart"
	ReqSetSmartStartStop RequestType = "SetSmartStartStop"
	ReqSetBrightness     RequestType = "SetBrightness"
	ReqSetLightStrip     RequestType = "SetLightStrip"

	// Temperature related
	ReqGetTemperature         RequestType = "GetTemperature"
	ReqTestTemperatureReading RequestType = "TestTemperatureReading"
	ReqTestBridgeProgram      RequestType = "TestBridgeProgram"
	ReqGetBridgeProgramStatus RequestType = "GetBridgeProgramStatus"

	// Auto-start related
	ReqSetWindowsAutoStart    RequestType = "SetWindowsAutoStart"
	ReqCheckWindowsAutoStart  RequestType = "CheckWindowsAutoStart"
	ReqIsRunningAsAdmin       RequestType = "IsRunningAsAdmin"
	ReqGetAutoStartMethod     RequestType = "GetAutoStartMethod"
	ReqSetAutoStartWithMethod RequestType = "SetAutoStartWithMethod"

	// Window related
	ReqShowWindow RequestType = "ShowWindow"
	ReqHideWindow RequestType = "HideWindow"
	ReqQuitApp    RequestType = "QuitApp"

	// Debug related
	ReqGetDebugInfo          RequestType = "GetDebugInfo"
	ReqSetDebugMode          RequestType = "SetDebugMode"
	ReqUpdateGuiResponseTime RequestType = "UpdateGuiResponseTime"

	// System related
	ReqPing              RequestType = "Ping"
	ReqIsAutoStartLaunch RequestType = "IsAutoStartLaunch"
	ReqSubscribeEvents   RequestType = "SubscribeEvents"
	ReqUnsubscribeEvents RequestType = "UnsubscribeEvents"
)

// Request IPC request
type Request struct {
	Type RequestType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Response IPC response
type Response struct {
	IsResponse bool            `json:"isResponse"` // Identifies this as a response, not an event
	Success    bool            `json:"success"`
	Error      string          `json:"error,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// Event IPC event (pushed from server to client)
type Event struct {
	IsEvent bool            `json:"isEvent"` // Identifies this as an event
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// EventType event types
const (
	EventFanDataUpdate      = "fan-data-update"
	EventTemperatureUpdate  = "temperature-update"
	EventDeviceConnected    = "device-connected"
	EventDeviceDisconnected = "device-disconnected"
	EventDeviceError        = "device-error"
	EventConfigUpdate       = "config-update"
	EventHotkeyTriggered    = "hotkey-triggered"
	EventHealthPing         = "health-ping"
	EventHeartbeat          = "heartbeat"
)

// Server IPC server
type Server struct {
	listener net.Listener
	clients  map[net.Conn]bool
	mutex    sync.RWMutex
	handler  RequestHandler
	logger   types.Logger
	running  bool
}

// RequestHandler request handler function type
type RequestHandler func(req Request) Response

// NewServer creates an IPC server
func NewServer(handler RequestHandler, logger types.Logger) *Server {
	return &Server{
		clients: make(map[net.Conn]bool),
		handler: handler,
		logger:  logger,
	}
}

// Start starts the server
func (s *Server) Start() error {
	// Create named pipe listener
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;WD)", // Allow all users access
	}

	listener, err := winio.ListenPipe(PipePath, cfg)
	if err != nil {
		return fmt.Errorf("failed to create named pipe: %v", err)
	}

	s.listener = listener
	s.running = true
	s.logInfo("IPC server started: %s", PipePath)

	// Accept connections
	go s.acceptConnections()

	return nil
}

// acceptConnections accepts client connections
func (s *Server) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				s.logError("Failed to accept connection: %v", err)
			}
			continue
		}

		s.mutex.Lock()
		s.clients[conn] = true
		s.mutex.Unlock()

		s.logInfo("New IPC client connected")
		go s.handleClient(conn)
	}
}

// handleClient handles a client connection
func (s *Server) handleClient(conn net.Conn) {
	defer func() {
		s.mutex.Lock()
		delete(s.clients, conn)
		s.mutex.Unlock()
		conn.Close()
		s.logInfo("IPC client disconnected")
	}()

	reader := bufio.NewReader(conn)

	for s.running {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			s.logDebug("Failed to read client request: %v", err)
			return
		}

		// Parse request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.logError("Failed to parse request: %v", err)
			continue
		}
		resp := s.handler(req)
		resp.IsResponse = true

		// Send response
		respBytes, err := json.Marshal(resp)
		if err != nil {
			s.logError("Failed to serialize response: %v", err)
			continue
		}

		_, err = conn.Write(append(respBytes, '\n'))
		if err != nil {
			s.logError("Failed to send response: %v", err)
			return
		}
	}
}

// BroadcastEvent broadcasts an event to all clients
func (s *Server) BroadcastEvent(eventType string, data any) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		s.logError("Failed to serialize event data: %v", err)
		return
	}

	event := Event{
		IsEvent: true, // Mark as event
		Type:    eventType,
		Data:    dataBytes,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		s.logError("Failed to serialize event: %v", err)
		return
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for conn := range s.clients {
		go func(c net.Conn) {
			_, err := c.Write(append(eventBytes, '\n'))
			if err != nil {
				s.logDebug("Failed to send event: %v", err)
			}
		}(conn)
	}
}

// Stop stops the server
func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}

	s.mutex.Lock()
	for conn := range s.clients {
		conn.Close()
	}
	s.clients = make(map[net.Conn]bool)
	s.mutex.Unlock()

	s.logInfo("IPC server stopped")
}

// HasClients checks if any clients are connected
func (s *Server) HasClients() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.clients) > 0
}

// Log helper methods
func (s *Server) logInfo(format string, v ...any) {
	if s.logger != nil {
		s.logger.Info(format, v...)
	}
}

func (s *Server) logError(format string, v ...any) {
	if s.logger != nil {
		s.logger.Error(format, v...)
	}
}

func (s *Server) logDebug(format string, v ...any) {
	if s.logger != nil {
		s.logger.Debug(format, v...)
	}
}

// Client IPC client
type Client struct {
	conn         net.Conn
	mutex        sync.Mutex
	reader       *bufio.Reader
	logger       types.Logger
	eventHandler func(Event)
	responseChan chan *Response
	connected    bool
	connMutex    sync.RWMutex
}

// NewClient creates an IPC client
func NewClient(logger types.Logger) *Client {
	return &Client{
		logger:       logger,
		responseChan: make(chan *Response, 1),
	}
}

// Connect connects to the server
func (c *Client) Connect() error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.connected {
		return nil
	}

	timeout := 5 * time.Second
	conn, err := winio.DialPipe(PipePath, &timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to IPC server: %v", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.connected = true
	c.logInfo("Connected to IPC server")

	// Start message receive loop
	go c.readLoop()

	return nil
}

// readLoop unified message read loop
func (c *Client) readLoop() {
	for {
		c.connMutex.RLock()
		if !c.connected || c.reader == nil {
			c.connMutex.RUnlock()
			return
		}
		reader := c.reader
		c.connMutex.RUnlock()

		line, err := reader.ReadBytes('\n')
		if err != nil {
			c.logDebug("Failed to read message: %v", err)
			c.connMutex.Lock()
			c.connected = false
			c.connMutex.Unlock()
			return
		}

		// Use generic struct to detect message type
		var msg struct {
			IsResponse bool `json:"isResponse"`
			IsEvent    bool `json:"isEvent"`
		}
		if err := json.Unmarshal(line, &msg); err != nil {
			c.logDebug("Failed to parse message type: %v", err)
			continue
		}

		if msg.IsResponse {
			var resp Response
			if err := json.Unmarshal(line, &resp); err == nil {
				select {
				case c.responseChan <- &resp:
				default:
					c.logDebug("Response channel full, discarding response")
				}
			}
		} else if msg.IsEvent {
			var event Event
			if err := json.Unmarshal(line, &event); err == nil && event.Type != "" {
				if c.eventHandler != nil {
					go c.eventHandler(event)
				}
			}
		}
	}
}

// SetEventHandler sets the event handler function
func (c *Client) SetEventHandler(handler func(Event)) {
	c.eventHandler = handler
}

// SendRequest sends a request and waits for a response
func (c *Client) SendRequest(reqType RequestType, data any) (*Response, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connMutex.RLock()
	if !c.connected || c.conn == nil {
		c.connMutex.RUnlock()
		return nil, fmt.Errorf("not connected to server")
	}
	conn := c.conn
	c.connMutex.RUnlock()

	var dataBytes json.RawMessage
	if data != nil {
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request data: %v", err)
		}
	}

	req := Request{
		Type: reqType,
		Data: dataBytes,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %v", err)
	}

	// Clear response channel
	select {
	case <-c.responseChan:
	default:
	}

	_, err = conn.Write(append(reqBytes, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	select {
	case resp := <-c.responseChan:
		return resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timed out waiting for response")
	}
}

// Close closes the connection
func (c *Client) Close() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	c.connected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// IsConnected checks if connected
func (c *Client) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// Log helper methods
func (c *Client) logInfo(format string, v ...any) {
	if c.logger != nil {
		c.logger.Info(format, v...)
	}
}

func (c *Client) logDebug(format string, v ...any) {
	if c.logger != nil {
		c.logger.Debug(format, v...)
	}
}

// CheckCoreServiceRunning checks if the core service is running
func CheckCoreServiceRunning() bool {
	timeout := 1 * time.Second
	conn, err := winio.DialPipe(PipePath, &timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetCoreLockFilePath gets the core service lock file path
func GetCoreLockFilePath() string {
	tempDir := os.TempDir()
	return fmt.Sprintf("%s/bs2pro-core.lock", tempDir)
}

// StartCoreRequestParams request params for starting the core service
type StartCoreRequestParams struct {
	ShowGUI bool `json:"showGUI"`
}

// SetAutoControlParams params for setting smart fan control
type SetAutoControlParams struct {
	Enabled bool `json:"enabled"`
}

// SetManualGearParams params for setting manual gear
type SetManualGearParams struct {
	Gear  string `json:"gear"`
	Level string `json:"level"`
}

// SetCustomSpeedParams params for setting custom speed
type SetCustomSpeedParams struct {
	Enabled bool `json:"enabled"`
	RPM     int  `json:"rpm"`
}

// SetBoolParams boolean params
type SetBoolParams struct {
	Enabled bool `json:"enabled"`
}

// SetStringParams string params
type SetStringParams struct {
	Value string `json:"value"`
}

// SetIntParams integer params
type SetIntParams struct {
	Value int `json:"value"`
}

// SetAutoStartWithMethodParams params for setting auto-start method
type SetAutoStartWithMethodParams struct {
	Enable bool   `json:"enable"`
	Method string `json:"method"`
}

// SetLightStripParams params for setting light strip
type SetLightStripParams struct {
	Config types.LightStripConfig `json:"config"`
}

// SetActiveFanCurveProfileParams params for setting active fan curve profile
type SetActiveFanCurveProfileParams struct {
	ID string `json:"id"`
}

// SaveFanCurveProfileParams params for saving a fan curve profile
type SaveFanCurveProfileParams struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Curve     []types.FanCurvePoint `json:"curve"`
	SetActive bool                  `json:"setActive"`
}

// DeleteFanCurveProfileParams params for deleting a fan curve profile
type DeleteFanCurveProfileParams struct {
	ID string `json:"id"`
}

// ImportFanCurveProfilesParams params for importing fan curve profiles
type ImportFanCurveProfilesParams struct {
	Code string `json:"code"`
}
