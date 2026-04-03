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

func (d *LiveDetailContent) HLSPath() (string, error) {
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

	bestTrack := selectHighestTrack(selected.EncodingTrack)
	if bestTrack != nil {
		trackPath, err := deriveTrackPlaylistPath(selected.Path, bestTrack.EncodingTrackID)
		if err == nil {
			return trackPath, nil
		}
	}

	return selected.Path, nil
}

func selectHighestTrack(tracks []LivePlaybackEncodingTrack) *LivePlaybackEncodingTrack {
	if len(tracks) == 0 {
		return nil
	}
	best := &tracks[0]
	for i := 1; i < len(tracks); i++ {
		candidate := &tracks[i]
		if candidate.VideoHeight > best.VideoHeight {
			best = candidate
			continue
		}
		if candidate.VideoHeight == best.VideoHeight && candidate.VideoBitRate > best.VideoBitRate {
			best = candidate
		}
	}
	return best
}

func deriveTrackPlaylistPath(masterPath string, trackID string) (string, error) {
	if masterPath == "" {
		return "", fmt.Errorf("empty master path")
	}
	if trackID == "" || trackID == "audioOnly" {
		return masterPath, nil
	}
	idx := strings.LastIndex(masterPath, "/")
	if idx == -1 {
		return "", fmt.Errorf("invalid master path")
	}
	prefix := masterPath[:idx+1]
	fileAndQuery := masterPath[idx+1:]
	parts := strings.SplitN(fileAndQuery, "?", 2)
	file := parts[0]
	query := ""
	if len(parts) == 2 {
		query = "?" + parts[1]
	}
	if strings.HasSuffix(file, "_hls_playlist.m3u8") {
		file = strings.TrimSuffix(file, "_hls_playlist.m3u8") + "_hls_" + trackID + "_playlist.m3u8"
		return prefix + file + query, nil
	}
	if strings.HasSuffix(file, "_playlist.m3u8") {
		file = strings.TrimSuffix(file, "_playlist.m3u8") + "_" + trackID + "_playlist.m3u8"
		return prefix + file + query, nil
	}
	return masterPath, nil
}
