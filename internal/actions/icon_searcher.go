package actions

// Icon represents a generic icon across different providers (Yoto, yotoicons.com, etc.).
type Icon interface {
	// ID returns the unique identifier or reference for the icon (e.g. media ID or hash).
	ID() string
	// Title returns the title or human-friendly label for the icon.
	Title() string
	// Provider returns the icon source ("yoto", "yotoicons.com", etc.).
	Provider() string
	// Bytes returns the raw image data (typically 16x16 PNG).
	Bytes() ([]byte, error)
	// SHA256 returns the SHA-256 hex digest of the icon's image bytes.
	SHA256() (string, error)
	// Attribution returns author or attribution information for the icon.
	Attribution() string
}

// IconSearcher searches for icons by keyword across an icon provider.
type IconSearcher interface {
	// SearchForIcon finds icons matching any one of the given keywords.
	SearchForIcon(keywords []string) ([]Icon, error)
}
