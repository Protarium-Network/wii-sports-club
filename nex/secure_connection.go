package nex

import (
	"fmt"
	"time"

	nex "github.com/PretendoNetwork/nex-go/v2"
	nex_types "github.com/PretendoNetwork/nex-go/v2/types"
	common_secure "github.com/PretendoNetwork/nex-protocols-common-go/v2/secure-connection"
	secure "github.com/PretendoNetwork/nex-protocols-go/v2/secure-connection"
	"github.com/Protarium-Network/wii-sports-club/database"
	"github.com/Protarium-Network/wii-sports-club/globals"
)

// registerSecureConnection gère l'arrivée des consoles (Register) et leur départ.
func registerSecureConnection() {
	secureProtocol := secure.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(secureProtocol)
	secureCommon := common_secure.NewCommonProtocol(secureProtocol)

	// Wii Sports Club utilise SecureConnection::Register (méthode 1), confirmé par
	// des captures de console. L'authentification passe quand même d'abord par TicketGranting.
	secureCommon.EnableInsecureRegister()
	secureCommon.CreateReportDBRecord = database.CreateReportDBRecord

	// Logs des StationURLs à chaque connexion, pour diagnostiquer les problèmes P2P
	// (IP privée vs publique).
	secureCommon.OnAfterRegister = func(packet nex.PacketInterface, vecMyURLs nex_types.List[nex_types.StationURL]) {
		logRegistration("REGISTER", packet)
	}
	secureCommon.OnAfterRegisterEx = func(packet nex.PacketInterface, vecMyURLs nex_types.List[nex_types.StationURL], hCustomData nex_types.DataHolder) {
		logRegistration("REGISTER-EX", packet)
	}

	// IMPORTANT : ce callback s'exécute APRÈS le DisconnectParticipant de Pretendo,
	// car Pretendo ajoute son propre handler en premier dans NewCommonProtocol.
	globals.SecureEndpoint.OnConnectionEnded(onPlayerDisconnected)
}

func logRegistration(label string, packet nex.PacketInterface) {
	conn := packet.Sender().(*nex.PRUDPConnection)
	urls := make([]string, len(conn.StationURLs))
	for i, url := range conn.StationURLs {
		urls[i] = url.URL()
	}
	fmt.Printf("[%s] PID=%d RVCID=%d UDP=%s URLs=%v\n", label, uint64(conn.PID()), conn.ID, conn.Address().String(), urls)
}

// onPlayerDisconnected retire le joueur puis remet les parties en ordre.
func onPlayerDisconnected(connection *nex.PRUDPConnection) {
	pid := uint32(connection.PID())
	fmt.Printf("[SERVER] Joueur déconnecté PID=%d\n", pid)
	globals.RemovePlayer(pid)

	go func() {
		// On laisse 200 ms à Pretendo pour finir ses requêtes en base.
		time.Sleep(200 * time.Millisecond)

		if globals.MatchmakingManager == nil || globals.MatchmakingManager.Database == nil {
			return
		}
		reopenSessionsWithRoom(pid)
		removeEmptySessions()
	}()
}

// reopenSessionsWithRoom rouvre la participation des parties qui ont de nouveau de la place.
// Sans ça, le 2e joueur ne retrouve plus la session du 1er à la recherche suivante.
func reopenSessionsWithRoom(disconnectedPID uint32) {
	result, err := globals.MatchmakingManager.Database.Exec(`
		UPDATE matchmaking.matchmake_sessions ms
		SET open_participation = true
		FROM matchmaking.gatherings g
		WHERE ms.id = g.id
		  AND g.registered = true
		  AND ms.open_participation = false
		  AND COALESCE(array_length(g.participants, 1), 0) < g.max_participants
	`)
	if err != nil {
		fmt.Printf("[SERVER] Erreur ré-ouverture sessions: %v\n", err)
		return
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		fmt.Printf("[SERVER] ✓ %d session(s) ré-ouverte(s) après déconnexion PID=%d\n", rows, disconnectedPID)
	}
}

// removeEmptySessions retire du dashboard les parties sans aucun participant.
func removeEmptySessions() {
	rows, err := globals.MatchmakingManager.Database.Query(`
		SELECT id FROM matchmaking.gatherings
		WHERE registered = true AND COALESCE(array_length(participants, 1), 0) = 0
	`)
	if err != nil {
		return
	}

	var emptyGIDs []uint32
	for rows.Next() {
		var gid uint32
		if rows.Scan(&gid) == nil {
			emptyGIDs = append(emptyGIDs, gid)
		}
	}
	rows.Close()

	for _, gid := range emptyGIDs {
		globals.RemoveSession(gid)
	}
}
