package player

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/snakelayer/discord-oversessions/owbot/overwatch"
)

type PlayerState struct {
	User *discordgo.User
	Game *discordgo.Game

	BattleTag string
	UserStats *overwatch.UserStats

	Timestamp time.Time
}
