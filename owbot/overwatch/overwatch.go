package overwatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	// The base url of the owapi
	apiBaseUrl = "https://owapi.net/api/v3/"
)

// Top level response to a u/<battle-tag>/stats request
type statsResponse struct {
	KR *regionStats `json:"kr"`
	EU *regionStats `json:"eu"`
	US *regionStats `json:"us"`
}

// Region sub-level part of the stats response
type regionStats struct {
	Stats struct {
		Competitive *UserStats `json:"competitive"`
	} `json:"stats"`
}

type UserStats struct {
	BattleTag string
	Region    string

	OverallStats struct {
		CompRank int     `json:"comprank"`
		Games    int     `json:"games"`
		Level    int     `json:"level"`
		Losses   int     `json:"losses"`
		Prestige int     `json:"prestige"`
		Wins     int     `json:"wins"`
		WinRate  float32 `json:"win_Rate"`
	} `json:"overall_stats"`
	GameStats struct {
		Deaths       float32 `json:"deaths"`
		Eliminations float32 `json:"eliminations"`
		SoloKills    float32 `json:"solo_kills"`
		KPD          float32 `json:"kpd"`
		TimePlayed   float32 `json:"time_played"`
		Medals       float32 `json:"medals"`
		MedalsGold   float32 `json:"medals_gold"`
		MedalsSilver float32 `json:"medals_silver"`
		MedalsBronze float32 `json:"medals_bronze"`
	} `json:"game_stats"`
}

func (userStats UserStats) String() string {
	return fmt.Sprintf("{BattleTag:%v OverallStats:%v}", userStats.BattleTag, userStats.OverallStats)
}

// Top level response to a u/<battle-tag>/heroes request
type heroesResponse struct {
	KR *RegionHeroes `json:"kr"`
	EU *RegionHeroes `json:"eu"`
	US *RegionHeroes `json:"us"`
}

type RegionHeroes struct {
	Heroes struct {
		Stats struct {
			Competitive *AllHeroStats `json:"competitive"`
			// Quickplay is ignored
		} `json:"stats"`
	} `json:"heroes"`
}

type AllHeroStats struct {
	Ana       *HeroStruct `json:"ana"`
	Bastion   *HeroStruct `json:"bastion"`
	Dva       *HeroStruct `json:"dva"`
	Junkrat   *HeroStruct `json:"junkrat"`
	Lucio     *HeroStruct `json:"lucio"`
	Mccree    *HeroStruct `json:"mccree"`
	Mei       *HeroStruct `json:"mei"`
	Mercy     *HeroStruct `json:"mercy"`
	Orisa     *HeroStruct `json:"orisa"`
	Reinhardt *HeroStruct `json:"reinhardt"`
	Roadhog   *HeroStruct `json:"roadhog"`
	Soldier76 *HeroStruct `json:"soldier76"`
	Torbjorn  *HeroStruct `json:"torbjorn"`
	Tracer    *HeroStruct `json:"tracer"`
	Winston   *HeroStruct `json:"winston"`
	Zarya     *HeroStruct `json:"zarya"`
	Zenyatta  *HeroStruct `json:"zenyatta"`
}

type HeroStruct struct {
	// AverageStats is ignored
	GeneralStats struct {
		GamesLost   float32 `json:"games_lost"`
		GamesPlayed float32 `json:"games_played"`
		GamesWon    float32 `json:"games_won"`
	} `json:"general_stats"`
	// HeroStats is ignored
}

func (heroStruct HeroStruct) String() string {
	return fmt.Sprintf("{Played:%v Won:%v Lost:%v}", heroStruct.GeneralStats.GamesPlayed, heroStruct.GeneralStats.GamesWon, heroStruct.GeneralStats.GamesLost)
}

// ErrorResponse is an error that is populated with additional error
// data for the failed request.
// TODO: do we get any extra data on error?
type ErrorResponse struct {
	// The response that caused the error
	Response *http.Response
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%v %v: %d", e.Response.Request.Method, e.Response.Request.URL, e.Response.StatusCode)
}

type OverwatchClient struct {
	logger *logrus.Entry
	client *http.Client

	baseUrl *url.URL

	// Channel of request "tokens". A token must be obtained before
	// making a request against the api, so that we limit the amount
	// of requests we do to a single request at a time. (which we do
	// to not spam the third-party OWAPI we are using)
	nextCh chan bool
}

// Creates a new OverwatchClient, a rest client for querying a third party
// overwatch api.
func NewOverwatchClient(logger *logrus.Logger) (*OverwatchClient, error) {
	// Store the logger as an Entry, adding the module to all log calls
	overwatchLogger := logger.WithField("module", "overwatch")
	client := http.DefaultClient
	baseUrl, _ := url.Parse(apiBaseUrl)

	// Create and initialize the next channel with a token. We use a buffer
	// size of 1 so returning tokens (and the initial add) does not block
	nextCh := make(chan bool, 1)
	nextCh <- true

	return &OverwatchClient{
		logger:  overwatchLogger,
		client:  client,
		baseUrl: baseUrl,
		nextCh:  nextCh,
	}, nil
}

// Takes a response and returns an error if the status code is not within
// the 200-299 range.
func CheckResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}
	return &ErrorResponse{Response: resp}
}

// Creates a new Request for the provided urlStr. The urlStr is resolved
// against baseUrl, and should not include a starting slash. The context
// must not be nil, and is assigned to the request.
func (ow *OverwatchClient) NewRequest(ctx context.Context, urlStr string) (*http.Request, error) {
	ref, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	reqUrl := ow.baseUrl.ResolveReference(ref).String()
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	return req, nil
}

// Do sends a request. If v is not nil, the response is treated as JSON and decoded to v.
// This method blocks until the request is sent and the response is received and parsed.
func (ow *OverwatchClient) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := ow.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); err == nil {
			err = cerr
		}
	}()
	reqLogger := ow.logger.WithFields(logrus.Fields{"method": req.Method, "url": req.URL})

	err = CheckResponse(resp)
	if err != nil {
		reqLogger.WithError(err).Warn("Bad response")
		return nil, err
	}

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
		if err != nil {
			errLogger := reqLogger.WithError(err)
			// We ignore UnmarshalTypeError errors, as returning the zero-value for the
			// field is better than returning nothing
			if _, ok := err.(*json.UnmarshalTypeError); ok {
				errLogger.Warn("Ignoring type error when decoding response as JSON")
			} else {
				errLogger.Error("Could not decode response as JSON")
				return nil, err
			}
		}
	}

	reqLogger.Debug("Request was successful")
	return resp, nil
}

// Returns a UserStats object for the provided BattleTag.
func (ow *OverwatchClient) getStats(ctx context.Context, battleTag string) (*UserStats, error) {
	// Url friendly battleTag
	battleTag = strings.Replace(battleTag, "#", "-", -1)

	// We wait here until either we can obtain a "request token" from nextCh,
	// or our context is canceled.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ow.nextCh:
		defer func() {
			ow.nextCh <- true
		}()
	}

	path := fmt.Sprintf("u/%s/stats", battleTag)
	req, err := ow.NewRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	res := &statsResponse{}
	_, err = ow.Do(req, res)
	if err != nil {
		return nil, err
	}

	// Determine the region to use
	regionStats, regionName := ow.getBestRegion(res)
	if regionStats == nil || regionStats.Stats.Competitive == nil {
		return nil, errors.New("Could not find a region with " +
			"competitive stats for player")
	}

	// Grab the userStats, also add the battle tag from the request
	userStats := regionStats.Stats.Competitive
	userStats.BattleTag = battleTag
	userStats.Region = regionName

	return userStats, nil
}

func (ow *OverwatchClient) getHeroes(ctx context.Context, battleTag string) (*AllHeroStats, error) {
	// Url friendly battleTag
	battleTag = strings.Replace(battleTag, "#", "-", -1)

	// We wait here until either we can obtain a "request token" from nextCh,
	// or our context is canceled.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ow.nextCh:
		defer func() {
			ow.nextCh <- true
		}()
	}

	path := fmt.Sprintf("u/%s/heroes", battleTag)
	req, err := ow.NewRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	res := &heroesResponse{}
	_, err = ow.Do(req, res)
	if err != nil {
		return nil, err
	}

	regionHeroes, _ := ow.getUSRegion(res)
	return regionHeroes.Heroes.Stats.Competitive, nil
}

// Takes a stats response and returns the "best matching" region.
// The best match is the region with most played games in. May
// return nil if all regions are nil.
func (ow *OverwatchClient) getBestRegion(res *statsResponse) (*regionStats, string) {
	type region struct {
		name  string
		stats *regionStats
	}
	regions := []region{
		{"US", res.US},
		{"EU", res.EU},
		{"KR", res.KR},
	}
	var bestMatch region
	mostPlayed := 0

	for _, region := range regions {
		stats := region.stats

		if stats == nil || stats.Stats.Competitive == nil {
			continue
		}
		regionPlayed := stats.Stats.Competitive.OverallStats.Games
		if regionPlayed > mostPlayed {
			mostPlayed = regionPlayed
			bestMatch = region
		}
	}
	return bestMatch.stats, bestMatch.name
}

func (ow *OverwatchClient) getUSRegion(res *heroesResponse) (*RegionHeroes, string) {
	return res.US, "US"
}

func (ow *OverwatchClient) GetStatsAndHeroes(ctx context.Context, battleTag string) (*UserStats, *AllHeroStats, error) {
	stats, err := ow.getStats(ctx, battleTag)
	if err != nil {
		return nil, nil, errors.New("owapi stats request failed")
	}

	// without a delay owapi sometimes returns 429 Too Many Requests
	time.Sleep(1 * time.Second)

	heroes, err := ow.getHeroes(ctx, battleTag)
	if err != nil {
		return nil, nil, errors.New("owapi heroes request failed")
	}

	return stats, heroes, nil
}
