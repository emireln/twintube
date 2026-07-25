package utils

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// GenerateRandomID creates a random hex string of given byte length
func GenerateRandomID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length*2]
	}
	return hex.EncodeToString(bytes)
}

// GenerateRoomCode generates a user-friendly 8-character room code (e.g. "a7b9x2kp")
func GenerateRoomCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const codeLen = 8
	b := make([]byte, codeLen)
	randBytes := make([]byte, codeLen)
	rand.Read(randBytes)
	for i := range b {
		b[i] = charset[int(randBytes[i])%len(charset)]
	}
	return string(b)
}

var roomIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,32}$`)

// IsValidRoomID validates user-supplied room codes.
func IsValidRoomID(id string) bool {
	return roomIDPattern.MatchString(strings.TrimSpace(id))
}

// SanitizeNickname trims and caps display names for chat/rooms.
func SanitizeNickname(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if len(name) > 25 {
		name = name[:25]
	}
	return name
}

// ExtractYouTubeID parses various YouTube URL formats and extracts the 11-character video ID
func ExtractYouTubeID(input string) (string, error) {
	input = strings.TrimSpace(input)
	
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{11}$`, input); matched {
		return input, nil
	}

	re := regexp.MustCompile(`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?|shorts)\/|.*[?&]v=)|youtu\.be\/)([^"&?\/ ]{11})`)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("invalid YouTube URL or Video ID: %s", input)
}

// VideoInfo represents metadata and embed info for any supported video platform
type VideoInfo struct {
	Platform     string `json:"platform"`
	VideoID      string `json:"videoId"`
	EmbedURL     string `json:"embedUrl"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	ThumbnailURL string `json:"thumbnailUrl"`
	MediaKind    string `json:"mediaKind"`
	SourceURL    string `json:"sourceUrl"`
	Seekable     bool   `json:"seekable"`
}

func finishVideoInfo(info *VideoInfo, source string) *VideoInfo {
	if info.MediaKind == "" {
		info.MediaKind = "vod"
	}
	if info.SourceURL == "" {
		info.SourceURL = source
	}
	if info.MediaKind == "live" {
		info.Seekable = false
	} else {
		info.Seekable = true
	}
	return info
}

// ExtractVideoInfo parses YouTube, Vimeo, Twitch, Dailymotion, Streamable,
// PeerTube, HLS, direct progressive, or local video identities.
func ExtractVideoInfo(input string) (*VideoInfo, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty video input")
	}
	source := input
	lower := strings.ToLower(input)

	// Local file identity
	if strings.HasPrefix(lower, "local://") || strings.HasPrefix(lower, "local:") {
		id := strings.TrimSpace(input)
		id = strings.TrimPrefix(id, "local://")
		id = strings.TrimPrefix(id, "local:")
		id = strings.TrimPrefix(id, "LOCAL://")
		id = strings.TrimPrefix(id, "LOCAL:")
		id = strings.ToLower(strings.TrimSpace(id))
		reLocal := regexp.MustCompile(`^[a-f0-9]{16,64}$`)
		if !reLocal.MatchString(id) {
			return nil, fmt.Errorf("invalid local video id")
		}
		videoID := "local:" + id
		return finishVideoInfo(&VideoInfo{
			Platform: "local",
			VideoID:  videoID,
			EmbedURL: videoID,
			Title:    "Local video",
			Author:   "Local file",
			MediaKind: "vod",
		}, source), nil
	}

	// YouTube
	if ytID, err := ExtractYouTubeID(input); err == nil && ytID != "" {
		meta, _ := FetchYouTubeMetadata(ytID)
		kind := "vod"
		if strings.Contains(lower, "/live/") || strings.Contains(lower, "live=1") {
			kind = "live"
		}
		return finishVideoInfo(&VideoInfo{
			Platform:     "youtube",
			VideoID:      ytID,
			EmbedURL:     fmt.Sprintf("https://www.youtube.com/embed/%s?enablejsapi=1&autoplay=1", ytID),
			Title:        meta.Title,
			Author:       meta.AuthorName,
			ThumbnailURL: meta.ThumbnailURL,
			MediaKind:    kind,
		}, source), nil
	}

	// Vimeo
	reVimeo := regexp.MustCompile(`vimeo\.com\/(?:video\/)?([0-9]+)`)
	if matches := reVimeo.FindStringSubmatch(input); len(matches) > 1 {
		vimeoID := matches[1]
		return finishVideoInfo(&VideoInfo{
			Platform:  "vimeo",
			VideoID:   vimeoID,
			EmbedURL:  fmt.Sprintf("https://player.vimeo.com/video/%s?autoplay=1", vimeoID),
			Title:     "Vimeo Video (" + vimeoID + ")",
			Author:    "Vimeo",
			MediaKind: "vod",
		}, source), nil
	}

	// Twitch VOD
	reTwitchVOD := regexp.MustCompile(`twitch\.tv\/videos\/([0-9]+)`)
	if matches := reTwitchVOD.FindStringSubmatch(input); len(matches) > 1 {
		twitchID := matches[1]
		return finishVideoInfo(&VideoInfo{
			Platform:  "twitch",
			VideoID:   "vod:" + twitchID,
			EmbedURL:  fmt.Sprintf("https://player.twitch.tv/?video=%s&parent=HOSTNAME", twitchID),
			Title:     "Twitch Video (" + twitchID + ")",
			Author:    "Twitch",
			MediaKind: "vod",
		}, source), nil
	}

	// Twitch live channel
	reTwitchChan := regexp.MustCompile(`(?:www\.)?twitch\.tv\/([a-zA-Z0-9_]{3,25})\/?(?:\?.*)?$`)
	if matches := reTwitchChan.FindStringSubmatch(input); len(matches) > 1 {
		ch := strings.ToLower(matches[1])
		reserved := map[string]bool{"videos": true, "directory": true, "downloads": true, "jobs": true, "p": true, "settings": true}
		if !reserved[ch] {
			return finishVideoInfo(&VideoInfo{
				Platform:  "twitch",
				VideoID:   "channel:" + ch,
				EmbedURL:  fmt.Sprintf("https://player.twitch.tv/?channel=%s&parent=HOSTNAME", ch),
				Title:     "Twitch Live: " + ch,
				Author:    "Twitch",
				MediaKind: "live",
			}, source), nil
		}
	}

	// Dailymotion
	reDM := regexp.MustCompile(`(?:dailymotion\.com\/(?:video|embed\/video)\/|dai\.ly\/)([a-zA-Z0-9]+)`)
	if matches := reDM.FindStringSubmatch(input); len(matches) > 1 {
		id := matches[1]
		return finishVideoInfo(&VideoInfo{
			Platform:  "dailymotion",
			VideoID:   id,
			EmbedURL:  fmt.Sprintf("https://www.dailymotion.com/embed/video/%s", id),
			Title:     "Dailymotion (" + id + ")",
			Author:    "Dailymotion",
			MediaKind: "vod",
		}, source), nil
	}

	// Streamable
	reStreamable := regexp.MustCompile(`streamable\.com\/(?:e\/)?([a-zA-Z0-9]+)`)
	if matches := reStreamable.FindStringSubmatch(input); len(matches) > 1 {
		id := matches[1]
		return finishVideoInfo(&VideoInfo{
			Platform:  "streamable",
			VideoID:   id,
			EmbedURL:  fmt.Sprintf("https://streamable.com/e/%s", id),
			Title:     "Streamable (" + id + ")",
			Author:    "Streamable",
			MediaKind: "vod",
		}, source), nil
	}

	// PeerTube
	rePeerTube := regexp.MustCompile(`(?i)^https?://([^/]+)/(?:w|videos/watch|videos/embed)/([a-zA-Z0-9_-]{10,})`)
	if matches := rePeerTube.FindStringSubmatch(input); len(matches) > 2 {
		host := matches[1]
		uuid := matches[2]
		return finishVideoInfo(&VideoInfo{
			Platform:  "peertube",
			VideoID:   host + "|" + uuid,
			EmbedURL:  fmt.Sprintf("https://%s/videos/embed/%s", host, uuid),
			Title:     "PeerTube video",
			Author:    host,
			MediaKind: "vod",
		}, source), nil
	}

	// HLS
	if strings.Contains(lower, ".m3u8") {
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return nil, fmt.Errorf("HLS URL must use http(s)")
		}
		parts := strings.Split(input, "/")
		filename := parts[len(parts)-1]
		if q := strings.Index(filename, "?"); q >= 0 {
			filename = filename[:q]
		}
		if filename == "" {
			filename = "HLS Stream"
		}
		kind := "vod"
		if strings.Contains(lower, "live") {
			kind = "live"
		}
		return finishVideoInfo(&VideoInfo{
			Platform:  "hls",
			VideoID:   input,
			EmbedURL:  input,
			Title:     filename,
			Author:    "HLS",
			MediaKind: kind,
		}, source), nil
	}

	// Direct progressive (explicit extensions only — no catch-all http)
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".ogg") ||
		strings.Contains(lower, ".mp4?") || strings.Contains(lower, ".webm?") || strings.Contains(lower, ".ogg?") {
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return nil, fmt.Errorf("direct video URL must use http(s)")
		}
		parts := strings.Split(input, "/")
		filename := parts[len(parts)-1]
		if q := strings.Index(filename, "?"); q >= 0 {
			filename = filename[:q]
		}
		if filename == "" {
			filename = "Direct Video"
		}
		return finishVideoInfo(&VideoInfo{
			Platform:  "direct",
			VideoID:   input,
			EmbedURL:  input,
			Title:     filename,
			Author:    "Direct Video",
			MediaKind: "vod",
		}, source), nil
	}

	return nil, fmt.Errorf("unrecognized video URL: %s", input)
}

// YouTubeMetadata represents oEmbed video response
type YouTubeMetadata struct {
	Title        string `json:"title"`
	AuthorName   string `json:"author_name"`
	ThumbnailURL string `json:"thumbnail_url"`
}

// FetchYouTubeMetadata fetches public metadata via YouTube's oEmbed endpoint
func FetchYouTubeMetadata(videoID string) (*YouTubeMetadata, error) {
	targetURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", url.QueryEscape(videoID))
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(targetURL)
	if err != nil {
		return &YouTubeMetadata{
			Title:        "YouTube Video (" + videoID + ")",
			AuthorName:   "YouTube",
			ThumbnailURL: fmt.Sprintf("https://img.youtube.com/vi/%s/hqdefault.jpg", videoID),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &YouTubeMetadata{
			Title:        "YouTube Video (" + videoID + ")",
			AuthorName:   "YouTube",
			ThumbnailURL: fmt.Sprintf("https://img.youtube.com/vi/%s/hqdefault.jpg", videoID),
		}, nil
	}

	var meta YouTubeMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return &YouTubeMetadata{
			Title:        "YouTube Video (" + videoID + ")",
			AuthorName:   "YouTube",
			ThumbnailURL: fmt.Sprintf("https://img.youtube.com/vi/%s/hqdefault.jpg", videoID),
		}, nil
	}

	if meta.ThumbnailURL == "" {
		meta.ThumbnailURL = fmt.Sprintf("https://img.youtube.com/vi/%s/hqdefault.jpg", videoID)
	}

	return &meta, nil
}
