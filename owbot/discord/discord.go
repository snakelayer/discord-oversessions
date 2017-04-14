package discord

import (
	"errors"
	"regexp"

	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/snakelayer/discord-oversessions/owbot/player"
)

var regexOverwatchChannel = regexp.MustCompile(`^over.*$`)

type DiscordAdapter struct {
	session *discordgo.Session
	guild   *discordgo.UserGuild
	channel *discordgo.Channel
	logger  *logrus.Entry
}

func New(logger *logrus.Logger, token string) (*DiscordAdapter, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	return &DiscordAdapter{
		session: session,
		logger:  logger.WithField("module", "discord"),
	}, nil
}

func (discordAdapter *DiscordAdapter) Connect() error {
	return discordAdapter.session.Open()
}

func (discordAdapter *DiscordAdapter) AddHandler(handler interface{}) {
	discordAdapter.session.AddHandler(handler)
}

func (discordAdapter *DiscordAdapter) SetPlayerPresences(playerStates map[string]player.PlayerState) {
	guild, err := discordAdapter.session.Guild(discordAdapter.guild.ID)
	if err != nil {
		discordAdapter.logger.WithField("guildId", discordAdapter.guild.ID).Error("no guild found")
	}

	for _, presence := range guild.Presences {
		userId := presence.User.ID

		playerState := playerStates[userId]

		playerState.Game = presence.Game
		discordAdapter.setUser(userId, &playerState)

		playerStates[userId] = playerState
	}
}

func (discordAdapter *DiscordAdapter) setUser(userId string, playerState *player.PlayerState) {
	user, err := discordAdapter.session.User(userId)
	if err != nil {
		discordAdapter.logger.WithField("userId", userId).Error("could not find user")
	}

	discordAdapter.logger.WithField("user", user).Debug("user data")
	playerState.User = user
}

func (discordAdapter *DiscordAdapter) SetGuildAndOverwatchChannel() {
	guilds, err := discordAdapter.session.UserGuilds()
	if err != nil {
		return
	}
	if len(guilds) < 1 {
		return
	}
	discordAdapter.logger.WithField("guild", *guilds[0]).Debug("guild data")
	discordAdapter.guild = guilds[0]

	channels, err := discordAdapter.session.GuildChannels(guilds[0].ID)
	if err != nil {
		return
	}

	for _, channel := range channels {
		discordAdapter.logger.WithField("channel", channel).Debug("channel data")
		if channel.Type == "voice" {
			continue
		}

		if regexOverwatchChannel.MatchString(channel.Name) {
			discordAdapter.logger.WithField("channelId", channel.ID).WithField("channelName", channel.Name).Debug("found overwatch channel")
			discordAdapter.channel = channel
			return
		}

		if discordAdapter.channel == nil {
			discordAdapter.channel = channel
		}
	}

	if discordAdapter.channel == nil {
		discordAdapter.logger.Error("no text channel found")
	}
}

func (discordAdapter *DiscordAdapter) CreateMessage(content string) (m *discordgo.Message, err error) {
	if discordAdapter.channel.ID == "" {
		return nil, errors.New("no text channel for message sending")
	}

	return discordAdapter.session.ChannelMessageSend(discordAdapter.channel.ID, content)
}

func (discordAdapter *DiscordAdapter) UpdateMessage(messageId string, content string) (m *discordgo.Message, err error) {
	if messageId == "" {
		return nil, errors.New("missing messageId")
	}

	if discordAdapter.channel.ID == "" {
		return nil, errors.New("no text channel for message sending")
	}

	return discordAdapter.session.ChannelMessageEdit(discordAdapter.channel.ID, messageId, content)
}

func (discordAdapter *DiscordAdapter) Close() {
	discordAdapter.session.Close()
}
