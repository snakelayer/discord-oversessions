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
	commandTimeout = 5 * time.Second

	maxGetUserStatsAttempts = 10
)

type playerSessionData struct {
	Username string
	FinalSR  int
	SRDiff   int

	Hours   int
	Minutes int
}

var templateDiffMessage = template.Must(template.New("DiffMessage").Parse(strings.TrimSpace(`
**{{ .Username }}**:
SR: {{ .FinalSR }} ({{if (ge .SRDiff 0)}}+{{end}}{{ .SRDiff }})
length: {{if (gt .Hours 0)}}{{ .Hours }} hrs {{end}}{{ .Minutes }} min
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
}

func (bot *Bot) presenceUpdate(session *discordgo.Session, presenceUpdate *discordgo.PresenceUpdate) {
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
		bot.getPlayerStats(ctx, &nextPlayerState)
	} else if stoppedPlaying(prevPlayerState, nextPlayerState) {
		bot.generateSessionReport(&prevPlayerState, &nextPlayerState)
	}

	bot.playerStates[userId] = nextPlayerState
	bot.logger.WithField("prev", prevPlayerState).WithField("next", nextPlayerState).Debug("player state transition")
}

func (bot *Bot) getPlayerStats(ctx context.Context, playerState *player.PlayerState) {
	stats, err := bot.overwatch.GetStats(ctx, playerState.BattleTag)
	if err != nil {
		bot.logger.WithError(err).Error("failed call to overwatch api")
	}

	playerState.UserStats = stats
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
		bot.getPlayerStats(ctx, next)

		if isStatsDifferent(prev.UserStats, next.UserStats) {
			bot.logger.Debug("successfully retrieved user stats")
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
	} else if isStatsDifferent(prev.UserStats, next.UserStats) {
		hours, minutes := getHoursMinutesFromDuration(next.Timestamp.Sub(prev.Timestamp))
		bot.logger.WithField("hours", hours).WithField("minutes", minutes).Debug("session duration")

		messageContent = bot.getTemplateMessage(templateDiffMessage, playerSessionData{
			Username: next.User.Username,
			FinalSR:  next.UserStats.OverallStats.CompRank,
			SRDiff:   next.UserStats.OverallStats.CompRank - prev.UserStats.OverallStats.CompRank,
			Hours:    hours,
			Minutes:  minutes,
		})
	} else {
		// do nothing when there is no change
	}

	if messageContent != "" {
		bot.discord.CreateMessage(messageContent)
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

func isStatsDifferent(prev *overwatch.UserStats, next *overwatch.UserStats) bool {
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
