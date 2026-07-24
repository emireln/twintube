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

// GenerateRoomCode generates a user-friendly 6-character room code (e.g. "a7b9x2")
func GenerateRoomCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	randBytes := make([]byte, 6)
	rand.Read(randBytes)
	for i := range b {
		b[i] = charset[int(randBytes[i])%len(charset)]
	}
	return string(b)
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
