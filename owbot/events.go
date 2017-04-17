package owbot

import (
	"bytes"
	"context"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/snakelayer/discord-oversessions/owbot/overwatch"
	"github.com/snakelayer/discord-oversessions/owbot/player"
)

const (
	// Longest amount of time a command is processed until given up on
	commandTimeout = 10 * time.Second

	maxGetUserStatsAttempts = 10
)

var HeroEmojiMap = map[string]string{
	"ana":        "ana",
	"bastion":    "bastion",
	"dva":        "dva",
	"genji":      "genji",
	"hanzo":      "hanzo",
	"junkrat":    "junkrat",
	"lucio":      "lucio",
	"mccree":     "mccree",
	"mei":        "mei",
	"mercy":      "mercy",
	"orisa":      "orisa",
	"pharah":     "pharah",
	"reaper":     "reaper",
	"reinhardt":  "reinhardt",
	"roadhog":    "roadhog",
	"soldier76":  "soldier76",
	"symmetra":   "symmetra",
	"torbjorn":   "torbjorn",
	"tracer":     "tracer",
	"widowmaker": "widowmaker",
	"winston":    "winston",
	"zarya":      "zarya",
	"zenyatta":   "zenyatta",
}

type playerSessionData struct {
	Username string
	FinalSR  int
	SRDiff   int

	Hours   int
	Minutes int

	HeroesWDL map[string]WDL
}

type WDL struct {
	Win  int
	Draw int
	Loss int
}

func (sessionData playerSessionData) WinString() string {
	var buffer bytes.Buffer

	for hero, wdl := range sessionData.HeroesWDL {
		for i := 0; i < wdl.Win; i++ {
			buffer.WriteString(HeroEmojiMap[hero])
		}
	}

	return buffer.String()
}

func (sessionData playerSessionData) DrawString() string {
	var buffer bytes.Buffer

	for hero, wdl := range sessionData.HeroesWDL {
		for i := 0; i < wdl.Draw; i++ {
			buffer.WriteString(HeroEmojiMap[hero])
		}
	}

	return buffer.String()
}

func (sessionData playerSessionData) LossString() string {
	var buffer bytes.Buffer

	for hero, wdl := range sessionData.HeroesWDL {
		for i := 0; i < wdl.Loss; i++ {
			buffer.WriteString(HeroEmojiMap[hero])
		}
	}

	return buffer.String()
}

var templateDiffMessage = template.Must(template.New("DiffMessage").Parse(strings.TrimSpace(`
**{{ .Username }}**:
length: {{if (gt .Hours 0)}}{{ .Hours }} hrs {{end}}{{ .Minutes }} min
wins: {{.WinString}}
draws: {{.DrawString}}
losses: {{.LossString}}
SR: {{ .FinalSR }} ({{if (ge .SRDiff 0)}}+{{end}}{{ .SRDiff }})
`)))

var templateNoChangeMessage = template.Must(template.New("NoChangeMessage").Parse(strings.TrimSpace(`
**{{ .User.Username }}**: SR {{ .UserStats.OverallStats.CompRank }}
`)))

var templateErrorMessage = template.Must(template.New("ErrorMessage").Parse(strings.TrimSpace(`
**{{ .User.Username }}**: *(error retrieving data)*
`)))

// A BattleTag is 3-12 characters, followed by "#", followed by digits
var regexBattleTag = regexp.MustCompile(`^\w{3,12}#\d+$`)

func (bot *Bot) getTemplateMessage(template *template.Template, data interface{}) string {
	var message bytes.Buffer
	err := template.Execute(&message, data)
	if err != nil {
		bot.logger.WithFields(logrus.Fields{
			"error":    err,
			"template": template.Name,
		}).Error("Failed executing template")
		return ""
	}

	return message.String()
}

func (bot *Bot) readyHandler(session *discordgo.Session, ready *discordgo.Ready) {
	//session.UpdateStatus(0, "!help")

	bot.discord.SetGuildAndOverwatchChannel()
	bot.discord.SetPlayerStates(bot.playerStates)
	bot.setActivePlayerStats(bot.playerStates)
	msg, _ := bot.discord.ReadMessage("303409836215762944")
	bot.logger.WithField("msg contents", msg.Content).Debug("emoji check")
}

func (bot *Bot) presenceUpdate(session *discordgo.Session, presenceUpdate *discordgo.PresenceUpdate) {
	bot.logger.WithField("presenceUpdate", presenceUpdate).Debug("start handling presenceUpdate")
	if presenceUpdate.Game != nil && !bot.discord.IsOverwatch(presenceUpdate.Game) {
		return
	}
	userId := presenceUpdate.User.ID
	prevPlayerState := bot.playerStates[userId]

	var nextPlayerState = prevPlayerState
	nextPlayerState.Timestamp = time.Now()
	nextPlayerState.Game = presenceUpdate.Game

	// TODO handle overlapping events
	if startedPlaying(prevPlayerState, nextPlayerState) {
		ctx, _ := context.WithTimeout(context.Background(), commandTimeout)
		bot.getOverwatchDataWithDelay(ctx, &nextPlayerState)
	} else if stoppedPlaying(prevPlayerState, nextPlayerState) {
		bot.generateSessionReport(&prevPlayerState, &nextPlayerState)
	}

	bot.playerStates[userId] = nextPlayerState
	bot.logger.WithField("prev", prevPlayerState).WithField("next", nextPlayerState).Debug("player state transition")
}

func (bot *Bot) getOverwatchDataWithDelay(ctx context.Context, playerState *player.PlayerState) {
	bot.getPlayerStats(ctx, playerState)
	// without a delay owapi sometimes returns 429 Too Many Requests
	time.Sleep(1 * time.Second)
	bot.getPlayerHeroes(ctx, playerState)
}

func (bot *Bot) getPlayerStats(ctx context.Context, playerState *player.PlayerState) {
	stats, err := bot.overwatch.GetStats(ctx, playerState.BattleTag)
	if err != nil {
		bot.logger.WithError(err).Error("failed owapi stats request")
	}

	playerState.UserStats = stats
}

func (bot *Bot) getPlayerHeroes(ctx context.Context, playerState *player.PlayerState) {
	heroes, err := bot.overwatch.GetHeroes(ctx, playerState.BattleTag)
	if err != nil {
		bot.logger.WithError(err).Error("failed owapi heroes request")
	}

	playerState.AllHeroStats = heroes
}

func (bot *Bot) setActivePlayerStats(playerStates map[string]player.PlayerState) {
	for userId, playerState := range playerStates {
		if playerState.BattleTag == "" {
			bot.logger.WithField("userId", userId).Warn("can't get player stats without a battleTag")
			continue
		}

		if bot.discord.IsOverwatch(playerState.Game) {
			bot.logger.WithField("userId", userId).Debug("initializing player stats")
			ctx, _ := context.WithTimeout(context.Background(), commandTimeout)
			bot.getPlayerStats(ctx, &playerState)
			playerStates[userId] = playerState
		}
	}
}

func (bot *Bot) generateSessionReport(prev *player.PlayerState, next *player.PlayerState) {

	// unfortunately, owapi only updates after a player has closed overwatch,
	// and sometimes it takes several minutes before changes are visible
	bot.logger.WithField("player", prev.User.Username).Debug("attempt to get user stats")
	for i := 0; i < maxGetUserStatsAttempts; i++ {
		bot.logger.WithField("attempt", i).Debug("retry")

		ctx, _ := context.WithTimeout(context.Background(), commandTimeout)
		bot.getOverwatchDataWithDelay(ctx, next)

		if isStatsDifferent(prev.AllHeroStats, next.AllHeroStats) {
			bot.logger.Debug("successfully retrieved updated stats")
			break
		}

		time.Sleep(1 * time.Minute)
	}

	var messageContent string
	if prev.UserStats == nil && next.UserStats == nil {
		bot.logger.Warn("no user stats found")
	} else if prev.UserStats == nil && next.UserStats != nil {
		bot.logger.Warn("no previous user stats found")
		messageContent = bot.getTemplateMessage(templateNoChangeMessage, next)
	} else if prev.UserStats != nil && next.UserStats == nil {
		bot.logger.Warn("no next user stats found")
		messageContent = bot.getTemplateMessage(templateNoChangeMessage, prev)
	} else if isStatsDifferent(prev.AllHeroStats, next.AllHeroStats) {
		hours, minutes := getHoursMinutesFromDuration(next.Timestamp.Sub(prev.Timestamp))
		playerSessionData := playerSessionData{
			Username:  next.User.Username,
			FinalSR:   next.UserStats.OverallStats.CompRank,
			SRDiff:    next.UserStats.OverallStats.CompRank - prev.UserStats.OverallStats.CompRank,
			Hours:     hours,
			Minutes:   minutes,
			HeroesWDL: bot.getHeroesWDL(prev.AllHeroStats, next.AllHeroStats),
		}
		messageContent = bot.getTemplateMessage(templateDiffMessage, playerSessionData)

		bot.logger.WithField("playerSessionData", playerSessionData).Info("outputting session data")
	} else {
		// do nothing when there is no change
	}

	if messageContent != "" {
		bot.discord.CreateMessage(messageContent)
	}
}

func (bot *Bot) getHeroesWDL(prev *overwatch.AllHeroStats, next *overwatch.AllHeroStats) map[string]WDL {
	heroesWDL := make(map[string]WDL)

	emptyHeroStruct := overwatch.HeroStruct{}
	emptyHeroStruct.GeneralStats.GamesLost = 0
	emptyHeroStruct.GeneralStats.GamesPlayed = 0
	emptyHeroStruct.GeneralStats.GamesWon = 0

	if next.Ana != nil {
		if prev.Ana != nil {
			heroesWDL["ana"] = makeWDL(prev.Ana, next.Ana)
		} else {
			heroesWDL["ana"] = makeWDL(&emptyHeroStruct, next.Ana)
		}
	}
	if next.Bastion != nil {
		if prev.Bastion != nil {
			heroesWDL["bastion"] = makeWDL(prev.Bastion, next.Bastion)
		} else {
			heroesWDL["bastion"] = makeWDL(&emptyHeroStruct, next.Bastion)
		}
	}
	if next.Dva != nil {
		if prev.Dva != nil {
			heroesWDL["dva"] = makeWDL(prev.Dva, next.Dva)
		} else {
			heroesWDL["dva"] = makeWDL(&emptyHeroStruct, next.Dva)
		}
	}
	if next.Junkrat != nil {
		if prev.Junkrat != nil {
			heroesWDL["junkrat"] = makeWDL(prev.Junkrat, next.Junkrat)
		} else {
			heroesWDL["junkrat"] = makeWDL(&emptyHeroStruct, next.Junkrat)
		}
	}
	if next.Lucio != nil {
		if prev.Lucio != nil {
			heroesWDL["lucio"] = makeWDL(prev.Lucio, next.Lucio)
		} else {
			heroesWDL["lucio"] = makeWDL(&emptyHeroStruct, next.Lucio)
		}
	}
	if next.Mccree != nil {
		if prev.Mccree != nil {
			heroesWDL["mccree"] = makeWDL(prev.Mccree, next.Mccree)
		} else {
			heroesWDL["mccree"] = makeWDL(&emptyHeroStruct, next.Mccree)
		}
	}
	if next.Mei != nil {
		if prev.Mei != nil {
			heroesWDL["mei"] = makeWDL(prev.Mei, next.Mei)
		} else {
			heroesWDL["mei"] = makeWDL(&emptyHeroStruct, next.Mei)
		}
	}
	if next.Mercy != nil {
		if prev.Mercy != nil {
			heroesWDL["mercy"] = makeWDL(prev.Mercy, next.Mercy)
		} else {
			heroesWDL["mercy"] = makeWDL(&emptyHeroStruct, next.Mercy)
		}
	}
	if next.Orisa != nil {
		if prev.Orisa != nil {
			heroesWDL["orisa"] = makeWDL(prev.Orisa, next.Orisa)
		} else {
			heroesWDL["orisa"] = makeWDL(&emptyHeroStruct, next.Orisa)
		}
	}
	if next.Reinhardt != nil {
		if prev.Reinhardt != nil {
			heroesWDL["reinhardt"] = makeWDL(prev.Reinhardt, next.Reinhardt)
		} else {
			heroesWDL["reinhardt"] = makeWDL(&emptyHeroStruct, next.Reinhardt)
		}
	}
	if next.Roadhog != nil {
		if prev.Roadhog != nil {
			heroesWDL["roadhog"] = makeWDL(prev.Roadhog, next.Roadhog)
		} else {
			heroesWDL["roadhog"] = makeWDL(&emptyHeroStruct, next.Roadhog)
		}
	}
	if next.Soldier76 != nil {
		if prev.Soldier76 != nil {
			heroesWDL["soldier76"] = makeWDL(prev.Soldier76, next.Soldier76)
		} else {
			heroesWDL["soldier76"] = makeWDL(&emptyHeroStruct, next.Soldier76)
		}
	}
	if next.Torbjorn != nil {
		if prev.Torbjorn != nil {
			heroesWDL["torbjorn"] = makeWDL(prev.Torbjorn, next.Torbjorn)
		} else {
			heroesWDL["torbjorn"] = makeWDL(&emptyHeroStruct, next.Torbjorn)
		}
	}
	if next.Tracer != nil {
		if prev.Tracer != nil {
			heroesWDL["tracer"] = makeWDL(prev.Tracer, next.Tracer)
		} else {
			heroesWDL["tracer"] = makeWDL(&emptyHeroStruct, next.Tracer)
		}
	}
	if next.Winston != nil {
		if prev.Winston != nil {
			heroesWDL["winston"] = makeWDL(prev.Winston, next.Winston)
		} else {
			heroesWDL["winston"] = makeWDL(&emptyHeroStruct, next.Winston)
		}
	}
	if next.Zarya != nil {
		if prev.Zarya != nil {
			heroesWDL["zarya"] = makeWDL(prev.Zarya, next.Zarya)
		} else {
			heroesWDL["zarya"] = makeWDL(&emptyHeroStruct, next.Zarya)
		}
	}
	if next.Zenyatta != nil {
		if prev.Zenyatta != nil {
			heroesWDL["zenyatta"] = makeWDL(prev.Zenyatta, next.Zenyatta)
		} else {
			heroesWDL["zenyatta"] = makeWDL(&emptyHeroStruct, next.Zenyatta)
		}
	}
	if next.Ana != nil {
		if prev.Ana != nil {
			heroesWDL["ana"] = makeWDL(prev.Ana, next.Ana)
		} else {
			heroesWDL["ana"] = makeWDL(&emptyHeroStruct, next.Ana)
		}
	}

	return heroesWDL
}

func makeWDL(prev *overwatch.HeroStruct, next *overwatch.HeroStruct) WDL {
	return WDL{
		Win:  int(next.GeneralStats.GamesWon - prev.GeneralStats.GamesWon),
		Draw: int((next.GeneralStats.GamesPlayed - next.GeneralStats.GamesWon - next.GeneralStats.GamesLost) - (prev.GeneralStats.GamesPlayed - prev.GeneralStats.GamesWon - prev.GeneralStats.GamesLost)),
		Loss: int(next.GeneralStats.GamesLost - prev.GeneralStats.GamesLost),
	}
}

func startedPlaying(prev player.PlayerState, next player.PlayerState) bool {
	if prev.Game == nil && next.Game != nil {
		return true
	}

	return false
}

func stoppedPlaying(prev player.PlayerState, next player.PlayerState) bool {
	if prev.Game != nil && next.Game == nil {
		return true
	}

	return false
}

func isStatsDifferent(prev *overwatch.AllHeroStats, next *overwatch.AllHeroStats) bool {
	if reflect.DeepEqual(prev, next) {
		return false
	}

	return true
}

func getHoursMinutesFromDuration(duration time.Duration) (int, int) {
	var hours, minutes int
	minutes = int(duration.Minutes())
	hours = minutes / 60
	minutes -= hours * 60

	return hours, minutes
}
