package chzzk_live

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"strings"
)

type Client struct {
	Cookie string
	client *http.Client
}

func NewClient(aut string, ses string) *Client {
	cookie := strings.TrimSpace(fmt.Sprintf("NID_AUT=%s; NID_SES=%s", aut, ses))
	return &Client{Cookie: cookie, client: http.DefaultClient}
}

func (c *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1")
	if c.Cookie != "" {
		req.Header.Add("Cookie", c.Cookie)
	}
	return c.client.Do(req)
}

func (c *Client) GetBody(url string) (string, error) {
	res, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type liveStatusResponse struct {
	Code    int               `json:"code"`
	Message *string           `json:"message"`
	Content LiveStatusContent `json:"content"`
}

type LiveStatusContent struct {
	Status                string `json:"status"`
	LiveTitle             string `json:"liveTitle"`
	OpenDate              string `json:"openDate"`
	ChannelID             string `json:"channelId"`
	LivePollingStatusJSON string `json:"livePollingStatusJson"`
}

func (c *Client) GetLiveStatus(channelID string) (*LiveStatusContent, error) {
	url := fmt.Sprintf("https://api.chzzk.naver.com/polling/v3.1/channels/%s/live-status?includePlayerRecommendContent=true", channelID)
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	data := liveStatusResponse{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data.Content, nil
}

type liveDetailResponse struct {
	Code    int               `json:"code"`
	Message *string           `json:"message"`
	Content LiveDetailContent `json:"content"`
}

type LiveDetailContent struct {
	LiveID           int                `json:"liveId"`
	LiveTitle        string             `json:"liveTitle"`
	Status           string             `json:"status"`
	OpenDate         string             `json:"openDate"`
	LivePlaybackJSON string             `json:"livePlaybackJson"`
	Channel          LiveDetailChannel  `json:"channel"`
}

type LiveDetailChannel struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
}

type LivePlayback struct {
	Media []LivePlaybackMedia `json:"media"`
}

type LivePlaybackMedia struct {
	MediaID       string                     `json:"mediaId"`
	Protocol      string                     `json:"protocol"`
	Path          string                     `json:"path"`
	Latency       string                     `json:"latency,omitempty"`
	EncodingTrack []LivePlaybackEncodingTrack `json:"encodingTrack,omitempty"`
}

type LivePlaybackEncodingTrack struct {
	EncodingTrackID string `json:"encodingTrackId"`
	VideoBitRate    int    `json:"videoBitRate,omitempty"`
	VideoWidth      int    `json:"videoWidth,omitempty"`
	VideoHeight     int    `json:"videoHeight,omitempty"`
}

func (c *Client) GetLiveDetail(channelID string) (*LiveDetailContent, error) {
	url := fmt.Sprintf("https://api.chzzk.naver.com/service/v3.3/channels/%s/live-detail?cu=false&dt=22b9b&tm=true", channelID)
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	data := liveDetailResponse{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data.Content, nil
}

func ParseLivePlayback(raw string) (*LivePlayback, error) {
	playback := &LivePlayback{}
	if err := json.Unmarshal([]byte(raw), playback); err != nil {
		return nil, err
	}
	return playback, nil
}

func (d *LiveDetailContent) HLSPath(client *Client) (string, error) {
	playback, err := ParseLivePlayback(d.LivePlaybackJSON)
	if err != nil {
		return "", err
	}

	var selected *LivePlaybackMedia
	for i, media := range playback.Media {
		if media.MediaID == "HLS" && media.Path != "" {
			selected = &playback.Media[i]
			break
		}
	}
	if selected == nil {
		return "", fmt.Errorf("no HLS media path found")
	}

	masterBody, err := client.GetBody(selected.Path)
	if err != nil {
		return "", err
	}
	variantURL, err := selectHighestVariant(selected.Path, masterBody)
	if err != nil {
		return "", err
	}
	return variantURL, nil
}

func selectHighestVariant(base string, master string) (string, error) {
	lines := strings.Split(master, "\n")
	bestBandwidth := -1
	bestResolution := -1
	bestURL := ""

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		if i+1 >= len(lines) {
			break
		}
		attrs := strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
		bandwidth := parseIntAttr(attrs, "BANDWIDTH")
		resolution := parseResolutionHeight(attrs)
		candidate := strings.TrimSpace(lines[i+1])
		candidateURL := resolveURL(base, candidate)
		if resolution > bestResolution || (resolution == bestResolution && bandwidth > bestBandwidth) {
			bestResolution = resolution
			bestBandwidth = bandwidth
			bestURL = candidateURL
		}
		i++
	}

	if bestURL == "" {
		return "", fmt.Errorf("no variant playlist found in master HLS")
	}
	return bestURL, nil
}

func parseIntAttr(attrs string, key string) int {
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, key+"=") {
			continue
		}
		value := strings.TrimPrefix(part, key+"=")
		n, err := strconv.Atoi(value)
		if err == nil {
			return n
		}
	}
	return -1
}

func parseResolutionHeight(attrs string) int {
	for _, part := range strings.Split(attrs, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "RESOLUTION=") {
			continue
		}
		value := strings.TrimPrefix(part, "RESOLUTION=")
		pieces := strings.SplitN(value, "x", 2)
		if len(pieces) != 2 {
			return -1
		}
		n, err := strconv.Atoi(pieces[1])
		if err == nil {
			return n
		}
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
