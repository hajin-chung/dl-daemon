package chzzk_live

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	variants, err := parseMasterPlaylist(selected.Path, masterBody)
	if err != nil {
		return "", err
	}
	if len(variants) == 0 {
		return "", fmt.Errorf("no variants in HLS master playlist")
	}
	best := variants[0]
	for _, candidate := range variants[1:] {
		if candidate.Height > best.Height || (candidate.Height == best.Height && candidate.Bandwidth > best.Bandwidth) {
			best = candidate
		}
	}
	return best.URL, nil
}
