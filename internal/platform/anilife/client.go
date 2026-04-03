package anilife

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL     = "https://anilife.live"
	userAgent   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36"
	playerRegex = `"(https:\/\/anilife\.live\/h\/live\?.+?)"`
	aldataRegex = `var _aldata = '(.+?)'`
)

type transport struct {
	base http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	return &Client{client: &http.Client{Transport: &transport{}}}
}

func (c *Client) GetBytesWithHeaders(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func (c *Client) GetBodyWithReferer(url string, referer string) (string, error) {
	body, err := c.GetBytesWithHeaders(url, map[string]string{"Referer": referer})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type Anime struct {
	ID    string
	Title string
	URL   string
}

type Episode struct {
	Title string
	URL   string
	Num   string
}

type alData struct {
	VidURL1080 string `json:"vid_url_1080"`
	VidURL720  string `json:"vid_url_720"`
}

type videoData struct {
	URL string `json:"url"`
}

func (c *Client) Search(query string) ([]*Anime, error) {
	query = neturl.QueryEscape(query)
	res, err := c.client.Get(fmt.Sprintf("%s/search?keyword=%s", baseURL, query))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return parseSearchResults(res.Body)
}

func parseSearchResults(body io.Reader) ([]*Anime, error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}

	animes := []*Anime{}
	doc.Find(".bsx").Each(func(i int, s *goquery.Selection) {
		url := s.Find("a").AttrOr("href", "")
		title := strings.TrimSpace(s.Find("h2[itemprop]").Text())
		parts := strings.Split(strings.TrimRight(url, "/"), "/")
		id := parts[len(parts)-1]
		animes = append(animes, &Anime{ID: id, Title: title, URL: url})
	})
	return animes, nil
}

func (c *Client) GetAnime(id string) (*Anime, []*Episode, error) {
	res, err := c.client.Get(fmt.Sprintf("%s/detail/id/%s", baseURL, id))
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, nil, err
	}

	title := strings.TrimSpace(doc.Find("h1.entry-title").First().Text())
	anime := &Anime{ID: id, Title: title, URL: res.Request.URL.String()}
	episodes := []*Episode{}
	doc.Find("div.eplister li").Each(func(i int, s *goquery.Selection) {
		episodes = append(episodes, &Episode{
			Title: strings.TrimSpace(s.Find(".epl-title").Text()),
			Num:   strings.TrimSpace(s.Find(".epl-num").Text()),
			URL:   absoluteURL(baseURL, s.Find("a").AttrOr("href", "")),
		})
	})

	return anime, episodes, nil
}

func (c *Client) GetEpisodeHLS(episode *Episode, anime *Anime) (string, error) {
	playerURLs, err := c.getPlayerURLs(episode, anime)
	if err != nil {
		return "", err
	}
	if len(playerURLs) == 0 {
		return "", fmt.Errorf("no player url found")
	}

	playerURL := playerURLs[0]
	aldata, err := c.getAlData(playerURL)
	if err != nil {
		return "", err
	}

	videoURL := ""
	switch {
	case aldata.VidURL1080 != "":
		videoURL = "https://" + strings.TrimPrefix(aldata.VidURL1080, "https://")
	case aldata.VidURL720 != "":
		videoURL = "https://" + strings.TrimPrefix(aldata.VidURL720, "https://")
	default:
		return "", fmt.Errorf("hls url not found in aldata")
	}

	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", playerURL)
	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var videos []videoData
	if err := json.Unmarshal(body, &videos); err != nil {
		return "", err
	}
	if len(videos) == 0 || videos[0].URL == "" {
		return "", fmt.Errorf("empty video data response")
	}
	return videos[0].URL, nil
}

func (c *Client) getPlayerURLs(episode *Episode, anime *Anime) ([]string, error) {
	req, err := http.NewRequest("GET", episode.URL, nil)
	if err != nil {
		return nil, err
	}
	if anime != nil && anime.URL != "" {
		req.Header.Set("Referer", anime.URL)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return parsePlayerURLs(string(body)), nil
}

func parsePlayerURLs(html string) []string {
	re := regexp.MustCompile(playerRegex)
	matches := re.FindAllStringSubmatch(html, -1)
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			urls = append(urls, m[1])
		}
	}
	return urls
}

func (c *Client) getAlData(playerURL string) (*alData, error) {
	res, err := c.client.Get(playerURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return parseAlData(string(body))
}

func parseAlData(html string) (*alData, error) {
	re := regexp.MustCompile(aldataRegex)
	match := re.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("no aldata found")
	}
	decoded, err := base64.StdEncoding.DecodeString(match[1])
	if err != nil {
		return nil, err
	}
	data := &alData{}
	if err := json.Unmarshal(decoded, data); err != nil {
		return nil, err
	}
	return data, nil
}

func absoluteURL(base string, path string) string {
	if path == "" {
		return ""
	}
	u, err := neturl.Parse(path)
	if err == nil && u.IsAbs() {
		return u.String()
	}
	baseParsed, err := neturl.Parse(base)
	if err != nil {
		return path
	}
	partParsed, err := neturl.Parse(path)
	if err != nil {
		return path
	}
	return baseParsed.ResolveReference(partParsed).String()
}
