package utils

import "testing"

func TestExtractVideoInfoProviders(t *testing.T) {
	cases := []struct {
		in       string
		platform string
		kind     string
		seekable bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "youtube", "vod", true},
		{"https://vimeo.com/123456789", "vimeo", "vod", true},
		{"https://www.twitch.tv/videos/987654321", "twitch", "vod", true},
		{"https://www.twitch.tv/somechannel", "twitch", "live", false},
		{"https://www.dailymotion.com/video/x7abcde", "dailymotion", "vod", true},
		{"https://streamable.com/abcd12", "streamable", "vod", true},
		{"https://peertube.example/w/abcdefghijklmnop", "peertube", "vod", true},
		{"https://cdn.example.com/stream.m3u8", "hls", "vod", true},
		{"https://cdn.example.com/clip.mp4", "direct", "vod", true},
		{"local:0123456789abcdef0123456789abcdef", "local", "vod", true},
	}

	for _, tc := range cases {
		info, err := ExtractVideoInfo(tc.in)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.in, err)
		}
		if info.Platform != tc.platform {
			t.Fatalf("%s: platform=%s want %s", tc.in, info.Platform, tc.platform)
		}
		if info.MediaKind != tc.kind {
			t.Fatalf("%s: mediaKind=%s want %s", tc.in, info.MediaKind, tc.kind)
		}
		if info.Seekable != tc.seekable {
			t.Fatalf("%s: seekable=%v want %v", tc.in, info.Seekable, tc.seekable)
		}
	}
}

func TestExtractVideoInfoRejectsBareHTTP(t *testing.T) {
	if _, err := ExtractVideoInfo("https://example.com/page"); err == nil {
		t.Fatal("expected bare http URL to be rejected")
	}
}
