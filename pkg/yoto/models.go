package yoto

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Card represents a Yoto card (playlist or physical card)
type Card struct {
	CardID    string    `json:"cardId"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Content   *Content  `json:"content"`
	Metadata  *Metadata `json:"metadata"`

	extra extras // fields the API returned that this client does not model
}

// Content contains the actual audio structure
type Content struct {
	Chapters []Chapter `json:"chapters"`

	extra extras
}

// Chapter represents a group of tracks (usually 1:1 with tracks for MYO)
type Chapter struct {
	Key          string  `json:"key"`
	Title        string  `json:"title"`
	Duration     int     `json:"duration"`
	Tracks       []Track `json:"tracks"`
	Display      Display `json:"display"`
	OverlayLabel string  `json:"overlayLabel,omitempty"`

	extra extras
}

// Track represents a single audio file
type Track struct {
	Key          string  `json:"key"`
	Title        string  `json:"title"`
	TrackURL     string  `json:"trackUrl"`
	Duration     int     `json:"duration"`
	FileSize     int     `json:"fileSize"`
	Format       string  `json:"format"`
	Display      Display `json:"display"`
	OverlayLabel string  `json:"overlayLabel,omitempty"`
	Type         string  `json:"type"`

	extra extras
}

// Display holds icon information
type Display struct {
	Icon16x16 string `json:"icon16x16"`

	extra extras
}

// Metadata holds descriptive info
//
// Author and Description are omitted when empty: an empty one means "not being
// changed" everywhere it is set from, and writing it would add a field to cards
// that never had one.
type Metadata struct {
	Author      string `json:"author,omitempty"`
	Description string `json:"description,omitempty"`
	Media       Media  `json:"media"`

	extra extras
}

// Media holds aggregate stats
type Media struct {
	Duration int `json:"duration"`
	FileSize int `json:"fileSize"`

	extra extras
}

// LibraryResponse is the top-level response from /card/family/library
type LibraryResponse struct {
	Cards []LibraryItem `json:"cards"`
}

type LibraryItem struct {
	CardID string `json:"cardId"`
	Card   Card   `json:"card"`
}

// Device represents a Yoto player
type Device struct {
	ID         string `json:"deviceId"`
	Name       string `json:"name"`
	DeviceType string `json:"deviceType"`
	Online     bool   `json:"online"`
	Status     *DeviceStatus
}

type DeviceStatus struct {
	StatusVersion  int    `json:"statusVersion,omitempty"`
	FwVersion      string `json:"fwVersion,omitempty"`
	ProductType    string `json:"productType,omitempty"`
	BatteryLevel   int    `json:"batteryLevel"`
	Charging       bool   `json:"charging"`
	FreeDisk       int    `json:"freeDisk,omitempty"`
	ALS            int    `json:"als,omitempty"`
	ActiveCard     string `json:"activeCard"`
	CardInserted   bool   `json:"cardInserted,omitempty"`
	PlayingStatus  string `json:"playingStatus,omitempty"`
	Headphones     bool   `json:"headphones,omitempty"`
	BluetoothHp    bool   `json:"bluetoothHp,omitempty"`
	Volume         int    `json:"volume"`
	UserVolume     int    `json:"userVolume,omitempty"`
	TimeFormat     string `json:"timeFormat,omitempty"`
	NightlightMode string `json:"nightlightMode,omitempty"`
	Day            bool   `json:"day,omitempty"`
}

// UnmarshalJSON handles both the schema documented in API specs (bools/strings)
// and the schema sent by physical devices (0/1 integers for booleans).
func (s *DeviceStatus) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	getInt := func(k string) int {
		if v, ok := raw[k].(float64); ok {
			return int(v)
		}
		return 0
	}
	getString := func(k string) string {
		if v, ok := raw[k].(string); ok {
			return v
		}
		return ""
	}
	getBool := func(k string) bool {
		switch v := raw[k].(type) {
		case bool:
			return v
		case float64:
			return v != 0
		case string:
			return strings.ToLower(v) == "true" || v == "1"
		default:
			return false
		}
	}
	getPlayingStatus := func() string {
		switch v := raw["playingStatus"].(type) {
		case string:
			return v
		case float64:
			switch int(v) {
			case 0:
				return "stopped"
			case 1:
				return "playing"
			case 2:
				return "paused"
			default:
				return fmt.Sprintf("%d", int(v))
			}
		default:
			return ""
		}
	}

	s.StatusVersion = getInt("statusVersion")
	s.FwVersion = getString("fwVersion")
	s.ProductType = getString("productType")
	s.BatteryLevel = getInt("batteryLevel")
	s.Charging = getBool("charging")
	s.FreeDisk = getInt("freeDisk")
	s.ALS = getInt("als")
	s.ActiveCard = getString("activeCard")
	s.CardInserted = getBool("cardInserted")
	s.PlayingStatus = getPlayingStatus()
	s.Headphones = getBool("headphones")
	s.BluetoothHp = getBool("bluetoothHp")
	s.Volume = getInt("volume")
	s.UserVolume = getInt("userVolume")
	s.TimeFormat = getString("timeFormat")
	s.NightlightMode = getString("nightlightMode")
	s.Day = getBool("day")

	return nil
}

type DevicesResponse struct {
	Devices []Device `json:"devices"`
}

// AudioSHA256 identifies the audio a track plays: the SHA-256 of its transcoded
// file, in unpadded base64url. Empty if the URL carries no hash.
//
// The API gives that hash in two shapes, and they have to be treated as the same
// thing. A track written to a card names its audio as "yoto:#<hash>"; the same
// track read back names it with a signed CDN URL, which carries the hash in a
// "#sha256=<hash>" fragment.
func AudioSHA256(trackURL string) string {
	_, fragment, ok := strings.Cut(trackURL, "#")
	if !ok {
		return ""
	}
	return strings.TrimPrefix(fragment, "sha256=")
}

// IconRef normalizes an icon to the "yoto:#<hash>" form a card has to be written
// with. Icons are read back as https URLs ending in the hash, and writing one of
// those back leaves the track with no icon at all.
//
// Anything else is returned unchanged, including a URL that does not end in what
// looks like a hash: better to send it and let the API rule on it than to mangle
// it here.
func IconRef(icon string) string {
	if !strings.HasPrefix(icon, "http") {
		return icon
	}

	hash, _, _ := strings.Cut(icon, "?")
	if idx := strings.LastIndex(hash, "/"); idx != -1 {
		hash = hash[idx+1:]
	}
	if len(hash) != sha256RefLength {
		return icon
	}
	return "yoto:#" + hash
}

// sha256RefLength is how long a SHA-256 is once Yoto has written it out: 32 bytes
// in unpadded base64url.
const sha256RefLength = 43
