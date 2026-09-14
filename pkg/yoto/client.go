package yoto

import "errors"

// YotoClient is the full set of operations this package exposes. Commands that
// talk to the Yoto REST API go through the HTTP sub-client; commands that talk
// to individual players go through MQTT.
type YotoClient interface {
	// ── Library (HTTP) ───────────────────────────────────────────────────
	ListCards() ([]Card, error)
	GetCard(id string) (*Card, error)
	DeleteCard(id string) error
	CreateCard(card *Card) error
	UpdateCard(id string, card *Card) error
	DownloadFile(url string, destPath string) error
	UploadIcon(path string) (string, error)
	GetPublicIcons() ([]DisplayIcon, error)
	GetUserIcons() ([]DisplayIcon, error)
	FetchBytes(url string) ([]byte, error)
	GetUploadURL(sha256Hash string, filename string) (*UploadURLResponse, error)
	UploadFile(path string, uploadURL string) error
	PollTranscode(uploadID string) (*TranscodeData, error)

	// ── Devices (HTTP for listing, MQTT for control) ─────────────────────
	ListDevices() ([]Device, error)

	// ── Device control (MQTT) ────────────────────────────────────────────
	GetDeviceStatus(deviceID string) (*DeviceStatus, error)
	SetVolume(deviceID string, volume int) error
	PlayCard(deviceID string, cardID string) error
	StopPlayer(deviceID string) error
	PausePlayer(deviceID string) error

	// ── Auth (HTTP) ──────────────────────────────────────────────────────
	AuthorizeURLFor(challenge, state string) string
	ExchangeCode(code, verifier string) (*TokenResponse, error)
	RefreshToken(refreshToken string) (*TokenResponse, error)

	// ── Lifecycle ────────────────────────────────────────────────────────
	Close() error
}

// ClientOption configures a Client at construction time.
type ClientOption func(*Client)

// WithMQTTBrokerURL overrides the default MQTT broker. Useful in tests.
func WithMQTTBrokerURL(url string) ClientOption {
	return func(c *Client) {
		c.mqttClient.brokerURL = url
	}
}

// Client is the top-level wrapper that every command receives. It routes each
// call to the HTTP or MQTT sub-client as appropriate.
type Client struct {
	httpClient *HTTPClient
	mqttClient *MQTTClient
}

// Compile-time check.
var _ YotoClient = (*Client)(nil)

// NewClient creates a Client ready to talk to both the REST API and the MQTT
// broker. The optional ClientOption values let callers override defaults (the
// MQTT broker URL, for instance).
func NewClient(token, clientID string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: newHTTPClient(token, clientID),
		mqttClient: newMQTTClient(token),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ── Library (HTTP) ───────────────────────────────────────────────────────────

func (c *Client) ListCards() ([]Card, error)             { return c.httpClient.ListCards() }
func (c *Client) GetCard(id string) (*Card, error)       { return c.httpClient.GetCard(id) }
func (c *Client) DeleteCard(id string) error             { return c.httpClient.DeleteCard(id) }
func (c *Client) CreateCard(card *Card) error            { return c.httpClient.CreateCard(card) }
func (c *Client) UpdateCard(id string, card *Card) error { return c.httpClient.UpdateCard(id, card) }
func (c *Client) DownloadFile(url string, destPath string) error {
	return c.httpClient.DownloadFile(url, destPath)
}
func (c *Client) UploadIcon(path string) (string, error) { return c.httpClient.UploadIcon(path) }
func (c *Client) GetPublicIcons() ([]DisplayIcon, error)  { return c.httpClient.GetPublicIcons() }
func (c *Client) GetUserIcons() ([]DisplayIcon, error)    { return c.httpClient.GetUserIcons() }
func (c *Client) FetchBytes(url string) ([]byte, error)   { return c.httpClient.FetchBytes(url) }
func (c *Client) GetUploadURL(hash string, filename string) (*UploadURLResponse, error) {
	return c.httpClient.GetUploadURL(hash, filename)
}
func (c *Client) UploadFile(path string, uploadURL string) error {
	return c.httpClient.UploadFile(path, uploadURL)
}
func (c *Client) PollTranscode(uploadID string) (*TranscodeData, error) {
	return c.httpClient.PollTranscode(uploadID)
}

// ── Devices ──────────────────────────────────────────────────────────────────

func (c *Client) ListDevices() ([]Device, error) { return c.httpClient.ListDevices() }

// ── Device control (MQTT) ────────────────────────────────────────────────────

func (c *Client) GetDeviceStatus(deviceID string) (*DeviceStatus, error) {
	return c.mqttClient.GetDeviceStatus(deviceID)
}
func (c *Client) SetVolume(deviceID string, volume int) error {
	return c.mqttClient.SetVolume(deviceID, volume)
}
func (c *Client) PlayCard(deviceID string, cardID string) error {
	return c.mqttClient.PlayCard(deviceID, cardID)
}
func (c *Client) StopPlayer(deviceID string) error  { return c.mqttClient.StopPlayer(deviceID) }
func (c *Client) PausePlayer(deviceID string) error { return c.mqttClient.PausePlayer(deviceID) }

// ── Auth (HTTP) ──────────────────────────────────────────────────────────────

func (c *Client) AuthorizeURLFor(challenge, state string) string {
	return c.httpClient.AuthorizeURLFor(challenge, state)
}
func (c *Client) ExchangeCode(code, verifier string) (*TokenResponse, error) {
	return c.httpClient.ExchangeCode(code, verifier)
}
func (c *Client) RefreshToken(refreshToken string) (*TokenResponse, error) {
	return c.httpClient.RefreshToken(refreshToken)
}

func (c *Client) SetBaseURL(url string) {
	c.httpClient.SetBaseURL(url)
}

// ── Lifecycle ────────────────────────────────────────────────────────────────

func (c *Client) Close() error {
	var errs []error
	if err := c.mqttClient.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := c.httpClient.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
