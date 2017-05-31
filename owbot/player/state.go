package player

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/snakelayer/discord-oversessions/owbot/overwatch"
)

// duration within which a change is considered recent
var recentDuration = time.Duration(2) * time.Second

type PlayerState struct {
	UpdateMutex *sync.Mutex

	User *discordgo.User
	Game *discordgo.Game

	BattleTag    string
	UserStats    *overwatch.UserStats
	AllHeroStats *overwatch.AllHeroStats

	Timestamp time.Time
}

func (state PlayerState) RecentlyUpdated() bool {
	if time.Since(state.Timestamp) < recentDuration {
		return true
	}

	return false
}

func (state PlayerState) String() string {
	return fmt.Sprintf("{User:%v Game:%v BattleTag:%v UserStats:%v AllHeroStats:%v Timestamp:%v}", state.User, state.Game, state.BattleTag, state.UserStats, state.AllHeroStats, state.Timestamp)
}
