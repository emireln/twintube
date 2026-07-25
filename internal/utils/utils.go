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
}

// ExtractVideoInfo parses YouTube, Vimeo, Twitch, or Direct video URLs
func ExtractVideoInfo(input string) (*VideoInfo, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty video input")
	}

	// 1. YouTube check
	if ytID, err := ExtractYouTubeID(input); err == nil && ytID != "" {
		meta, _ := FetchYouTubeMetadata(ytID)
		return &VideoInfo{
			Platform:     "youtube",
			VideoID:      ytID,
			EmbedURL:     fmt.Sprintf("https://www.youtube.com/embed/%s?enablejsapi=1&autoplay=1", ytID),
			Title:        meta.Title,
			Author:       meta.AuthorName,
			ThumbnailURL: meta.ThumbnailURL,
		}, nil
	}

	// 2. Vimeo (vimeo.com/12345678)
	reVimeo := regexp.MustCompile(`vimeo\.com\/(?:video\/)?([0-9]+)`)
	if matches := reVimeo.FindStringSubmatch(input); len(matches) > 1 {
		vimeoID := matches[1]
		return &VideoInfo{
			Platform:     "vimeo",
			VideoID:      vimeoID,
			EmbedURL:     fmt.Sprintf("https://player.vimeo.com/video/%s?autoplay=1", vimeoID),
			Title:        "Vimeo Video (" + vimeoID + ")",
			Author:       "Vimeo",
			ThumbnailURL: "",
		}, nil
	}

	// 3. Twitch (twitch.tv/videos/12345)
	reTwitch := regexp.MustCompile(`twitch\.tv\/videos\/([0-9]+)`)
	if matches := reTwitch.FindStringSubmatch(input); len(matches) > 1 {
		twitchID := matches[1]
		return &VideoInfo{
			Platform:     "twitch",
			VideoID:      twitchID,
			EmbedURL:     fmt.Sprintf("https://player.twitch.tv/?video=%s&parent=localhost", twitchID),
			Title:        "Twitch Video (" + twitchID + ")",
			Author:       "Twitch",
			ThumbnailURL: "",
		}, nil
	}

	// 4. Local file identity (client-only bytes; server stores hash for sync)
	// Formats: local:<hex> | local://<hex>
	lower := strings.ToLower(input)
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
		return &VideoInfo{
			Platform:     "local",
			VideoID:      videoID,
			EmbedURL:     videoID,
			Title:        "Local video",
			Author:       "Local file",
			ThumbnailURL: "",
		}, nil
	}

	// 5. Direct Video URL (.mp4, .webm, .ogg, .m3u8)
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".m3u8") || strings.HasPrefix(lower, "http") {
		parts := strings.Split(input, "/")
		filename := parts[len(parts)-1]
		if filename == "" { filename = "Direct Video Stream" }
		return &VideoInfo{
			Platform:     "direct",
			VideoID:      input,
			EmbedURL:     input,
			Title:        filename,
			Author:       "Direct Video",
			ThumbnailURL: "",
		}, nil
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
