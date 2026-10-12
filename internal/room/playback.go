package room

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"twintube/internal/utils"
)

// ErrInvalidPlayback marks a client playback report that fails validation.
var ErrInvalidPlayback = errors.New("invalid playback report")

const (
	maxPlaybackVideoIDBytes = 2048
	maxPlaybackSourceBytes  = 2048
	maxPlaybackSeconds      = 10_000_000 // about 115 days, far beyond any real session
	maxPlaybackTitleRunes   = 200
)

var allowedPlaybackPlatforms = map[string]bool{
	"youtube":     true,
	"vimeo":       true,
	"twitch":      true,
	"dailymotion": true,
	"streamable":  true,
	"peertube":    true,
	"direct":      true,
	"hls":         true,
	"local":       true,
}

// videoIDPatterns mirror the identities utils.ExtractVideoInfo produces for each
// platform. Direct and HLS sources are validated as HTTP(S) URLs instead.
var videoIDPatterns = map[string]*regexp.Regexp{
	"youtube":     regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`),
	"vimeo":       regexp.MustCompile(`^[0-9]{1,20}$`),
	"twitch":      regexp.MustCompile(`^(vod:[0-9]{1,20}|channel:[A-Za-z0-9_]{3,25})$`),
	"dailymotion": regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`),
	"streamable":  regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`),
	"peertube":    regexp.MustCompile(`^[^\s|/]{1,255}\|[A-Za-z0-9_-]{10,}$`),
	"local":       regexp.MustCompile(`^local:[a-f0-9]{16,64}$`),
}

// PlaybackReport is a client's playback state after validation.
type PlaybackReport struct {
	VideoID     string
	Status      string
	CurrentTime float64
	Title       string
	Meta        MediaMeta
}

// NewPlaybackReport validates a playback update from a client before it can change
// shared room state. Clients may only report PLAYING or PAUSED, a known platform,
// and an identity that matches that platform. Source URLs may not use script or
// data schemes, because the browser loads them into the player. Times are clamped
// to a sane range and titles are cut to a display length.
func NewPlaybackReport(videoID, status string, currentTime float64, title string, meta MediaMeta) (PlaybackReport, error) {
	if status != "PLAYING" && status != "PAUSED" {
		return PlaybackReport{}, invalidPlayback(fmt.Sprintf("unknown status %q", status))
	}
	if math.IsNaN(currentTime) || math.IsInf(currentTime, 0) {
		return PlaybackReport{}, invalidPlayback("non-finite playback time")
	}
	if currentTime < 0 {
		currentTime = 0
	}
	if currentTime > maxPlaybackSeconds {
		currentTime = maxPlaybackSeconds
	}
	if meta.MediaKind != "" && meta.MediaKind != "vod" && meta.MediaKind != "live" {
		return PlaybackReport{}, invalidPlayback(fmt.Sprintf("unknown media kind %q", meta.MediaKind))
	}
	if len(videoID) > maxPlaybackVideoIDBytes || hasControlChars(videoID) {
		return PlaybackReport{}, invalidPlayback("malformed video id")
	}

	platform := meta.Platform
	if videoID != "" && platform == "" {
		platform = inferPlaybackPlatform(videoID)
	}
	if platform != "" && !allowedPlaybackPlatforms[platform] {
		return PlaybackReport{}, invalidPlayback(fmt.Sprintf("unknown platform %q", platform))
	}
	if videoID != "" && !validPlaybackVideoID(platform, videoID) {
		return PlaybackReport{}, invalidPlayback(fmt.Sprintf("video id does not match platform %q", platform))
	}
	if err := validatePlaybackSource(platform, meta.SourceURL); err != nil {
		return PlaybackReport{}, err
	}

	return PlaybackReport{
		VideoID:     videoID,
		Status:      status,
		CurrentTime: currentTime,
		Title:       sanitizePlaybackTitle(title),
		Meta: MediaMeta{
			Platform:  platform,
			MediaKind: meta.MediaKind,
			SourceURL: meta.SourceURL,
			Seekable:  meta.Seekable,
		},
	}, nil
}

// inferPlaybackPlatform mirrors how UpdateVideoState labels an identity that
// arrives without a platform.
func inferPlaybackPlatform(videoID string) string {
	switch {
	case strings.HasPrefix(videoID, "local:"):
		return "local"
	case strings.HasPrefix(videoID, "vod:"), strings.HasPrefix(videoID, "channel:"):
		return "twitch"
	case isHTTPURL(videoID) && strings.Contains(strings.ToLower(videoID), ".m3u8"):
		return "hls"
	case isHTTPURL(videoID):
		return "direct"
	default:
		return "youtube"
	}
}

func validPlaybackVideoID(platform, videoID string) bool {
	switch platform {
	case "direct", "hls":
		return isHTTPURL(videoID)
	}
	pattern, ok := videoIDPatterns[platform]
	return ok && pattern.MatchString(videoID)
}

func validatePlaybackSource(platform, source string) error {
	if source == "" {
		return nil
	}
	if len(source) > maxPlaybackSourceBytes || hasControlChars(source) {
		return invalidPlayback("malformed source url")
	}
	if platform == "local" {
		if strings.HasPrefix(source, "local:") {
			return nil
		}
		return invalidPlayback("local media must use a local: source")
	}
	if hasDisallowedScheme(source) {
		return invalidPlayback("source url scheme is not allowed")
	}
	return nil
}

// hasDisallowedScheme reports whether s starts with a URL scheme other than http or
// https. A colon that appears after a slash, query, or fragment is part of a path,
// so bare hosts like "vimeo.com/123" pass.
func hasDisallowedScheme(s string) bool {
	idx := strings.IndexByte(s, ':')
	if idx < 0 || strings.ContainsAny(s[:idx], "/?#") {
		return false
	}
	scheme := strings.ToLower(s[:idx])
	return scheme != "http" && scheme != "https"
}

func isHTTPURL(s string) bool {
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Host != ""
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func sanitizePlaybackTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	return utils.TruncateRunes(strings.TrimSpace(b.String()), maxPlaybackTitleRunes)
}

func invalidPlayback(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPlayback, reason)
}
