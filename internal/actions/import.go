package actions

import (
	"log/slog"

	"github.com/vgaro/yotocli/internal/processing"
	"github.com/vgaro/yotocli/pkg/yoto"
)

// ImportFromURL downloads everything a URL holds and adds it to one playlist.
//
// With syncPlaylist set the playlist is made to match the URL rather than added
// to, which is how a feed that has gained an episode is re-imported without
// ending up with two copies of every old one. See AddTracks for what that keeps
// and what it removes.
func ImportFromURL(client *yoto.Client, url string, playlistName string, syncPlaylist bool) error {
	slog.Info("Downloading audio from URL", "url", url)
	downloads, cleanup, err := processing.DownloadFromURL(url)
	defer func() {
		if err := cleanup(); err != nil {
			slog.Warn("Failed to remove downloaded files", "error", err)
		}
	}()
	if err != nil {
		return err
	}

	slog.Info("Downloaded tracks from URL", "count", len(downloads))
	
	// If no playlist specified, use the title of the first download
	targetPlaylist := playlistName
	if targetPlaylist == "" {
		targetPlaylist = downloads[0].Name
	}

	// d.Name is passed as the track title: the downloaded file is named after
	// the yt-dlp ID, so leaving the title to be derived from it would put
	// things like "Ixrje2rXLMA" on the card.
	tracks := make([]Track, len(downloads))
	for i, d := range downloads {
		tracks[i] = Track{Path: d.Path, Title: d.Name}
	}

	return AddTracks(client, targetPlaylist, tracks, syncPlaylist)
}
