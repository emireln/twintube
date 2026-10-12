package room

import (
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewPlaybackReportAcceptsEveryPlatformTheAppProduces(t *testing.T) {
	localID := "local:" + strings.Repeat("ab", 32)
	cases := []struct {
		name  string
		video string
		meta  MediaMeta
	}{
		{"youtube id", "dQw4w9WgXcQ", MediaMeta{Platform: "youtube", MediaKind: "vod"}},
		{"youtube with bare-id source", "dQw4w9WgXcQ", MediaMeta{Platform: "youtube", SourceURL: "dQw4w9WgXcQ"}},
		{"youtube with URL source", "dQw4w9WgXcQ", MediaMeta{Platform: "youtube", SourceURL: "https://youtu.be/dQw4w9WgXcQ"}},
		{"local file", localID, MediaMeta{Platform: "local", SourceURL: localID}},
		{"direct mp4", "https://cdn.example.com/a.mp4", MediaMeta{Platform: "direct", SourceURL: "https://cdn.example.com/a.mp4"}},
		{"hls live", "https://cdn.example.com/live/index.m3u8", MediaMeta{Platform: "hls", MediaKind: "live"}},
		{"twitch vod, scheme-less source", "vod:12345", MediaMeta{Platform: "twitch", SourceURL: "twitch.tv/videos/12345"}},
		{"twitch channel", "channel:somestreamer", MediaMeta{Platform: "twitch", MediaKind: "live"}},
		{"vimeo, scheme-less source", "123456789", MediaMeta{Platform: "vimeo", SourceURL: "vimeo.com/123456789"}},
		{"dailymotion", "x8abc12", MediaMeta{Platform: "dailymotion"}},
		{"streamable", "abc123", MediaMeta{Platform: "streamable"}},
		{"peertube", "videos.example.org|abcdefghij12", MediaMeta{Platform: "peertube"}},
		{"cleared player", "", MediaMeta{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewPlaybackReport(tc.video, "PLAYING", 12.5, "", tc.meta); err != nil {
				t.Fatalf("legitimate report rejected: %v", err)
			}
		})
	}
}

func TestNewPlaybackReportRejectsUnsafeReports(t *testing.T) {
	cases := []struct {
		name   string
		video  string
		status string
		meta   MediaMeta
	}{
		{"unknown platform (iframe injection)", "x", "PLAYING", MediaMeta{Platform: "foo"}},
		{"script platform", "x", "PLAYING", MediaMeta{Platform: "javascript:alert(1)"}},
		{"javascript source", "dQw4w9WgXcQ", "PLAYING", MediaMeta{Platform: "youtube", SourceURL: "javascript:alert(document.cookie)"}},
		{"data source", "dQw4w9WgXcQ", "PLAYING", MediaMeta{Platform: "youtube", SourceURL: "data:text/html,<script>alert(1)</script>"}},
		{"javascript video id on direct", "javascript:alert(1)", "PLAYING", MediaMeta{Platform: "direct"}},
		{"youtube id wrong length", "x", "PAUSED", MediaMeta{Platform: "youtube"}},
		{"file url source", "https://cdn.example.com/a.mp4", "PLAYING", MediaMeta{Platform: "direct", SourceURL: "file:///etc/passwd"}},
		{"local platform with web source", localIDForTest(), "PLAYING", MediaMeta{Platform: "local", SourceURL: "https://evil.example/x"}},
		{"unknown status", "dQw4w9WgXcQ", "EXPLODE", MediaMeta{Platform: "youtube"}},
		{"empty status", "dQw4w9WgXcQ", "", MediaMeta{Platform: "youtube"}},
		{"unknown media kind", "dQw4w9WgXcQ", "PLAYING", MediaMeta{Platform: "youtube", MediaKind: "weird"}},
		{"control character in video id", "dQw4w9WgX\ncQ", "PLAYING", MediaMeta{Platform: "youtube"}},
		{"oversized video id", strings.Repeat("a", maxPlaybackVideoIDBytes+1), "PLAYING", MediaMeta{Platform: "direct"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPlaybackReport(tc.video, tc.status, 1, "", tc.meta)
			if !errors.Is(err, ErrInvalidPlayback) {
				t.Fatalf("expected ErrInvalidPlayback, got %v", err)
			}
		})
	}
}

func TestNewPlaybackReportClampsTimeAndTitle(t *testing.T) {
	report, err := NewPlaybackReport("dQw4w9WgXcQ", "PLAYING", -5, "", MediaMeta{Platform: "youtube"})
	if err != nil || report.CurrentTime != 0 {
		t.Fatalf("negative time should clamp to 0, got %v (err %v)", report.CurrentTime, err)
	}

	report, err = NewPlaybackReport("dQw4w9WgXcQ", "PLAYING", 1e12, "", MediaMeta{Platform: "youtube"})
	if err != nil || report.CurrentTime != maxPlaybackSeconds {
		t.Fatalf("huge time should clamp to the maximum, got %v (err %v)", report.CurrentTime, err)
	}

	if _, err := NewPlaybackReport("dQw4w9WgXcQ", "PLAYING", math.NaN(), "", MediaMeta{Platform: "youtube"}); !errors.Is(err, ErrInvalidPlayback) {
		t.Fatalf("NaN time must be rejected, got %v", err)
	}

	report, err = NewPlaybackReport("dQw4w9WgXcQ", "PAUSED", 3, "  "+strings.Repeat("é", 300)+"\x07 ", MediaMeta{Platform: "youtube"})
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString(report.Title) != maxPlaybackTitleRunes || !utf8.ValidString(report.Title) {
		t.Fatalf("title should be %d valid characters, got %d", maxPlaybackTitleRunes, utf8.RuneCountInString(report.Title))
	}
	if strings.ContainsRune(report.Title, '\x07') {
		t.Fatal("control characters must be stripped from titles")
	}
}

func TestNewPlaybackReportInfersPlatformWhenMissing(t *testing.T) {
	report, err := NewPlaybackReport("https://cdn.example.com/a.mp4", "PLAYING", 0, "", MediaMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Meta.Platform != "direct" {
		t.Fatalf("expected inferred platform direct, got %q", report.Meta.Platform)
	}
}

func localIDForTest() string {
	return "local:" + strings.Repeat("cd", 32)
}
