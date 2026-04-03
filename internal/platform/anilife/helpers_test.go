package anilife

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func TestOutputName(t *testing.T) {
	episode := &Episode{Num: "7", Title: `bad:/title?`}
	got := outputName(`Anime: Name`, episode)
	want := "Anime_ Name/007.bad__title_.mp4"
	if got != want {
		t.Fatalf("outputName() = %q, want %q", got, want)
	}
}

func TestParsePlayerURLs(t *testing.T) {
	html := `some text "https://anilife.live/h/live?foo=1" other "https://anilife.live/h/live?bar=2"`
	urls := parsePlayerURLs(html)
	if len(urls) != 2 {
		t.Fatalf("len(urls) = %d, want 2", len(urls))
	}
	if urls[0] != "https://anilife.live/h/live?foo=1" {
		t.Fatalf("urls[0] = %q", urls[0])
	}
}

func TestParseAlData(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{"vid_url_1080":"cdn.example/hls"}`))
	html := fmt.Sprintf("<script>var _aldata = '%s'</script>", payload)
	data, err := parseAlData(html)
	if err != nil {
		t.Fatalf("parseAlData() error = %v", err)
	}
	if data.VidURL1080 != "cdn.example/hls" {
		t.Fatalf("VidURL1080 = %q", data.VidURL1080)
	}
}
