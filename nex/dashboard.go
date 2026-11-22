package nex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Protarium-Network/wii-sports-club/globals"
)

const maxLogLines = 150

type DashboardData struct {
	Players  []globals.PlayerInfo  `json:"players"`
	Sessions []globals.SessionInfo `json:"sessions"`
}

func startDashboardAPI() {
	http.HandleFunc("/api/dashboard", handleDashboard)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/restart", handleRestart)

	fmt.Println("[DASHBOARD] API disponible sur http://127.0.0.1:8080/api/dashboard")
}

func setJSONHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	setJSONHeaders(w)

	players := globals.GetAllPlayers()
	sessions := globals.GetAllSessions()

	data := DashboardData{
		Players:  make([]globals.PlayerInfo, 0, len(players)),
		Sessions: make([]globals.SessionInfo, 0, len(sessions)),
	}
	for _, p := range players {
		data.Players = append(data.Players, *p)
	}
	for _, s := range sessions {
		data.Sessions = append(data.Sessions, *s)
	}

	json.NewEncoder(w).Encode(data)
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	setJSONHeaders(w)

	content, err := os.ReadFile("log/all.log")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
	}

	json.NewEncoder(w).Encode(lines)
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	setJSONHeaders(w)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"restarting"}`))

	fmt.Println("[DASHBOARD] Redémarrage demandé via l'interface web...")
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
