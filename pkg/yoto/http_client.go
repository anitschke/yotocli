package yoto

import (
	"fmt"
	"io"
	"os"

	"github.com/go-resty/resty/v2"
)

const (
	BaseURL = "https://api.yotoplay.com"
)

// HTTPClient handles communication with the Yoto REST API.
type HTTPClient struct {
	http     *resty.Client
	token    string
	clientID string
}

// newHTTPClient creates a new Yoto HTTP API client.
func newHTTPClient(token, clientID string) *HTTPClient {
	client := resty.New()
	client.SetBaseURL(BaseURL)
	client.SetHeader("User-Agent", "Yoto/2.73 (com.yotoplay.Yoto; build:10405; iOS 17.4.0)")

	if token != "" {
		client.SetAuthToken(token)
	}

	return &HTTPClient{
		http:     client,
		token:    token,
		clientID: clientID,
	}
}

func (c *HTTPClient) ListCards() ([]Card, error) {
	var result LibraryResponse
	resp, err := c.http.R().
		SetResult(&result).
		Get("/card/family/library")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("api error: %s", resp.String())
	}

	cards := make([]Card, len(result.Cards))
	for i, item := range result.Cards {
		cards[i] = item.Card
	}
	return cards, nil
}

func (c *HTTPClient) GetCard(id string) (*Card, error) {
	var result struct {
		Card Card `json:"card"`
	}
	resp, err := c.http.R().
		SetResult(&result).
		Get("/card/" + id)

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("api error: %s", resp.String())
	}

	return &result.Card, nil
}

func (c *HTTPClient) DeleteCard(id string) error {
	resp, err := c.http.R().
		Delete("/content/" + id)

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}
	return nil
}

func (c *HTTPClient) UploadIcon(path string) (string, error) {
	var result struct {
		ID string `json:"id"`
	}

	resp, err := c.http.R().
		SetFile("file", path).
		SetFormData(map[string]string{"autoConvert": "true"}).
		SetResult(&result).
		Post("/media/displayIcons/user/me/upload")

	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("api error: %s", resp.String())
	}

	return result.ID, nil
}

func (c *HTTPClient) GetPublicIcons() ([]DisplayIcon, error) {
	var result DisplayIconsResponse
	resp, err := c.http.R().
		SetResult(&result).
		Get("/media/displayIcons/user/yoto")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("api error: %s", resp.String())
	}

	return result.DisplayIcons, nil
}

func (c *HTTPClient) GetUserIcons() ([]DisplayIcon, error) {
	var result DisplayIconsResponse
	resp, err := c.http.R().
		SetResult(&result).
		Get("/media/displayIcons/user/me")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("api error: %s", resp.String())
	}

	return result.DisplayIcons, nil
}

func (c *HTTPClient) FetchBytes(url string) ([]byte, error) {
	resp, err := c.http.R().
		SetDoNotParseResponse(true).
		Get(url)

	if err != nil {
		return nil, err
	}
	defer resp.RawBody().Close()

	if resp.IsError() {
		return nil, fmt.Errorf("download failed: %s", resp.Status())
	}

	return io.ReadAll(resp.RawBody())
}

func (c *HTTPClient) UpdateCard(id string, card *Card) error { // Sanitize icons: Convert https URLs back to yoto:#hash format
	sanitizeCardForUpdate(card)

	// The API for content update seems to use the same endpoint as create (Upsert)
	// We POST to /content, and since the body has cardId, it should update.
	resp, err := c.http.R().
		SetBody(card).
		Post("/content")

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}
	return nil
}

func sanitizeCardForUpdate(card *Card) {
	if card.Content == nil {
		return
	}
	for i := range card.Content.Chapters {
		fixIcon(&card.Content.Chapters[i].Display)
		for j := range card.Content.Chapters[i].Tracks {
			fixIcon(&card.Content.Chapters[i].Tracks[j].Display)

			// Ensure Type is set
			if card.Content.Chapters[i].Tracks[j].Type == "" {
				card.Content.Chapters[i].Tracks[j].Type = "audio"
			}
		}
	}
}

func fixIcon(d *Display) {
	if d == nil {
		return
	}
	d.Icon16x16 = IconRef(d.Icon16x16)
}

func (c *HTTPClient) CreateCard(card *Card) error {
	resp, err := c.http.R().
		SetBody(card).
		Post("/content")

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}
	return nil
}

func (c *HTTPClient) DownloadFile(url string, destPath string) error {
	resp, err := c.http.R().
		SetDoNotParseResponse(true).
		Get(url)

	if err != nil {
		return err
	}
	defer resp.RawBody().Close()

	if resp.IsError() {
		return fmt.Errorf("download failed: %s", resp.Status())
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.RawBody())

	return err

}

func (c *HTTPClient) ListDevices() ([]Device, error) {

	var result DevicesResponse

	resp, err := c.http.R().
		SetResult(&result).
		Get("/device-v2/devices/mine")

	if err != nil {

		return nil, err

	}

	if resp.IsError() {

		return nil, fmt.Errorf("api error: %s", resp.String())

	}

	return result.Devices, nil

}

func (c *HTTPClient) SetBaseURL(url string) {
	c.http.SetBaseURL(url)
}

func (c *HTTPClient) Close() error {
	return nil
}
