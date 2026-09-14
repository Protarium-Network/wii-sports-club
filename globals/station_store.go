package globals

import (
	"sync"
	nex_types "github.com/PretendoNetwork/nex-go/v2/types"
)

// PlayerInfo stores information about a connected player
type PlayerInfo struct {
	PID         uint32
	Username    string
	StationURLs []string
	GID         uint32 // Current gathering/session ID
}

// SessionInfo stores information about a matchmaking session
type SessionInfo struct {
	GID         uint32
	GameMode    uint32
	Sport       string
	PlayerPIDs  []uint32
	MaxPlayers  uint32
}

var (
	stationStoreMu sync.RWMutex
	StationStore   = make(map[uint32][]nex_types.StationURL) // PID -> StationURLs

	sessionStoreMu sync.RWMutex
	SessionStore   = make(map[uint32]*SessionInfo) // GID -> SessionInfo

	playerStoreMu sync.RWMutex
	PlayerStore   = make(map[uint32]*PlayerInfo) // PID -> PlayerInfo
)

// SetPlayerStationURLs stores the StationURLs for a given PID
func SetPlayerStationURLs(pid uint32, urls []nex_types.StationURL) {
	stationStoreMu.Lock()
	defer stationStoreMu.Unlock()
	StationStore[pid] = urls
}

// GetPlayerStationURLs retrieves the StationURLs for a given PID
func GetPlayerStationURLs(pid uint32) []nex_types.StationURL {
	stationStoreMu.RLock()
	defer stationStoreMu.RUnlock()
	return StationStore[pid]
}

// SetPlayerInfo stores or updates info about a player
func SetPlayerInfo(pid uint32, info *PlayerInfo) {
	playerStoreMu.Lock()
	defer playerStoreMu.Unlock()
	PlayerStore[pid] = info
}

// RemovePlayer removes a player from the store
func RemovePlayer(pid uint32) {
	playerStoreMu.Lock()
	delete(PlayerStore, pid)
	playerStoreMu.Unlock()

	stationStoreMu.Lock()
	delete(StationStore, pid)
	stationStoreMu.Unlock()
}

// GetAllPlayers returns a snapshot of all connected players
func GetAllPlayers() []*PlayerInfo {
	playerStoreMu.RLock()
	defer playerStoreMu.RUnlock()
	players := make([]*PlayerInfo, 0, len(PlayerStore))
	for _, p := range PlayerStore {
		players = append(players, p)
	}
	return players
}

// SetSessionInfo stores or updates a matchmaking session
func SetSessionInfo(gid uint32, info *SessionInfo) {
	sessionStoreMu.Lock()
	defer sessionStoreMu.Unlock()
	SessionStore[gid] = info
}

// GetAllSessions returns a snapshot of all active sessions
func GetAllSessions() []*SessionInfo {
	sessionStoreMu.RLock()
	defer sessionStoreMu.RUnlock()
	sessions := make([]*SessionInfo, 0, len(SessionStore))
	for _, s := range SessionStore {
		sessions = append(sessions, s)
	}
	return sessions
}

// RemoveSession removes a matchmaking session from the store, e.g. once it has no participants left
func RemoveSession(gid uint32) {
	sessionStoreMu.Lock()
	delete(SessionStore, gid)
	sessionStoreMu.Unlock()
}

// GetSportName converts a GameMode to a human-readable sport name
func GetSportName(gameMode uint32) string {
	// Wii Sports Club sport detection based on GameMode high byte
	switch gameMode >> 24 {
	case 0x03:
		return "⛳ Golf"
	case 0x04:
		return "🎾 Tennis"
	case 0x02:
		return "🎳 Bowling"
	case 0x01:
		return "⚾ Baseball"
	case 0x05:
		return "🥊 Boxe"
	default:
		return "🎮 Inconnu"
	}
}
