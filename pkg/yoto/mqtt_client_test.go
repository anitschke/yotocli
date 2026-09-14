package yoto

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// mockBroker runs an in-process MQTT broker and returns its TCP broker URL
// (e.g. "tcp://127.0.0.1:12345") and a cleanup function.
func startMockBroker(t *testing.T) (string, func()) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}

	silentLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mochi.New(&mochi.Options{
		Logger: silentLogger,
	})
	_ = server.AddHook(new(auth.AllowHook), nil)

	netListener := listeners.NewNet("test-tcp", l)
	if err := server.AddListener(netListener); err != nil {
		l.Close()
		t.Fatalf("failed to add listener: %v", err)
	}

	go func() {
		_ = server.Serve()
	}()

	addr := l.Addr().String()
	brokerURL := fmt.Sprintf("tcp://%s", addr)

	cleanup := func() {
		_ = server.Close()
	}

	return brokerURL, cleanup
}

// mockDevice simulates a physical Yoto player connected to the broker.
type mockDevice struct {
	deviceID   string
	client     paho.Client
	mu         sync.Mutex
	lastVolume int
	lastCard   string
	stopped    bool
	paused     bool
}

func startMockDevice(t *testing.T, brokerURL, deviceID string) *mockDevice {
	t.Helper()

	dev := &mockDevice{
		deviceID: deviceID,
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("DEVICE_" + deviceID)
	opts.SetCleanSession(true)

	dev.client = paho.NewClient(opts)
	tok := dev.client.Connect()
	if !tok.WaitTimeout(3*time.Second) || tok.Error() != nil {
		t.Fatalf("mock device connect failed: %v", tok.Error())
	}

	// Subscribe to commands for this device
	cmdTopic := fmt.Sprintf("device/%s/command/#", deviceID)
	subTok := dev.client.Subscribe(cmdTopic, 0, func(_ paho.Client, msg paho.Message) {
		dev.handleCommand(msg.Topic(), msg.Payload())
	})
	if !subTok.WaitTimeout(3*time.Second) || subTok.Error() != nil {
		t.Fatalf("mock device subscribe failed: %v", subTok.Error())
	}

	t.Cleanup(func() {
		dev.client.Disconnect(100)
	})

	return dev
}

func (d *mockDevice) handleCommand(topic string, payload []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	statusReqTopic := fmt.Sprintf("device/%s/command/status/request", d.deviceID)
	volSetTopic := fmt.Sprintf("device/%s/command/volume/set", d.deviceID)
	cardStartTopic := fmt.Sprintf("device/%s/command/card/start", d.deviceID)
	cardStopTopic := fmt.Sprintf("device/%s/command/card/stop", d.deviceID)
	cardPauseTopic := fmt.Sprintf("device/%s/command/card/pause", d.deviceID)

	statusDataTopic := fmt.Sprintf("device/%s/data/status", d.deviceID)
	responseTopic := fmt.Sprintf("device/%s/response", d.deviceID)

	switch topic {
	case statusReqTopic:
		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"statusVersion":  2,
				"fwVersion":      "v2.15.0",
				"productType":    "player-v2",
				"batteryLevel":   85,
				"charging":       true,
				"freeDisk":       1024000,
				"als":            42,
				"activeCard":     "card123",
				"cardInserted":   true,
				"playingStatus":  "playing",
				"headphones":     false,
				"bluetoothHp":    false,
				"volume":         d.lastVolume,
				"userVolume":     10,
				"timeFormat":     "24",
				"nightlightMode": "off",
				"day":            true,
			},
		}
		data, _ := json.Marshal(resp)
		d.client.Publish(statusDataTopic, 0, false, data)

	case volSetTopic:
		var p struct {
			Volume int `json:"volume"`
		}
		_ = json.Unmarshal(payload, &p)
		d.lastVolume = p.Volume

		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"volume":   "OK",
				"req_body": string(payload),
			},
		}
		data, _ := json.Marshal(resp)
		d.client.Publish(responseTopic, 0, false, data)

	case cardStartTopic:
		var p struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(payload, &p)
		d.lastCard = p.URI
		d.stopped = false
		d.paused = false

		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"card":     "OK",
				"req_body": string(payload),
			},
		}
		data, _ := json.Marshal(resp)
		d.client.Publish(responseTopic, 0, false, data)

	case cardStopTopic:
		d.stopped = true
		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"card":     "OK",
				"req_body": string(payload),
			},
		}
		data, _ := json.Marshal(resp)
		d.client.Publish(responseTopic, 0, false, data)

	case cardPauseTopic:
		d.paused = true
		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"card":     "OK",
				"req_body": string(payload),
			},
		}
		data, _ := json.Marshal(resp)
		d.client.Publish(responseTopic, 0, false, data)
	}
}

func TestMQTT_GetDeviceStatus(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-1"
	dev := startMockDevice(t, brokerURL, deviceID)
	dev.lastVolume = 25

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))
	defer client.Close()

	status, err := client.GetDeviceStatus(deviceID)
	if err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}

	if status.BatteryLevel != 85 {
		t.Errorf("BatteryLevel = %d, want 85", status.BatteryLevel)
	}
	if !status.Charging {
		t.Errorf("Charging = false, want true")
	}
	if status.Volume != 25 {
		t.Errorf("Volume = %d, want 25", status.Volume)
	}
	if status.ActiveCard != "card123" {
		t.Errorf("ActiveCard = %q, want %q", status.ActiveCard, "card123")
	}
	if status.FwVersion != "v2.15.0" {
		t.Errorf("FwVersion = %q, want %q", status.FwVersion, "v2.15.0")
	}
	if status.PlayingStatus != "playing" {
		t.Errorf("PlayingStatus = %q, want %q", status.PlayingStatus, "playing")
	}
}

func TestMQTT_SetVolume(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-2"
	dev := startMockDevice(t, brokerURL, deviceID)

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))
	defer client.Close()

	if err := client.SetVolume(deviceID, 42); err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}

	// Give a moment for mock device state to record
	time.Sleep(50 * time.Millisecond)

	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.lastVolume != 42 {
		t.Errorf("mockDevice lastVolume = %d, want 42", dev.lastVolume)
	}
}

func TestMQTT_PlayCard(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-3"
	dev := startMockDevice(t, brokerURL, deviceID)

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))
	defer client.Close()

	if err := client.PlayCard(deviceID, "card_sleep_tales"); err != nil {
		t.Fatalf("PlayCard failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.lastCard != "card_sleep_tales" {
		t.Errorf("mockDevice lastCard = %q, want card_sleep_tales", dev.lastCard)
	}
}

func TestMQTT_StopAndPausePlayer(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-4"
	dev := startMockDevice(t, brokerURL, deviceID)

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))
	defer client.Close()

	if err := client.PausePlayer(deviceID); err != nil {
		t.Fatalf("PausePlayer failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	dev.mu.Lock()
	if !dev.paused {
		t.Errorf("mockDevice was not paused")
	}
	dev.mu.Unlock()

	if err := client.StopPlayer(deviceID); err != nil {
		t.Fatalf("StopPlayer failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	dev.mu.Lock()
	if !dev.stopped {
		t.Errorf("mockDevice was not stopped")
	}
	dev.mu.Unlock()
}

func TestMQTT_Close(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-5"
	startMockDevice(t, brokerURL, deviceID)

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))

	// Calling GetDeviceStatus opens the connection
	_, err := client.GetDeviceStatus(deviceID)
	if err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}

	// Verify connection is tracked
	client.mqttClient.mu.Lock()
	numConns := len(client.mqttClient.conns)
	client.mqttClient.mu.Unlock()
	if numConns != 1 {
		t.Errorf("expected 1 connection, got %d", numConns)
	}

	// Close client
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify all connections were cleaned up
	client.mqttClient.mu.Lock()
	numConns = len(client.mqttClient.conns)
	client.mqttClient.mu.Unlock()
	if numConns != 0 {
		t.Errorf("expected 0 connections after Close, got %d", numConns)
	}
}

func TestMQTT_CommandFailure(t *testing.T) {
	brokerURL, cleanup := startMockBroker(t)
	defer cleanup()

	deviceID := "test-device-fail"

	// Mock device that responds with FAIL
	opts := paho.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("DEVICE_" + deviceID)
	opts.SetCleanSession(true)
	devClient := paho.NewClient(opts)
	tok := devClient.Connect()
	if !tok.WaitTimeout(3*time.Second) || tok.Error() != nil {
		t.Fatalf("dev connect error: %v", tok.Error())
	}
	defer devClient.Disconnect(100)

	devClient.Subscribe(fmt.Sprintf("device/%s/command/#", deviceID), 0, func(_ paho.Client, msg paho.Message) {
		resp := map[string]interface{}{
			"status": map[string]interface{}{
				"volume": "FAIL",
			},
		}
		data, _ := json.Marshal(resp)
		devClient.Publish(fmt.Sprintf("device/%s/response", deviceID), 0, false, data)
	})

	client := NewClient("fake-token", "fake-client", WithMQTTBrokerURL(brokerURL))
	defer client.Close()

	err := client.SetVolume(deviceID, 10)
	if err == nil {
		t.Fatal("expected error on volume command FAIL")
	}
	if !strings.Contains(err.Error(), "volume command failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}
