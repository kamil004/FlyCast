package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type VideoMeta struct {
	Title        string `json:"title"`
	Year         int    `json:"year"`
	Overview     string `json:"overview"`
	IMDBID       string `json:"imdb_id"`
	TMDBID       string `json:"tmdb_id"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	Source       string `json:"source"`
}

type Provider interface {
	Search(query string) (*VideoMeta, error)
}

var (
	errEmptyQuery   = errors.New("empty_query")
	errNoResults    = errors.New("no_results")
	browserUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	httpClient      = &http.Client{Timeout: 12 * time.Second}
	reHTMLTitle     = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	reTitleYear     = regexp.MustCompile(`^(.*?)\s*\((\d{4})\)`)
	reFilmwebPlot   = regexp.MustCompile(`(?s)<span[^>]*itemprop="description"[^>]*>(.*?)</span>`)
	reFilmwebPoster = regexp.MustCompile(`<meta property="og:image" content="([^"]*)"`)
	reLDDesc        = regexp.MustCompile(`"description"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	reOGDesc        = regexp.MustCompile(`<meta property="og:description" content="([^"]*)"`)
	reReleaseTags   = regexp.MustCompile(`(?i)\b(2160p|1080p|720p|480p|4k|uhd|bluray|blu-ray|brrip|bdrip|webrip|web-dl|webdl|hdrip|dvdrip|hdtv|x264|x265|h264|h265|hevc|aac|ac3|eac3|dts|ddp?5[.\s]?1|5[.\s]?1|7[.\s]?1|remux|proper|repack|multi|dubbed|pl)\b`)
)

// Search fetches movie metadata from the requested source. Supported sources:
// "imdb" (default) and "filmweb"; "tmdb" is kept for compatibility.
func Search(source, query string) (*VideoMeta, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "filmweb":
		return filmwebSearch(query)
	case "tmdb":
		return NewTMDB("").Search(query)
	default:
		return imdbSearch(query)
	}
}

// get fetches a URL with browser-like headers (the sources reject the default
// Go user agent).
func get(rawurl string) (string, error) {
	body, _, err := fetch(rawurl, 4<<20)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// FetchImage downloads a poster/backdrop image (up to 12 MB).
func FetchImage(rawurl string) ([]byte, error) {
	body, ct, err := fetch(rawurl, 12<<20)
	if err != nil {
		return nil, err
	}
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("not_an_image: %s", ct)
	}
	return body, nil
}

func fetch(rawurl string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, rawurl, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "application/json, text/html, application/xhtml+xml, image/*, */*")
	req.Header.Set("Accept-Language", "pl-PL,pl;q=0.9,en;q=0.8")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("http_%d", resp.StatusCode)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// ------------------------------- IMDb --------------------------------------

// imdbSearch uses IMDb's public suggestion endpoint (no API key). It provides
// the title, year, IMDb id and poster; the plot is fetched best-effort from the
// title page and, when that is blocked, falls back to Filmweb's Polish synopsis.
// CleanQuery turns a scene-style filename into a friendlier search phrase
// (dots/underscores to spaces, common release tags removed).
func CleanQuery(q string) string {
	s := strings.ReplaceAll(q, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")
	s = reReleaseTags.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

type imdbEntry struct {
	ID    string `json:"id"`
	Title string `json:"l"`
	Q     string `json:"q"`
	QID   string `json:"qid"`
	Year  int    `json:"y"`
	Image struct {
		URL string `json:"imageUrl"`
	} `json:"i"`
}

// imdbSuggest queries IMDb's public suggestion endpoint (no API key).
func imdbSuggest(q string) ([]imdbEntry, error) {
	u := fmt.Sprintf("https://v3.sg.media-imdb.com/suggestion/%s/%s.json", firstLetter(q), url.PathEscape(q))
	body, err := get(u)
	if err != nil {
		return nil, err
	}
	var r struct {
		D []imdbEntry `json:"d"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return nil, err
	}
	return r.D, nil
}

// imdbMetaFromEntry builds metadata for one suggestion, filling the plot from
// the title page and falling back to Filmweb's synopsis when IMDb is blocked.
func imdbMetaFromEntry(e imdbEntry) *VideoMeta {
	m := &VideoMeta{
		Source:     "imdb",
		Title:      e.Title,
		Year:       e.Year,
		IMDBID:     e.ID,
		PosterPath: e.Image.URL,
	}
	if ov := imdbPlot(e.ID); ov != "" {
		m.Overview = ov
	} else if fm, err := filmwebSearch(e.Title); err == nil && fm.Overview != "" {
		if filmwebYearMatches(e.Year, fm.Year) {
			m.Overview = fm.Overview
		}
	}
	return m
}

func imdbSearch(query string) (*VideoMeta, error) {
	q := strings.TrimSpace(CleanQuery(query))
	if q == "" {
		return nil, errEmptyQuery
	}
	entries, err := imdbSuggest(q)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errNoResults
	}
	// prefer an actual movie, then any entry carrying an IMDb id
	pick := -1
	for i, e := range entries {
		if !strings.HasPrefix(e.ID, "tt") {
			continue
		}
		if e.QID == "movie" || e.Q == "feature" {
			pick = i
			break
		}
		if pick == -1 {
			pick = i
		}
	}
	if pick == -1 {
		return nil, errNoResults
	}
	return imdbMetaFromEntry(entries[pick]), nil
}

// ByIMDB resolves metadata for an explicit IMDb id (tt...), used when the
// automatic match is wrong and the user pins the correct title.
func ByIMDB(imdbID string) (*VideoMeta, error) {
	id := strings.TrimSpace(imdbID)
	if !strings.HasPrefix(id, "tt") {
		return nil, errEmptyQuery
	}
	entries, err := imdbSuggest(id)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.ID == id {
			return imdbMetaFromEntry(e), nil
		}
	}
	if len(entries) > 0 {
		return imdbMetaFromEntry(entries[0]), nil
	}
	return nil, errNoResults
}

// imdbPlot scrapes the title page's structured description. IMDb frequently
// answers with a bot challenge; any failure simply yields an empty string.
func imdbPlot(id string) string {
	body, err := get("https://www.imdb.com/title/" + id + "/")
	if err != nil {
		return ""
	}
	if m := reLDDesc.FindStringSubmatch(body); len(m) == 2 {
		return jsonUnescape(m[1])
	}
	if m := reOGDesc.FindStringSubmatch(body); len(m) == 2 {
		return html.UnescapeString(m[1])
	}
	return ""
}

func filmwebYearMatches(a, b int) bool {
	if a == 0 || b == 0 {
		return true
	}
	d := a - b
	return d >= -1 && d <= 1
}

// ------------------------------ Filmweb ------------------------------------

// filmwebSearch uses Filmweb's search API and the film page's structured data
// to return the Polish title, year and full plot.
func filmwebSearch(query string) (*VideoMeta, error) {
	q := strings.TrimSpace(CleanQuery(query))
	if q == "" {
		return nil, errEmptyQuery
	}
	body, err := get("https://www.filmweb.pl/api/v1/search?q=" + url.QueryEscape(q) + "&type=film")
	if err != nil {
		return nil, err
	}
	var r struct {
		SearchHits []struct {
			ID           int    `json:"id"`
			Type         string `json:"type"`
			MatchedTitle string `json:"matchedTitle"`
		} `json:"searchHits"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return nil, err
	}
	id := 0
	title := ""
	for _, h := range r.SearchHits {
		if h.Type == "film" && h.ID != 0 {
			id, title = h.ID, h.MatchedTitle
			break
		}
	}
	if id == 0 {
		return nil, errNoResults
	}

	m := &VideoMeta{Source: "filmweb", Title: title}
	// The slug is not validated by Filmweb: any "x-<n>-<id>" resolves and
	// redirects to the canonical page.
	page, err := get(fmt.Sprintf("https://www.filmweb.pl/film/x-1-%d", id))
	if err == nil {
		if t, year := parseFilmwebTitle(page); t != "" {
			m.Title = t
			m.Year = year
		}
		if p := reFilmwebPlot.FindStringSubmatch(page); len(p) == 2 {
			m.Overview = html.UnescapeString(strings.TrimSpace(stripTags(p[1])))
		}
		if p := reFilmwebPoster.FindStringSubmatch(page); len(p) == 2 {
			m.PosterPath = p[1]
		}
	}
	return m, nil
}

func parseFilmwebTitle(page string) (string, int) {
	m := reHTMLTitle.FindStringSubmatch(page)
	if len(m) != 2 {
		return "", 0
	}
	t := html.UnescapeString(strings.TrimSpace(m[1]))
	t = strings.TrimSuffix(t, " - Filmweb")
	if y := reTitleYear.FindStringSubmatch(t); len(y) == 3 {
		year := 0
		fmt.Sscan(y[2], &year)
		return strings.TrimSpace(y[1]), year
	}
	return t, 0
}

func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func firstLetter(q string) string {
	for _, r := range strings.ToLower(q) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return string(r)
		}
	}
	return "a"
}

func jsonUnescape(s string) string {
	var out string
	if err := json.Unmarshal([]byte(`"`+s+`"`), &out); err == nil {
		return out
	}
	return html.UnescapeString(s)
}

// ------------------------------- TMDb --------------------------------------

type TMDB struct {
	apiKey string
	lang   string
}

func NewTMDB(apiKey string) *TMDB {
	if apiKey == "" {
		apiKey = "0"
	}
	return &TMDB{apiKey: apiKey, lang: "en-US"}
}

func (t *TMDB) Search(query string) (*VideoMeta, error) {
	q := url.QueryEscape(cleanQuery(query))
	u := fmt.Sprintf("https://api.themoviedb.org/3/search/multi?api_key=%s&language=%s&query=%s&page=1&include_adult=false", t.apiKey, t.lang, q)
	body, err := get(u)
	if err != nil {
		return nil, err
	}
	var r struct {
		Results []struct {
			MediaType    string `json:"media_type"`
			Title        string `json:"title"`
			Name         string `json:"name"`
			ReleaseDate  string `json:"release_date"`
			FirstAirDate string `json:"first_air_date"`
			Overview     string `json:"overview"`
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
			ID           int    `json:"id"`
			IMDBID       string `json:"imdb_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		return nil, err
	}
	if len(r.Results) == 0 {
		return nil, errNoResults
	}
	res := r.Results[0]
	m := &VideoMeta{Source: "tmdb"}
	if res.Title != "" {
		m.Title = res.Title
	} else {
		m.Title = res.Name
	}
	if res.ReleaseDate != "" && len(res.ReleaseDate) >= 4 {
		fmt.Sscan(res.ReleaseDate[:4], &m.Year)
	} else if res.FirstAirDate != "" && len(res.FirstAirDate) >= 4 {
		fmt.Sscan(res.FirstAirDate[:4], &m.Year)
	}
	m.Overview = res.Overview
	m.TMDBID = fmt.Sprint(res.ID)
	if res.IMDBID != "" {
		m.IMDBID = res.IMDBID
	}
	if res.PosterPath != "" {
		m.PosterPath = "https://image.tmdb.org/t/p/w342" + res.PosterPath
	}
	if res.BackdropPath != "" {
		m.BackdropPath = "https://image.tmdb.org/t/p/w780" + res.BackdropPath
	}
	return m, nil
}

func cleanQuery(s string) string {
	return strings.TrimSpace(s)
}
