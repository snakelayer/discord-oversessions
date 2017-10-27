package discord

import (
	"errors"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/snakelayer/discord-oversessions/owbot/player"
)

var regexOverwatchChannel = regexp.MustCompile(`^over.*$`)

type DiscordAdapter struct {
	session   *discordgo.Session
	guild     *discordgo.UserGuild
	channel   *discordgo.Channel
	ownUserId string

	logger *logrus.Entry
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

func (discordAdapter *DiscordAdapter) SetPlayerState(userId string, playerState *player.PlayerState) {
	presence, err := discordAdapter.session.State.Presence(discordAdapter.guild.ID, userId)
	if err != nil {
		discordAdapter.logger.WithError(err).Error("could not get player presence")
		return
	}

	if discordAdapter.IsOverwatch(presence.Game) {
		playerState.Timestamp = time.Now()
		playerState.Game = presence.Game
	}
}

func (discordAdapter *DiscordAdapter) SetPlayerStates(playerStates map[string]player.PlayerState) {
	guild, err := discordAdapter.session.Guild(discordAdapter.guild.ID)
	if err != nil {
		discordAdapter.logger.WithField("guildId", discordAdapter.guild.ID).Error("no guild found")
	}

	for _, presence := range guild.Presences {
		userId := presence.User.ID

		playerState, ok := playerStates[userId]
		if !ok {
			continue
		}
		if discordAdapter.SetUser(userId, &playerState) != nil {
			continue
		}
		if playerState.User.Bot {
			delete(playerStates, userId)
			continue
		}

		if discordAdapter.IsOverwatch(presence.Game) {
			playerState.Timestamp = time.Now()
			playerState.Game = presence.Game
		}

		playerStates[userId] = playerState
	}
}

func (discordAdapter *DiscordAdapter) SetOwnUserId() error {
	user, err := discordAdapter.session.User("@me")
	if err != nil {
		discordAdapter.logger.WithError(err).Error("could not get own user info")
		return err
	}

	discordAdapter.logger.WithField("ownUserId", user.ID).Info("my userId")
	discordAdapter.ownUserId = user.ID
	return nil
}

func (discordAdapter *DiscordAdapter) GetOwnUserId() string {
	return discordAdapter.ownUserId
}

func (discordAdapter *DiscordAdapter) SetUser(userId string, playerState *player.PlayerState) error {
	user, err := discordAdapter.session.User(userId)
	if err != nil {
		discordAdapter.logger.WithError(err).WithField("userId", userId).Error("could not find user")
		return err
	}

	discordAdapter.logger.WithField("user", user).Debug("retrieved discord user data")
	playerState.User = user
	return nil
}

func (discordAdapter *DiscordAdapter) SetGuildAndOverwatchChannel() {
	guilds, err := discordAdapter.session.UserGuilds(1, "", "")
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
		if channel.Type == 2 {
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

func (discordAdapter *DiscordAdapter) GetOverwatchChannelId() string {
	return discordAdapter.channel.ID
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

func (discordAdapter *DiscordAdapter) ReadMessage(messageId string) (m *discordgo.Message, err error) {
	if messageId == "" {
		return nil, errors.New("missing messageId")
	}

	if discordAdapter.channel.ID == "" {
		return nil, errors.New("no text channel for message sending")
	}

	return discordAdapter.session.ChannelMessage(discordAdapter.channel.ID, messageId)
}

func (discordAdapter *DiscordAdapter) IsOverwatch(game *discordgo.Game) bool {
	return game != nil && game.Name == "Overwatch"
}

func (discordAdapter *DiscordAdapter) IsStreaming(game *discordgo.Game) bool {
	return game != nil && game.Type == 1
}

func (discordAdapter *DiscordAdapter) Close() {
	discordAdapter.session.Close()
}
