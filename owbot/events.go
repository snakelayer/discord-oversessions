package owbot

import (
	"bytes"
	"context"
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

// are these guild specific?
var HeroEmojiMap = map[string]string{
	"ana":        "<:ana:303409414151340035> ",
	"bastion":    "<:bastion:303409414554255360>",
	"dva":        "<:dva:303409415107772416>",
	"genji":      "<:genji:303409415187333130>",
	"hanzo":      "<:hanzo:303409414776422412>",
	"junkrat":    "<:junkrat:303409415112097792>",
	"lucio":      "<:lucio:303409415422476289>",
	"mccree":     "<:mccree:303409414780747786>",
	"mei":        "<:mei:303409415317356544>",
	"mercy":      "<:mercy:303409415346978818>",
	"orisa":      "<:orisa:303409418207232000>",
	"pharah":     "<:pharah:303409415065960450>",
	"reaper":     "<:reaper:303409414487015425>",
	"reinhardt":  "<:reinhardt:303409415011303425>",
	"roadhog":    "<:roadhog:303409415409762315>",
	"soldier76":  "<:soldier_76:303409415069892609>",
	"sombra":     "<:sombra:304543090541068289>",
	"symmetra":   "<:symmetra:303409415787380736>",
	"torbjorn":   "<:torbjorn:303409415514619904>",
	"tracer":     "<:tracer:303409415581859840>",
	"widowmaker": "<:widowmaker:303409415480934400>",
	"winston":    "<:winston:303409414822690821>",
	"zarya":      "<:zarya:303409415472676874>",
	"zenyatta":   "<:zenyatta:303409415166623745>",
}

type playerSessionData struct {
	Username string
	FinalSR  int
	SRDiff   int

	Hours   int
	Minutes int

	HeroesWDL    map[string]overwatch.WDL
	QuickplayWDL overwatch.WDL
}

func (sessionData playerSessionData) HasSRChange() bool {
	return sessionData.SRDiff != 0
}

func (sessionData playerSessionData) HasWins() bool {
	for _, wdl := range sessionData.HeroesWDL {
		if wdl.Win != 0 {
			return true
		}
	}

	return false
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

func (sessionData playerSessionData) HasDraws() bool {
	for _, wdl := range sessionData.HeroesWDL {
		if wdl.Draw != 0 {
			return true
		}
	}

	return false
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

func (sessionData playerSessionData) HasLosses() bool {
	for _, wdl := range sessionData.HeroesWDL {
		if wdl.Loss != 0 {
			return true
		}
	}

	return false
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

func (sessionData playerSessionData) IsEmptyQuickplay() bool {
	return sessionData.QuickplayWDL.IsEmpty()
}

var templateDiffMessage = template.Must(template.New("DiffMessage").Parse(strings.TrimSpace(`
**{{ .Username }}**:
session length: {{if (gt .Hours 0)}}{{ .Hours }} hrs {{end}}{{ .Minutes }} min
{{if not .IsEmptyQuickplay}}quickplay: {{.QuickplayWDL.Win}} {{if (eq .QuickplayWDL.Win 1)}}win{{else}}wins{{end}}, {{.QuickplayWDL.Loss}} {{if (eq .QuickplayWDL.Loss 1)}}loss{{else}}losses{{end}}{{end}}
{{if .HasWins}}comp wins: {{.WinString}}{{end}}
{{if .HasDraws}}comp draws: {{.DrawString}}{{end}}
{{if .HasLosses}}comp losses: {{.LossString}}{{end}}
{{if .HasSRChange}}SR: {{ .FinalSR }} ({{if (ge .SRDiff 0)}}+{{end}}{{ .SRDiff }}){{end}}
`)))

var templateNoChangeMessage = template.Must(template.New("NoChangeMessage").Parse(strings.TrimSpace(`
**{{ .User.Username }}**: SR {{ .RegionBlob.GetCompRank }}
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
	//msg, _ := bot.discord.ReadMessage("303409836215762944")
	//bot.logger.WithField("msg contents", msg.Content).Debug("emoji check")
}

func (bot *Bot) presenceUpdate(session *discordgo.Session, presenceUpdate *discordgo.PresenceUpdate) {
	bot.logger.WithField("presenceUpdate", presenceUpdate).Debug("start handling presenceUpdate")
	if presenceUpdate.Game != nil && !bot.discord.IsOverwatch(presenceUpdate.Game) {
		return
	}

	userId := presenceUpdate.User.ID
	if !bot.HasBattleTag(userId) {
		return
	}

	prevPlayerState := bot.playerStates[userId]

	prevPlayerState.UpdateMutex.Lock()
	defer prevPlayerState.UpdateMutex.Unlock()

	if prevPlayerState.RecentlyUpdated() {
		bot.logger.WithField("userId", userId).Info("abort processing due to recent change")
		return
	}

	if prevPlayerState.User == nil {
		bot.discord.SetUser(userId, &prevPlayerState)
	}

	var nextPlayerState = prevPlayerState
	nextPlayerState.Game = presenceUpdate.Game
	nextPlayerState.Timestamp = time.Now()

	if startedPlaying(prevPlayerState, nextPlayerState) {
		err := bot.setPlayerBlob(&nextPlayerState)
		if err != nil {
			return
		}
	} else if stoppedPlaying(prevPlayerState, nextPlayerState) {
		bot.generateSessionReport(&prevPlayerState, &nextPlayerState)
	}

	bot.playerStates[userId] = nextPlayerState
	bot.logger.WithField("prev", prevPlayerState).WithField("next", nextPlayerState).Debug("player state transition")
}

func (bot *Bot) setActivePlayerStats(playerStates map[string]player.PlayerState) {
	for userId, playerState := range playerStates {
		if playerState.BattleTag == "" {
			bot.logger.WithField("userId", userId).Warn("can't get player stats without a battleTag")
			continue
		}

		if bot.discord.IsOverwatch(playerState.Game) {
			bot.logger.WithField("userId", userId).Debug("initializing player overwatch data")
			bot.setPlayerBlob(&playerState)
			playerStates[userId] = playerState
		}
	}
}

func (bot *Bot) generateSessionReport(prev *player.PlayerState, next *player.PlayerState) {
	if prev.User == nil {
		bot.logger.WithField("playerState", prev).Error("skipping session report with missing User field")
		return
	}

	// unfortunately, owapi only updates after a player has closed overwatch,
	// and sometimes it takes several minutes before changes are visible
	bot.logger.WithField("player", prev.User.Username).Debug("attempt to get user stats")
	for i := 0; i < maxGetUserStatsAttempts; i++ {
		bot.logger.WithField("attempt", i).Debug("retry")

		bot.setPlayerBlob(next)

		if !prev.RegionBlob.Equals(next.RegionBlob) {
			bot.logger.Debug("successfully retrieved updated stats")
			break
		}

		time.Sleep(1 * time.Minute)
	}

	var messageContent string
	if prev.RegionBlob == nil && next.RegionBlob == nil {
		bot.logger.Warn("no user stats found")
	} else if prev.RegionBlob == nil && next.RegionBlob != nil {
		bot.logger.Warn("no previous user stats found")
		messageContent = bot.getTemplateMessage(templateNoChangeMessage, next)
	} else if prev.RegionBlob != nil && next.RegionBlob == nil {
		bot.logger.Warn("no next user stats found")
		messageContent = bot.getTemplateMessage(templateNoChangeMessage, prev)
	} else if !prev.RegionBlob.Equals(next.RegionBlob) {
		hours, minutes := getHoursMinutesFromDuration(next.Timestamp.Sub(prev.Timestamp))
		playerSessionData := playerSessionData{
			Username:     next.User.Username,
			FinalSR:      next.RegionBlob.GetCompRank(),
			SRDiff:       next.RegionBlob.GetCompRank() - prev.RegionBlob.GetCompRank(),
			Hours:        hours,
			Minutes:      minutes,
			HeroesWDL:    bot.getHeroesWDL(prev.RegionBlob.GetAllHeroStats(), next.RegionBlob.GetAllHeroStats()),
			QuickplayWDL: overwatch.GetQuickplayWDLDiff(prev.RegionBlob, next.RegionBlob),
		}
		messageContent = bot.getTemplateMessage(templateDiffMessage, playerSessionData)

		bot.logger.WithField("playerSessionData", playerSessionData).Info("outputting session data")
	} else {
		// do nothing when there is no change
		bot.logger.Info("session ended with no change")
	}

	if messageContent != "" {
		bot.discord.CreateMessage(messageContent)
	}
}

func (bot *Bot) setPlayerBlob(playerState *player.PlayerState) error {
	ctx, _ := context.WithTimeout(context.Background(), commandTimeout)

	blob, err := bot.overwatch.GetUSPlayerBlob(ctx, playerState.BattleTag)
	if err != nil {
		bot.logger.WithError(err).Error("failed to get player blob data")
		return err
	}

	playerState.RegionBlob = blob

	return nil
}

func (bot *Bot) getHeroesWDL(prev *overwatch.AllHeroStats, next *overwatch.AllHeroStats) map[string]overwatch.WDL {
	heroesWDL := make(map[string]overwatch.WDL)

	if next == nil {
		return heroesWDL
	}

	emptyHeroStruct := overwatch.HeroStruct{}
	emptyHeroStruct.GeneralStats.GamesLost = 0
	emptyHeroStruct.GeneralStats.GamesPlayed = 0
	emptyHeroStruct.GeneralStats.GamesWon = 0

	if next.Ana != nil {
		if prev.Ana != nil {
			heroesWDL["ana"] = overwatch.MakeWDL(prev.Ana, next.Ana)
		} else {
			heroesWDL["ana"] = overwatch.MakeWDL(&emptyHeroStruct, next.Ana)
		}
	}
	if next.Bastion != nil {
		if prev.Bastion != nil {
			heroesWDL["bastion"] = overwatch.MakeWDL(prev.Bastion, next.Bastion)
		} else {
			heroesWDL["bastion"] = overwatch.MakeWDL(&emptyHeroStruct, next.Bastion)
		}
	}
	if next.Dva != nil {
		if prev.Dva != nil {
			heroesWDL["dva"] = overwatch.MakeWDL(prev.Dva, next.Dva)
		} else {
			heroesWDL["dva"] = overwatch.MakeWDL(&emptyHeroStruct, next.Dva)
		}
	}
	if next.Genji != nil {
		if prev.Genji != nil {
			heroesWDL["genji"] = overwatch.MakeWDL(prev.Genji, next.Genji)
		} else {
			heroesWDL["genji"] = overwatch.MakeWDL(&emptyHeroStruct, next.Genji)
		}
	}
	if next.Hanzo != nil {
		if prev.Hanzo != nil {
			heroesWDL["hanzo"] = overwatch.MakeWDL(prev.Hanzo, next.Hanzo)
		} else {
			heroesWDL["hanzo"] = overwatch.MakeWDL(&emptyHeroStruct, next.Hanzo)
		}
	}
	if next.Junkrat != nil {
		if prev.Junkrat != nil {
			heroesWDL["junkrat"] = overwatch.MakeWDL(prev.Junkrat, next.Junkrat)
		} else {
			heroesWDL["junkrat"] = overwatch.MakeWDL(&emptyHeroStruct, next.Junkrat)
		}
	}
	if next.Lucio != nil {
		if prev.Lucio != nil {
			heroesWDL["lucio"] = overwatch.MakeWDL(prev.Lucio, next.Lucio)
		} else {
			heroesWDL["lucio"] = overwatch.MakeWDL(&emptyHeroStruct, next.Lucio)
		}
	}
	if next.Mccree != nil {
		if prev.Mccree != nil {
			heroesWDL["mccree"] = overwatch.MakeWDL(prev.Mccree, next.Mccree)
		} else {
			heroesWDL["mccree"] = overwatch.MakeWDL(&emptyHeroStruct, next.Mccree)
		}
	}
	if next.Mei != nil {
		if prev.Mei != nil {
			heroesWDL["mei"] = overwatch.MakeWDL(prev.Mei, next.Mei)
		} else {
			heroesWDL["mei"] = overwatch.MakeWDL(&emptyHeroStruct, next.Mei)
		}
	}
	if next.Mercy != nil {
		if prev.Mercy != nil {
			heroesWDL["mercy"] = overwatch.MakeWDL(prev.Mercy, next.Mercy)
		} else {
			heroesWDL["mercy"] = overwatch.MakeWDL(&emptyHeroStruct, next.Mercy)
		}
	}
	if next.Orisa != nil {
		if prev.Orisa != nil {
			heroesWDL["orisa"] = overwatch.MakeWDL(prev.Orisa, next.Orisa)
		} else {
			heroesWDL["orisa"] = overwatch.MakeWDL(&emptyHeroStruct, next.Orisa)
		}
	}
	if next.Pharah != nil {
		if prev.Pharah != nil {
			heroesWDL["pharah"] = overwatch.MakeWDL(prev.Pharah, next.Pharah)
		} else {
			heroesWDL["pharah"] = overwatch.MakeWDL(&emptyHeroStruct, next.Pharah)
		}
	}
	if next.Reaper != nil {
		if prev.Reaper != nil {
			heroesWDL["reaper"] = overwatch.MakeWDL(prev.Reaper, next.Reaper)
		} else {
			heroesWDL["reaper"] = overwatch.MakeWDL(&emptyHeroStruct, next.Reaper)
		}
	}
	if next.Reinhardt != nil {
		if prev.Reinhardt != nil {
			heroesWDL["reinhardt"] = overwatch.MakeWDL(prev.Reinhardt, next.Reinhardt)
		} else {
			heroesWDL["reinhardt"] = overwatch.MakeWDL(&emptyHeroStruct, next.Reinhardt)
		}
	}
	if next.Roadhog != nil {
		if prev.Roadhog != nil {
			heroesWDL["roadhog"] = overwatch.MakeWDL(prev.Roadhog, next.Roadhog)
		} else {
			heroesWDL["roadhog"] = overwatch.MakeWDL(&emptyHeroStruct, next.Roadhog)
		}
	}
	if next.Soldier76 != nil {
		if prev.Soldier76 != nil {
			heroesWDL["soldier76"] = overwatch.MakeWDL(prev.Soldier76, next.Soldier76)
		} else {
			heroesWDL["soldier76"] = overwatch.MakeWDL(&emptyHeroStruct, next.Soldier76)
		}
	}
	if next.Sombra != nil {
		if prev.Sombra != nil {
			heroesWDL["sombra"] = overwatch.MakeWDL(prev.Sombra, next.Sombra)
		} else {
			heroesWDL["sombra"] = overwatch.MakeWDL(&emptyHeroStruct, next.Sombra)
		}
	}
	if next.Symmetra != nil {
		if prev.Symmetra != nil {
			heroesWDL["symmetra"] = overwatch.MakeWDL(prev.Symmetra, next.Symmetra)
		} else {
			heroesWDL["symmetra"] = overwatch.MakeWDL(&emptyHeroStruct, next.Symmetra)
		}
	}
	if next.Torbjorn != nil {
		if prev.Torbjorn != nil {
			heroesWDL["torbjorn"] = overwatch.MakeWDL(prev.Torbjorn, next.Torbjorn)
		} else {
			heroesWDL["torbjorn"] = overwatch.MakeWDL(&emptyHeroStruct, next.Torbjorn)
		}
	}
	if next.Tracer != nil {
		if prev.Tracer != nil {
			heroesWDL["tracer"] = overwatch.MakeWDL(prev.Tracer, next.Tracer)
		} else {
			heroesWDL["tracer"] = overwatch.MakeWDL(&emptyHeroStruct, next.Tracer)
		}
	}
	if next.Widowmaker != nil {
		if prev.Widowmaker != nil {
			heroesWDL["widowmaker"] = overwatch.MakeWDL(prev.Widowmaker, next.Widowmaker)
		} else {
			heroesWDL["widowmaker"] = overwatch.MakeWDL(&emptyHeroStruct, next.Widowmaker)
		}
	}
	if next.Winston != nil {
		if prev.Winston != nil {
			heroesWDL["winston"] = overwatch.MakeWDL(prev.Winston, next.Winston)
		} else {
			heroesWDL["winston"] = overwatch.MakeWDL(&emptyHeroStruct, next.Winston)
		}
	}
	if next.Zarya != nil {
		if prev.Zarya != nil {
			heroesWDL["zarya"] = overwatch.MakeWDL(prev.Zarya, next.Zarya)
		} else {
			heroesWDL["zarya"] = overwatch.MakeWDL(&emptyHeroStruct, next.Zarya)
		}
	}
	if next.Zenyatta != nil {
		if prev.Zenyatta != nil {
			heroesWDL["zenyatta"] = overwatch.MakeWDL(prev.Zenyatta, next.Zenyatta)
		} else {
			heroesWDL["zenyatta"] = overwatch.MakeWDL(&emptyHeroStruct, next.Zenyatta)
		}
	}

	return heroesWDL
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

func getHoursMinutesFromDuration(duration time.Duration) (int, int) {
	var hours, minutes int
	minutes = int(duration.Minutes())
	hours = minutes / 60
	minutes -= hours * 60

	return hours, minutes
}
