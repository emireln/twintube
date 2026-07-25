package room

import (
	"math/rand"
	"sync"
	"time"
)

// StarterVideo is a default YouTube clip shown when a new room is created.
type StarterVideo struct {
	VideoID string
	Title   string
}

// DefaultStarterVideos is the pool used when seeding a brand-new room.
// One entry is chosen at random so rooms do not always open on the same clip.
var DefaultStarterVideos = []StarterVideo{
	{VideoID: "dQw4w9WgXcQ", Title: "Rick Astley - Never Gonna Give You Up"},
	{VideoID: "jNQXAC9IVRw", Title: "Me at the zoo"},
	{VideoID: "9bZkp7q19f0", Title: "PSY - GANGNAM STYLE"},
	{VideoID: "kJQP7kiw5Fk", Title: "Luis Fonsi - Despacito ft. Daddy Yankee"},
}

var (
	starterRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
	starterRandMu sync.Mutex
)

// PickRandomStarter returns one of the default starter videos.
func PickRandomStarter() StarterVideo {
	starterRandMu.Lock()
	defer starterRandMu.Unlock()
	if len(DefaultStarterVideos) == 0 {
		return StarterVideo{}
	}
	return DefaultStarterVideos[starterRand.Intn(len(DefaultStarterVideos))]
}

// StarterTitle looks up a known starter title by video ID.
func StarterTitle(videoID string) (string, bool) {
	for _, v := range DefaultStarterVideos {
		if v.VideoID == videoID {
			return v.Title, true
		}
	}
	return "", false
}
