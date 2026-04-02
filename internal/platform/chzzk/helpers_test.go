package chzzk

import "testing"

func TestOutputName(t *testing.T) {
	info := &VideoData{
		VideoNo: 123,
		Title:   `bad:/title?`,
		Date:    "2025-01-02 03:04:05",
	}

	name, err := OutputName(info)
	if err != nil {
		t.Fatalf("OutputName error: %v", err)
	}

	want := "25.01.02 badtitle [123].mp4"
	if name != want {
		t.Fatalf("name = %q, want %q", name, want)
	}
}

func TestParsePlaylistHLS(t *testing.T) {
	hls := "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:1.0,\nseg-1.m4s\n#EXTINF:1.0,\nseg-2.m4s\n"
	playlist, err := parsePlaylistHLS("https://example.com/path/master.m3u8", hls)
	if err != nil {
		t.Fatalf("parsePlaylistHLS error: %v", err)
	}

	if playlist.Init != "https://example.com/path/init.mp4" {
		t.Fatalf("init = %q", playlist.Init)
	}
	if len(playlist.Segments) != 2 {
		t.Fatalf("len(segments) = %d, want 2", len(playlist.Segments))
	}
	if playlist.Segments[0] != "https://example.com/path/seg-1.m4s" {
		t.Fatalf("seg0 = %q", playlist.Segments[0])
	}
}
