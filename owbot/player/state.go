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

	BattleTag  string
	RegionBlob *overwatch.RegionBlob

	Timestamp time.Time
}

func New(battleTag string) PlayerState {
	return PlayerState{
		BattleTag:   battleTag,
		Timestamp:   time.Now(),
		UpdateMutex: new(sync.Mutex)}
}

func (state PlayerState) RecentlyUpdated() bool {
	return time.Since(state.Timestamp) < recentDuration
}

func (state PlayerState) String() string {
	return fmt.Sprintf("{User:%v Game:%v BattleTag:%v Blob:%v Timestamp:%v}", state.User, state.Game, state.BattleTag, state.RegionBlob, state.Timestamp)
}
