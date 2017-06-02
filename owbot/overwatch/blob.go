package overwatch

import (
	"fmt"
	"reflect"
)

type WDL struct {
	Win  int
	Draw int
	Loss int
}

func MakeWDL(prev *HeroStruct, next *HeroStruct) WDL {
	return WDL{
		Win:  int(next.GeneralStats.GamesWon - prev.GeneralStats.GamesWon),
		Draw: int((next.GeneralStats.GamesPlayed - next.GeneralStats.GamesWon - next.GeneralStats.GamesLost) - (prev.GeneralStats.GamesPlayed - prev.GeneralStats.GamesWon - prev.GeneralStats.GamesLost)),
		Loss: int(next.GeneralStats.GamesLost - prev.GeneralStats.GamesLost),
	}
}

// Top level response to a u/<battle-tag>/blob request
type BlobResponse struct {
	KR *RegionBlob `json:"kr"`
	EU *RegionBlob `json:"eu"`
	US *RegionBlob `json:"us"`
}

type RegionBlob struct {
	Heroes struct {
		Stats struct {
			Competitive *AllHeroStats `json:"competitive"`
			// Quickplay is ignored
		} `json:"stats"`
	} `json:"heroes"`
	Stats struct {
		Competitive *UserStats `json:"competitive"`
	} `json:"stats"`
}

func (regionBlob *RegionBlob) Equals(regionBlob2 *RegionBlob) bool {
	return reflect.DeepEqual(regionBlob, regionBlob2)
}

func (regionBlob *RegionBlob) GetCompRank() int {
	return regionBlob.Stats.Competitive.OverallStats.CompRank
}

func (regionBlob *RegionBlob) GetAllHeroStats() *AllHeroStats {
	return regionBlob.Heroes.Stats.Competitive
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

type AllHeroStats struct {
	Ana        *HeroStruct `json:"ana"`
	Bastion    *HeroStruct `json:"bastion"`
	Dva        *HeroStruct `json:"dva"`
	Genji      *HeroStruct `json:"genji"`
	Hanzo      *HeroStruct `json:"hanzo"`
	Junkrat    *HeroStruct `json:"junkrat"`
	Lucio      *HeroStruct `json:"lucio"`
	Mccree     *HeroStruct `json:"mccree"`
	Mei        *HeroStruct `json:"mei"`
	Mercy      *HeroStruct `json:"mercy"`
	Orisa      *HeroStruct `json:"orisa"`
	Pharah     *HeroStruct `json:"pharah"`
	Reaper     *HeroStruct `json:"reaper"`
	Reinhardt  *HeroStruct `json:"reinhardt"`
	Roadhog    *HeroStruct `json:"roadhog"`
	Soldier76  *HeroStruct `json:"soldier76"`
	Sombra     *HeroStruct `json:"sombra"`
	Symmetra   *HeroStruct `json:"symmetra"`
	Torbjorn   *HeroStruct `json:"torbjorn"`
	Tracer     *HeroStruct `json:"tracer"`
	Widowmaker *HeroStruct `json:"widowmaker"`
	Winston    *HeroStruct `json:"winston"`
	Zarya      *HeroStruct `json:"zarya"`
	Zenyatta   *HeroStruct `json:"zenyatta"`
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
