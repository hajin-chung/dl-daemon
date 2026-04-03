package chzzk_live

import (
	"fmt"
	neturl "net/url"
	"path/filepath"
	"strings"
)

type MasterVariant struct {
	URL        string
	Bandwidth  int
	Width      int
	Height     int
}

type MediaPlaylist struct {
	InitURL   string
	Segments  []string
	Ended     bool
}

func parseMasterPlaylist(base string, body string) ([]MasterVariant, error) {
	lines := strings.Split(body, "\n")
	variants := []MasterVariant{}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		if i+1 >= len(lines) {
			break
		}
		attrs := strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
		url := resolveURL(base, strings.TrimSpace(lines[i+1]))
		width, height := parseResolution(attrs)
		variants = append(variants, MasterVariant{
			URL:       url,
			Bandwidth: parseIntKVAttr(attrs, "BANDWIDTH"),
			Width:     width,
			Height:    height,
		})
		i++
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("no master variants found")
	}
	return variants, nil
}

func parseMediaPlaylist(base string, body string) (*MediaPlaylist, error) {
	lines := strings.Split(body, "\n")
	playlist := &MediaPlaylist{Segments: []string{}}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "#EXT-X-ENDLIST" {
			playlist.Ended = true
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MAP:") {
			uri := parseQuotedAttr(strings.TrimPrefix(line, "#EXT-X-MAP:"), "URI")
			playlist.InitURL = resolveURL(base, uri)
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			if i+1 >= len(lines) {
				return nil, fmt.Errorf("missing segment url after EXTINF")
			}
			segment := strings.TrimSpace(lines[i+1])
			playlist.Segments = append(playlist.Segments, resolveURL(base, segment))
			i++
		}
	}
	if playlist.InitURL == "" {
		return nil, fmt.Errorf("no init segment found")
	}
	return playlist, nil
}

func parseResolution(attrs string) (int, int) {
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "RESOLUTION=") {
			continue
		}
		value := strings.TrimPrefix(part, "RESOLUTION=")
		pieces := strings.SplitN(value, "x", 2)
		if len(pieces) != 2 {
			return -1, -1
		}
		return parseInt(pieces[0]), parseInt(pieces[1])
	}
	return -1, -1
}

func parseQuotedAttr(attrs string, key string) string {
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		prefix := key + "=\""
		if !strings.HasPrefix(part, prefix) {
			continue
		}
		return strings.TrimSuffix(strings.TrimPrefix(part, prefix), "\"")
	}
	return ""
}

func parseInt(raw string) int {
	var n int
	fmt.Sscanf(raw, "%d", &n)
	return n
}

func parseIntKVAttr(attrs string, key string) int {
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, key+"=") {
			continue
		}
		return parseInt(strings.TrimPrefix(part, key+"="))
	}
	return -1
}

func resolveURL(base string, ref string) string {
	baseURL, err := neturl.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := neturl.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}

func localSegmentPath(workDir string, index int, ext string) string {
	if ext == "" {
		ext = ".seg"
	}
	return filepath.Join(workDir, "segments", fmt.Sprintf("%08d%s", index, ext))
}
