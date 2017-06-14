package owbot

import (
	"github.com/Sirupsen/logrus"
	"github.com/snakelayer/discord-oversessions/owbot/discord"
	"github.com/snakelayer/discord-oversessions/owbot/overwatch"
	"github.com/snakelayer/discord-oversessions/owbot/player"
)

// The bot is the main component of the ow-bot. It handles events
// from Discord and uses the overwatch client to respond to queries.
type Bot struct {
	logger       *logrus.Entry
	overwatch    *overwatch.OverwatchClient
	discord      *discord.DiscordAdapter
	playerStates map[string]player.PlayerState
}

func (bot *Bot) Start() error {
	// TODO: Check that we are not started

	bot.discord.AddHandler(bot.readyHandler)
	bot.discord.AddHandler(bot.presenceUpdate)
	bot.discord.AddHandler(bot.messageCreate)

	bot.logger.Info("Bot starting, connecting...")
	if err := bot.discord.Connect(); err != nil {
		bot.logger.WithField("error", err).Error("discordsession could not connect")
		return err
	}
	bot.logger.Debug("Connected to Discord")

	return nil
}

func (bot *Bot) Stop() {
	bot.discord.Close()
	bot.logger.Debug("Disconnected from Discord")
}

func NewBot(logger *logrus.Logger, token string, battleTagMap map[string]string) (*Bot, error) {
	overwatch, err := overwatch.NewOverwatchClient(logger)
	if err != nil {
		return nil, err
	}

	discordAdapter, err := discord.New(logger, token)
	if err != nil {
		return nil, err
	}

	var playerStates = make(map[string]player.PlayerState)
	for userId, battleTag := range battleTagMap {
		playerState := player.New(battleTag)
		playerStates[userId] = playerState
		logger.WithField("userId", userId).WithField("battleTag", battleTag).Debug("initialized player state")
	}

	return &Bot{
		logger:       logger.WithField("module", "main"),
		overwatch:    overwatch,
		discord:      discordAdapter,
		playerStates: playerStates,
	}, nil
}

func (bot *Bot) HasBattleTag(userId string) bool {
	if bot.playerStates[userId].BattleTag == "" {
		bot.logger.WithField("userId", userId).Info("no associated battleTag")
		return false
	}

	return true
}
