package yoto

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	// DefaultMQTTBrokerURL is the Yoto production MQTT broker, reachable over
	// WebSocket-over-TLS with the x-amzn-mqtt-ca ALPN extension.
	DefaultMQTTBrokerURL = "wss://aqrphjqbp3u2z-ats.iot.eu-west-2.amazonaws.com/mqtt"

	mqttConnectTimeout  = 10 * time.Second
	mqttResponseTimeout = 10 * time.Second
)

// MQTTClient talks to Yoto players through an AWS IoT MQTT broker.
//
// Each player requires its own MQTT connection (the username encodes the device
// ID), so the client manages one connection per device, created lazily on first
// use and reused for the lifetime of the client.
type MQTTClient struct {
	brokerURL string
	token     string

	mu    sync.Mutex
	conns map[string]*deviceConn
}

// deviceConn is one live MQTT connection to a single Yoto player.
type deviceConn struct {
	client   mqtt.Client
	deviceID string

	// Inbound messages are routed to these channels by topic. Each has a
	// buffer of 1; a new message replaces any unread one.
	statusCh   chan []byte
	responseCh chan []byte
	eventsCh   chan []byte
}

func newMQTTClient(token string) *MQTTClient {
	return &MQTTClient{
		brokerURL: DefaultMQTTBrokerURL,
		token:     token,
		conns:     make(map[string]*deviceConn),
	}
}

// getOrCreateConn returns a connected, subscribed MQTT client for the given
// device, creating (and caching) one if necessary.
func (m *MQTTClient) getOrCreateConn(deviceID string) (*deviceConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.conns[deviceID]; ok {
		if conn.client.IsConnected() {
			return conn, nil
		}
		// Stale connection; tear it down and reconnect.
		conn.client.Disconnect(0)
	}

	conn := &deviceConn{
		deviceID:   deviceID,
		statusCh:   make(chan []byte, 1),
		responseCh: make(chan []byte, 1),
		eventsCh:   make(chan []byte, 1),
	}

	clientID := "DASH" + deviceID
	username := deviceID + "?x-amz-customauthorizer-name=PublicJWTAuthorizer"

	statusTopic := fmt.Sprintf("device/%s/data/status", deviceID)
	responseTopic := fmt.Sprintf("device/%s/response", deviceID)
	eventsTopic := fmt.Sprintf("device/%s/data/events", deviceID)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(m.brokerURL)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(m.token)
	opts.SetKeepAlive(300 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	opts.SetConnectRetry(false) // we handle retries ourselves

	if strings.HasPrefix(m.brokerURL, "wss://") {
		opts.SetTLSConfig(&tls.Config{})
	}

	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		topic := strings.TrimPrefix(msg.Topic(), "/")
		switch topic {
		case statusTopic:
			drain(conn.statusCh)
			conn.statusCh <- msg.Payload()
		case responseTopic:
			drain(conn.responseCh)
			conn.responseCh <- msg.Payload()
		case eventsTopic:
			drain(conn.eventsCh)
			conn.eventsCh <- msg.Payload()
		}
	})

	// Re-subscribe on every connect (including automatic reconnects).
	topics := map[string]byte{
		statusTopic:   0,
		responseTopic: 0,
		eventsTopic:   0,
	}
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		c.SubscribeMultiple(topics, nil)
	})

	conn.client = mqtt.NewClient(opts)

	tok := conn.client.Connect()
	if !tok.WaitTimeout(mqttConnectTimeout) {
		return nil, fmt.Errorf("mqtt: connect timeout for device %s", deviceID)
	}
	if tok.Error() != nil {
		return nil, fmt.Errorf("mqtt: connect error for device %s: %w", deviceID, tok.Error())
	}

	// Confirm subscription before proceeding with commands
	subTok := conn.client.SubscribeMultiple(topics, nil)
	if !subTok.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("mqtt: subscribe timeout for device %s", deviceID)
	}
	if subTok.Error() != nil {
		return nil, fmt.Errorf("mqtt: subscribe error for device %s: %w", deviceID, subTok.Error())
	}

	m.conns[deviceID] = conn
	return conn, nil
}

// drain removes any pending value from ch so the next send will not block.
func drain(ch <-chan []byte) {
	select {
	case <-ch:
	default:
	}
}

// ── Device control methods ───────────────────────────────────────────────────

// GetDeviceStatus connects to the device via MQTT, requests its status, and
// waits for the response.
func (m *MQTTClient) GetDeviceStatus(deviceID string) (*DeviceStatus, error) {
	conn, err := m.getOrCreateConn(deviceID)
	if err != nil {
		return nil, err
	}

	// Drain stale status messages.
	drain(conn.statusCh)

	topic := fmt.Sprintf("device/%s/command/status/request", deviceID)
	tok := conn.client.Publish(topic, 0, false, "")
	if !tok.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("mqtt: publish timeout for status request on device %s", deviceID)
	}
	if tok.Error() != nil {
		return nil, fmt.Errorf("mqtt: publish error for status request: %w", tok.Error())
	}

	select {
	case payload := <-conn.statusCh:
		var result struct {
			Status DeviceStatus `json:"status"`
		}
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil, fmt.Errorf("mqtt: failed to parse status response: %w", err)
		}
		return &result.Status, nil
	case <-time.After(mqttResponseTimeout):
		return nil, fmt.Errorf("mqtt: timeout waiting for status from device %s", deviceID)
	}
}

// SetVolume publishes a volume/set command and waits for the acknowledgment.
func (m *MQTTClient) SetVolume(deviceID string, volume int) error {
	conn, err := m.getOrCreateConn(deviceID)
	if err != nil {
		return err
	}

	payload, _ := json.Marshal(map[string]int{"volume": volume})
	topic := fmt.Sprintf("device/%s/command/volume/set", deviceID)
	tok := conn.client.Publish(topic, 0, false, payload)
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: publish timeout for volume set on device %s", deviceID)
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt: publish error for volume set: %w", tok.Error())
	}

	return m.waitForResponse(conn, "volume")
}

// PlayCard starts card playback on the device.
func (m *MQTTClient) PlayCard(deviceID string, cardID string) error {
	conn, err := m.getOrCreateConn(deviceID)
	if err != nil {
		return err
	}

	payload, _ := json.Marshal(map[string]string{"uri": cardID})
	topic := fmt.Sprintf("device/%s/command/card/start", deviceID)
	tok := conn.client.Publish(topic, 0, false, payload)
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: publish timeout for card start on device %s", deviceID)
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt: publish error for card start: %w", tok.Error())
	}

	return m.waitForResponse(conn, "card")
}

// StopPlayer stops playback on the device.
func (m *MQTTClient) StopPlayer(deviceID string) error {
	conn, err := m.getOrCreateConn(deviceID)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("device/%s/command/card/stop", deviceID)
	tok := conn.client.Publish(topic, 0, false, "")
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: publish timeout for card stop on device %s", deviceID)
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt: publish error for card stop: %w", tok.Error())
	}

	return m.waitForResponse(conn, "card")
}

// PausePlayer pauses playback on the device.
func (m *MQTTClient) PausePlayer(deviceID string) error {
	conn, err := m.getOrCreateConn(deviceID)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("device/%s/command/card/pause", deviceID)
	tok := conn.client.Publish(topic, 0, false, "")
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt: publish timeout for card pause on device %s", deviceID)
	}
	if tok.Error() != nil {
		return fmt.Errorf("mqtt: publish error for card pause: %w", tok.Error())
	}

	return m.waitForResponse(conn, "card")
}

// waitForResponse waits for an acknowledgment on the device's response topic.
// key is the response field to check (e.g. "volume", "card").
func (m *MQTTClient) waitForResponse(conn *deviceConn, key string) error {
	drain(conn.responseCh)

	select {
	case payload := <-conn.responseCh:
		var result struct {
			Status map[string]json.RawMessage `json:"status"`
		}
		if err := json.Unmarshal(payload, &result); err != nil {
			return fmt.Errorf("mqtt: failed to parse response: %w", err)
		}
		if val, ok := result.Status[key]; ok {
			var s string
			if err := json.Unmarshal(val, &s); err == nil && s == "FAIL" {
				return fmt.Errorf("mqtt: device reported %s command failed", key)
			}
		}
		return nil
	case <-time.After(mqttResponseTimeout):
		// A timeout waiting for the ack is not fatal — the command was
		// published and may have been acted on.
		return nil
	}
}

// Close disconnects every cached MQTT connection.
func (m *MQTTClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, conn := range m.conns {
		conn.client.Disconnect(250) // 250ms quiesce
		delete(m.conns, id)
	}
	return nil
}
