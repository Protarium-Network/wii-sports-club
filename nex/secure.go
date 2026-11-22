package nex

import (
	"fmt"
	"os"
	"strconv"

	nex "github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	pqextended "github.com/PretendoNetwork/pq-extended"
	"github.com/Protarium-Network/wii-sports-club/database"
	"github.com/Protarium-Network/wii-sports-club/globals"
)

// Serveur "Secure" : c'est ici que les consoles arrivent après s'être authentifiées.
//
// Organisation du package :
//   secure.go             -> démarrage du serveur (ce fichier)
//   secure_connection.go  -> enregistrement des consoles + déconnexions
//   nat.go                -> traversée NAT / connexion directe (P2P)
//   matchmaking.go        -> recherche et gestion des parties
//   services.go           -> utility, datastore, classements, service item
//   dashboard.go          -> petite API HTTP de supervision

// StartSecureServer crée, configure puis lance le serveur Secure.
func StartSecureServer() {
	globals.SecureServer = nex.NewPRUDPServer()

	globals.SecureEndpoint = nex.NewPRUDPEndPoint(1)
	globals.SecureEndpoint.IsSecureEndPoint = true
	globals.SecureEndpoint.ServerAccount = globals.SecureServerAccount
	globals.SecureEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.SecureEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername
	globals.SecureServer.BindPRUDPEndPoint(globals.SecureEndpoint)

	globals.SecureServer.ByteStreamSettings.UseStructureHeader = false
	globals.SecureServer.LibraryVersions.SetDefault(nex.NewLibraryVersion(3, 4, 7))
	globals.SecureServer.AccessKey = "4d324052"

	registerLogging()

	globals.MatchmakingManager = common_globals.NewMatchmakingManager(globals.SecureEndpoint, database.Postgres)

	registerSecureServerProtocols()

	port, _ := strconv.Atoi(os.Getenv("PN_SECURE_SERVER_PORT"))
	globals.SecureServer.Listen(port)
}

// registerLogging affiche chaque requête reçue, chaque déconnexion et chaque erreur.
func registerLogging() {
	globals.SecureEndpoint.OnData(func(packet nex.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil {
			return
		}
		fmt.Printf("[WSC Secure] PID=%d %s\n", uint64(packet.Sender().PID()), globals.GetProtocolNameByID(request.ProtocolID))
	})

	globals.SecureEndpoint.OnConnectionEnded(func(connection *nex.PRUDPConnection) {
		fmt.Printf("[WSC Secure] PID=%d disconnected\n", uint64(connection.PID()))
	})

	globals.SecureEndpoint.OnError(func(err *nex.Error) {
		globals.Logger.Errorf("[WSC Secure] %v", err)
	})
}

// registerSecureServerProtocols branche tous les protocoles du serveur.
// L'ORDRE COMPTE : le nettoyage à la déconnexion (secure_connection.go) doit être
// enregistré après la connexion sécurisée de Pretendo.
func registerSecureServerProtocols() {
	fmt.Println("[BUILD] WSC matchmaking sync=host-first-v3")

	registerSecureConnection()
	registerUtility()
	registerDatastore()
	registerNATTraversal()
	registerMatchmaking()
	registerRanking()
	registerServiceItem()

	startDashboardAPI()
}

// newEmptyResponse construit une réponse RMC "succès" sans données.
func newEmptyResponse(protocolID uint16, methodID, callID uint32) *nex.RMCMessage {
	response := nex.NewRMCSuccess(globals.SecureEndpoint, nil)
	response.ProtocolID = protocolID
	response.MethodID = methodID
	response.CallID = callID
	return response
}

// gatheringOfPlayer renvoie la dernière partie (gathering) où se trouve le joueur,
// avec la liste de ses participants.
func gatheringOfPlayer(pid uint64) (gid uint32, participants []uint64, err error) {
	db := globals.MatchmakingManager.Database

	err = db.QueryRow(
		`SELECT id FROM matchmaking.gatherings WHERE $1 = ANY(participants) ORDER BY id DESC LIMIT 1`,
		pid,
	).Scan(&gid)
	if err != nil {
		return 0, nil, err
	}

	// Erreur ignorée volontairement : sans liste, on renvoie simplement une liste vide.
	_ = db.QueryRow(
		`SELECT participants FROM matchmaking.gatherings WHERE id=$1`, gid,
	).Scan(pqextended.Array(&participants))

	return gid, participants, nil
}
