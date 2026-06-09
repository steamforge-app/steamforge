package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	hltbBase       = "https://howlongtobeat.com"
	hltbInitPath   = "/api/bleed/init"
	hltbSearchPath = "/api/bleed"
	hltbTimeout    = 30 * time.Second
	hltbTokenTTL   = 5 * time.Minute
)

// HLTBTimes holds completion time estimates from HowLongToBeat in decimal hours.
type HLTBTimes struct {
	Main          float32 `json:"main"`
	MainExtra     float32 `json:"mainExtra"`
	Completionist float32 `json:"completionist"`
}

type hltbToken struct {
	value     string
	hpKey     string
	hpVal     string
	fetchedAt time.Time
}

// HLTBService fetches completion times from howlongtobeat.com.
type HLTBService struct {
	client *http.Client
	ctx    context.Context
	token  *hltbToken
}

func NewHLTBService(ctx context.Context) *HLTBService {
	return &HLTBService{
		client: &http.Client{Timeout: hltbTimeout},
		ctx:    ctx,
	}
}

// Search returns completion times for the game closest to name.
// Returns nil with no error when no match is found.
func (h *HLTBService) Search(name string) (*HLTBTimes, error) {
	if err := h.ensureToken(); err != nil {
		slog.Warn("hltb: ensureToken failed", "error", err)
		return nil, fmt.Errorf("hltb token: %w", err)
	}

	results, err := h.doSearch(name)
	if err != nil {
		slog.Warn("hltb: doSearch failed", "name", name, "error", err)
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	best := bestMatch(name, results)
	if best == nil {
		return nil, nil
	}

	return &HLTBTimes{
		Main:          secondsToHours(best.GameplayMain),
		MainExtra:     secondsToHours(best.GameplayMainExtra),
		Completionist: secondsToHours(best.GameplayComplete),
	}, nil
}

// ensureToken fetches a fresh auth token when absent or expired.
func (h *HLTBService) ensureToken() error {
	if h.token != nil && time.Since(h.token.fetchedAt) < hltbTokenTTL {
		return nil
	}

	url := fmt.Sprintf("%s%s?t=%d", hltbBase, hltbInitPath, time.Now().UnixMilli())
	req, err := http.NewRequestWithContext(h.ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	h.setHeaders(req, hltbBase+"/")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hltb init: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var init struct {
		Token string `json:"token"`
		HpKey string `json:"hpKey"`
		HpVal string `json:"hpVal"`
	}
	if err := json.Unmarshal(body, &init); err != nil || init.Token == "" {
		init.Token = extractToken(body)
	}
	if init.Token == "" {
		return fmt.Errorf("hltb: no auth token in init response")
	}

	h.token = &hltbToken{
		value:     init.Token,
		hpKey:     init.HpKey,
		hpVal:     init.HpVal,
		fetchedAt: time.Now(),
	}
	return nil
}


type hltbSearchBody struct {
	SearchType    string            `json:"searchType"`
	SearchTerms   []string          `json:"searchTerms"`
	SearchPage    int               `json:"searchPage"`
	Size          int               `json:"size"`
	SearchOptions hltbSearchOptions `json:"searchOptions"`
	UseCache      bool              `json:"useCache"`
}

type hltbSearchOptions struct {
	Games      hltbGameOptions `json:"games"`
	Users      hltbUserOptions `json:"users"`
	Lists      hltbListOptions `json:"lists"`
	Filter     string          `json:"filter"`
	Sort       int             `json:"sort"`
	Randomizer int             `json:"randomizer"`
}

type hltbGameOptions struct {
	UserID        int            `json:"userId"`
	Platform      string         `json:"platform"`
	SortCategory  string         `json:"sortCategory"`
	RangeCategory string         `json:"rangeCategory"`
	Modifier      string         `json:"modifier"`
	RangeTime     hltbRangeTime  `json:"rangeTime"`
	Gameplay      hltbGameplay   `json:"gameplay"`
	RangeYear     hltbRangeYear  `json:"rangeYear"`
}

type hltbGameplay struct {
	Perspective string `json:"perspective"`
	Flow        string `json:"flow"`
	Genre       string `json:"genre"`
	Difficulty  string `json:"difficulty"`
}

type hltbRangeTime struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type hltbRangeYear struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

type hltbUserOptions struct {
	SortCategory string `json:"sortCategory"`
}

type hltbListOptions struct {
	SortCategory string `json:"sortCategory"`
}

type hltbSearchResult struct {
	GameID           int     `json:"game_id"`
	GameName         string  `json:"game_name"`
	GameplayMain     int     `json:"comp_main"`
	GameplayMainExtra int    `json:"comp_plus"`
	GameplayComplete int     `json:"comp_100"`
}

type hltbSearchResponse struct {
	Data []hltbSearchResult `json:"data"`
}

func (h *HLTBService) doSearch(name string) ([]hltbSearchResult, error) {
	payload := hltbSearchBody{
		SearchType:  "games",
		SearchTerms: strings.Fields(name),
		SearchPage:  1,
		Size:        20,
		UseCache:    true,
		SearchOptions: hltbSearchOptions{
			Games: hltbGameOptions{
				UserID:        0,
				Platform:      "",
				SortCategory:  "popular",
				RangeCategory: "main",
				Modifier:      "",
				RangeTime:     hltbRangeTime{Min: 0, Max: 0},
				Gameplay:      hltbGameplay{},
				RangeYear:     hltbRangeYear{},
			},
			Users:      hltbUserOptions{SortCategory: "postcount"},
			Lists:      hltbListOptions{SortCategory: "follows"},
			Filter:     "",
			Sort:       0,
			Randomizer: 0,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Inject the dynamic hpKey field that HLTB requires in the body.
	if h.token.hpKey != "" {
		var m map[string]any
		if err := json.Unmarshal(bodyBytes, &m); err != nil {
			return nil, err
		}
		m[h.token.hpKey] = h.token.hpVal
		if bodyBytes, err = json.Marshal(m); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(h.ctx, "POST", hltbBase+hltbSearchPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	h.setHeaders(req, hltbBase+"/")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", h.token.value)
	req.Header.Set("x-hp-key", h.token.hpKey)
	req.Header.Set("x-hp-val", h.token.hpVal)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hltb search: status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result hltbSearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("hltb search: decode: %w", err)
	}
	return result.Data, nil
}

func (h *HLTBService) setHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", hltbBase)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// bestMatch picks the result whose name is closest to the query using a
// simple normalised character-overlap similarity score.
func bestMatch(query string, results []hltbSearchResult) *hltbSearchResult {
	q := normalize(query)
	best := -1.0
	var match *hltbSearchResult
	for i := range results {
		score := similarity(q, normalize(results[i].GameName))
		if score > best {
			best = score
			match = &results[i]
		}
	}
	return match
}

func normalize(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9 ]`).ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// similarity returns a Jaccard-like score over character bigrams.
func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	ba, bb := bigrams(a), bigrams(b)
	intersection := 0
	for k, ca := range ba {
		if cb, ok := bb[k]; ok {
			if ca < cb {
				intersection += ca
			} else {
				intersection += cb
			}
		}
	}
	union := 0
	for _, c := range ba {
		union += c
	}
	for k, c := range bb {
		if _, ok := ba[k]; !ok {
			union += c
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func bigrams(s string) map[string]int {
	m := make(map[string]int)
	runes := []rune(s)
	for i := 0; i+1 < len(runes); i++ {
		m[string(runes[i:i+2])]++
	}
	return m
}

// HLTB returns times in seconds; convert to hours rounded to 1 decimal.
func secondsToHours(seconds int) float32 {
	if seconds <= 0 {
		return 0
	}
	hours := float64(seconds) / 3600.0
	return float32(math.Round(hours*10) / 10)
}

// extractToken tries to pull a token value from a raw JSON body when the
// top-level "token" field is absent (HLTB occasionally nests it differently).
var tokenRe = regexp.MustCompile(`"token"\s*:\s*"([^"]+)"`)

func extractToken(body []byte) string {
	m := tokenRe.FindSubmatch(body)
	if len(m) > 1 {
		return string(m[1])
	}
	return ""
}
