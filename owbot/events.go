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

var templateInitialMessage = template.Must(template.New("InitialMessage").Parse(strings.TrimSpace(`
**{{ .Username }}**: SR *(pending...)*
`)))

type playerSessionData struct {
	Username string
	FinalSR  int
	SRDiff   int
}

var templateUpdatedMessage = template.Must(template.New("UpdatedMessage").Parse(strings.TrimSpace(`
**{{ .Username }}**: SR {{ .FinalSR }} ({{if (ge .SRDiff 0)}}+{{end}}{{ .SRDiff }})
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
	initialMessage := bot.getTemplateMessage(templateInitialMessage, next.User)
	messageResult, err := bot.discord.CreateMessage(initialMessage)
	if err != nil {
		bot.logger.Error("failed to send message")
		return
	}

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

	var lastMessage string
	if prev.UserStats == nil && next.UserStats == nil {
		bot.logger.Warn("no user stats found")
		lastMessage = bot.getTemplateMessage(templateErrorMessage, prev)
	} else if prev.UserStats == nil && next.UserStats != nil {
		bot.logger.Warn("no previous user stats found")
		lastMessage = bot.getTemplateMessage(templateNoChangeMessage, next)
	} else if prev.UserStats != nil && next.UserStats == nil {
		bot.logger.Warn("no next user stats found")
		lastMessage = bot.getTemplateMessage(templateNoChangeMessage, prev)
	} else if isStatsDifferent(prev.UserStats, next.UserStats) {
		lastMessage = bot.getTemplateMessage(templateUpdatedMessage, playerSessionData{Username: next.User.Username,
			FinalSR: next.UserStats.OverallStats.CompRank,
			SRDiff:  next.UserStats.OverallStats.CompRank - prev.UserStats.OverallStats.CompRank})
	} else {
		lastMessage = bot.getTemplateMessage(templateNoChangeMessage, next)
	}

	bot.discord.UpdateMessage(messageResult.ID, lastMessage)
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
