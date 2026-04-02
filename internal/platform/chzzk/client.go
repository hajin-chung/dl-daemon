package chzzk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
)

type Client struct {
	Cookie string
	client *http.Client
}

func NewClient(aut string, ses string) *Client {
	cookie := strings.TrimSpace(fmt.Sprintf("NID_AUT=%s; NID_SES=%s", aut, ses))
	return &Client{
		Cookie: cookie,
		client: http.DefaultClient,
	}
}

func (c *Client) Auth(aut string, ses string) {
	c.Cookie = strings.TrimSpace(fmt.Sprintf("NID_AUT=%s; NID_SES=%s", aut, ses))
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

func (c *Client) GetWithRetry(url string, retry int) (*http.Response, error) {
	var err error
	for retry > 0 {
		res, getErr := c.Get(url)
		if getErr == nil {
			return res, nil
		}
		err = getErr
		retry--
	}
	return nil, err
}

type VideoType string

const (
	HLS  VideoType = "HLS"
	DASH VideoType = "DASH"
)

type VideoURL struct {
	Type VideoType
	URL  string
}

type videoResponse struct {
	Code    int          `json:"code"`
	Content videoContent `json:"content"`
}

type videoContent struct {
	Adult        bool   `json:"adult"`
	InKey        string `json:"inKey"`
	PlaybackJSON string `json:"liveRewindPlaybackJson"`
	VideoID      string `json:"videoId"`
}

type videoPlayback struct {
	Media []videoPlaybackMedia `json:"media"`
}

type videoPlaybackMedia struct {
	Path string `json:"path"`
}

func (c *Client) GetVideoURL(videoNo int) (*VideoURL, error) {
	url := fmt.Sprintf("https://api.chzzk.naver.com/service/v3/videos/%d", videoNo)
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	video := videoResponse{}
	if err := json.Unmarshal(body, &video); err != nil {
		return nil, err
	}

	if video.Content.InKey == "" {
		playback := videoPlayback{}
		if err := json.Unmarshal([]byte(video.Content.PlaybackJSON), &playback); err != nil {
			return nil, err
		}
		if len(playback.Media) == 0 {
			return nil, errors.New("no playback media found")
		}
		return &VideoURL{Type: HLS, URL: playback.Media[0].Path}, nil
	}

	dashURL := fmt.Sprintf("https://apis.naver.com/neonplayer/vodplay/v1/playback/%s?key=%s", video.Content.VideoID, video.Content.InKey)
	req, err := http.NewRequest("GET", dashURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/xml")
	req.Header.Add("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1")

	res, err = c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	doc, err := xmlquery.Parse(res.Body)
	if err != nil {
		return nil, err
	}

	representations := xmlquery.Find(doc, "//Representation[@mimeType='video/mp4']")
	maxBandwidth := 0
	for _, node := range representations {
		for _, attr := range node.Attr {
			if attr.Name.Local != "bandwidth" {
				continue
			}
			bandwidth, err := strconv.Atoi(attr.Value)
			if err != nil {
				continue
			}
			if bandwidth > maxBandwidth {
				maxBandwidth = bandwidth
			}
		}
	}
	if maxBandwidth == 0 {
		return nil, errors.New("no mp4 representation found")
	}

	query := fmt.Sprintf("//Representation[@mimeType='video/mp4'][@bandwidth='%d']/BaseURL", maxBandwidth)
	node := xmlquery.FindOne(doc, query)
	if node == nil {
		return nil, errors.New("baseurl not found")
	}

	return &VideoURL{Type: DASH, URL: node.InnerText()}, nil
}

type UserStatus struct {
	Code    int               `json:"code"`
	Content UserStatusContent `json:"content"`
}

type UserStatusContent struct {
	HasProfile bool    `json:"HasProfile"`
	NickName   *string `json:"nickname,omitempty"`
}

func (c *Client) GetUserStatus() (*UserStatus, error) {
	res, err := c.Get("https://comm-api.game.naver.com/nng_main/v1/user/getUserStatus")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	status := UserStatus{}
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

type VideoData struct {
	VideoNo  int    `json:"videoNo"`
	Duration int    `json:"duration"`
	Title    string `json:"videoTitle"`
	Date     string `json:"publishDate"`
}

type videoDataResponse struct {
	Code    int       `json:"code"`
	Content VideoData `json:"content"`
}

func (c *Client) GetVideoInfo(videoNo int) (*VideoData, error) {
	url := fmt.Sprintf("https://api.chzzk.naver.com/service/v2/videos/%d", videoNo)
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	data := videoDataResponse{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data.Content, nil
}

type videoListContent struct {
	Data       []VideoData `json:"data"`
	TotalPages int         `json:"totalPages"`
}

type videoListResponse struct {
	Code    int              `json:"code"`
	Content videoListContent `json:"content"`
}

func (c *Client) GetVideoList(channelID string) ([]VideoData, error) {
	totalPages := 1
	videos := []VideoData{}
	for page := 0; page < totalPages; page++ {
		url := fmt.Sprintf("https://api.chzzk.naver.com/service/v1/channels/%s/videos?page=%d", channelID, page)
		res, err := c.Get(url)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, err
		}

		data := videoListResponse{}
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, err
		}
		videos = append(videos, data.Content.Data...)
		totalPages = data.Content.TotalPages
	}

	for i, j := 0, len(videos)-1; i < j; i, j = i+1, j-1 {
		videos[i], videos[j] = videos[j], videos[i]
	}
	return videos, nil
}
