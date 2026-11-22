package nex

import (
	"fmt"

	nex "github.com/PretendoNetwork/nex-go/v2"
	nex_types "github.com/PretendoNetwork/nex-go/v2/types"
	common_mm "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making"
	common_mm_ext "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making-ext"
	common_mm_db "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making/database"
	common_mmext "github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension"
	matchmake "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	match_making_ext "github.com/PretendoNetwork/nex-protocols-go/v2/match-making-ext"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"
	"github.com/Protarium-Network/wii-sports-club/globals"
)

// Matchmaking : trouver / créer une partie, la fermer, et donner aux joueurs
// les adresses des autres participants.

// Le jeu envoie 9 attributs dans ses critères de recherche, mais la session n'en stocke
// que 6 (CheckValidMatchmakeSession l'exige). Le SQL compare les tailles : avec 9 contre 6,
// aucune session n'était trouvée et chaque joueur créait la sienne.
const maxSearchAttribs = 6

func registerMatchmaking() {
	// MatchMaking (protocole de base)
	mmProtocol := matchmake.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(mmProtocol)
	common_mm.NewCommonProtocol(mmProtocol).SetManager(globals.MatchmakingManager)
	mmProtocol.SetHandlerGetSessionURLs(handleGetSessionURLs)

	// MatchmakeExtension (0x6D)
	mmExtProtocol := matchmake_extension.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(mmExtProtocol)
	commonMMExt := common_mmext.NewCommonProtocol(mmExtProtocol)
	commonMMExt.SetManager(globals.MatchmakingManager)
	// Évite une erreur fatale sur le callback de nettoyage.
	commonMMExt.CleanupMatchmakeSessionSearchCriterias = func(list nex_types.List[match_making_types.MatchmakeSessionSearchCriteria]) {}
	setCloseParticipationHandler(mmExtProtocol)
	setAutoMatchmakeHandler(mmExtProtocol)

	registerMatchMakingExt()
}

// registerMatchMakingExt enregistre MatchMakingExt (0x32), un protocole DIFFÉRENT de
// MatchmakeExtension (0x6D) malgré le nom presque identique. Wii Sports Club appelle
// EndParticipation en fin de match : sans réponse, la requête expirait côté console
// (cause probable de retours au menu après un match).
func registerMatchMakingExt() {
	protocol := match_making_ext.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	common := common_mm_ext.NewCommonProtocol(protocol)
	common.SetManager(globals.MatchmakingManager)
	common.OnAfterEndParticipation = func(packet nex.PacketInterface, idGathering nex_types.UInt32, strMessage nex_types.String) {
		fmt.Printf("[MATCH-END] PID=%d a quitté GID=%d (message=%q)\n",
			uint64(packet.Sender().PID()), uint32(idGathering), string(strMessage))
	}
}

// ---------------------------------------------------------------------------
// CloseParticipation : l'hôte ferme le lobby pour lancer la partie.
// ---------------------------------------------------------------------------

func setCloseParticipationHandler(protocol *matchmake_extension.Protocol) {
	official := protocol.CloseParticipation

	protocol.SetHandlerCloseParticipation(func(err error, packet nex.PacketInterface, callID uint32, gid nex_types.UInt32) (*nex.RMCMessage, *nex.Error) {
		if official == nil {
			return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "CloseParticipation not implemented")
		}

		// On reste aligné sur l'implémentation de référence de Pretendo.
		// SwitchGathering n'est PAS un signal de début de match : l'envoyer à tous les
		// non-hôtes (ancienne expérience) pouvait désynchroniser les consoles.
		requesterPID := uint64(packet.Sender().PID())
		fmt.Printf("[MATCH] CloseParticipation GID=%d requester=%d (Pretendo reference handler)\n", uint32(gid), requesterPID)

		response, nexError := official(err, packet, callID, gid)
		if nexError != nil {
			fmt.Printf("[MATCH] CloseParticipation GID=%d rejected for PID=%d: %v\n", uint32(gid), requesterPID, nexError)
			return nil, nexError
		}

		fmt.Printf("[MATCH] Lobby GID=%d closed by owner PID=%d\n", uint32(gid), requesterPID)
		return response, nil
	})
}

// ---------------------------------------------------------------------------
// AutoMatchmakeWithSearchCriteriaPostpone : recherche automatique d'une partie.
// ---------------------------------------------------------------------------

func setAutoMatchmakeHandler(protocol *matchmake_extension.Protocol) {
	original := protocol.AutoMatchmakeWithSearchCriteriaPostpone

	protocol.SetHandlerAutoMatchmakeWithSearchCriteriaPostpone(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		lstSearchCriteria nex_types.List[match_making_types.MatchmakeSessionSearchCriteria],
		anyGathering match_making_types.GatheringHolder,
		strMessage nex_types.String,
	) (*nex.RMCMessage, *nex.Error) {
		logReceivedSession(anyGathering)
		criteria := truncateSearchCriteria(lstSearchCriteria)

		if original == nil {
			return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "AutoMatchmakeWithSearchCriteriaPostpone not implemented")
		}

		response, rmcError := original(err, packet, callID, criteria, anyGathering, strMessage)
		if rmcError != nil {
			fmt.Printf("[MATCHMAKING] Erreur: %v\n", rmcError)
			return nil, rmcError
		}
		fmt.Printf("[MATCHMAKING] ✓ Succès\n")
		return response, nil
	})
}

// logReceivedSession affiche la session envoyée par la console. On ne la modifie pas :
// les flags d'origine (0x30) sont ceux des captures officielles.
func logReceivedSession(anyGathering match_making_types.GatheringHolder) {
	if anyGathering.Object == nil {
		return
	}
	session, ok := anyGathering.Object.(match_making_types.MatchmakeSession)
	if !ok {
		return
	}
	fmt.Printf("[MATCHMAKING] Session reçue: %d attribs, GameMode=%d, MaxParticipants=%d, MinParticipants=%d, OpenParticipation=%v, Flags=0x%X\n",
		len(session.Attributes), uint32(session.GameMode),
		uint16(session.Gathering.MaximumParticipants), uint16(session.Gathering.MinimumParticipants),
		bool(session.OpenParticipation), uint32(session.Gathering.Flags))
}

// truncateSearchCriteria limite chaque critère à maxSearchAttribs attributs.
func truncateSearchCriteria(list nex_types.List[match_making_types.MatchmakeSessionSearchCriteria]) nex_types.List[match_making_types.MatchmakeSessionSearchCriteria] {
	result := make(nex_types.List[match_making_types.MatchmakeSessionSearchCriteria], 0, len(list))
	for _, criteria := range list {
		rawLen := len(criteria.Attribs)
		keep := rawLen
		if keep > maxSearchAttribs {
			keep = maxSearchAttribs
		}
		criteria.Attribs = append(nex_types.List[nex_types.String](nil), criteria.Attribs[:keep]...)
		result = append(result, criteria)

		fmt.Printf("[MATCHMAKING] Critères: %d→%d attribs, ExcludeLocked=%v, GameMode=%q\n",
			rawLen, keep, bool(criteria.ExcludeLocked), string(criteria.GameMode))
	}
	return result
}

// ---------------------------------------------------------------------------
// GetSessionURLs : adresses des autres joueurs de la partie.
// ---------------------------------------------------------------------------

func handleGetSessionURLs(err error, packet nex.PacketInterface, callID uint32, gid nex_types.UInt32) (*nex.RMCMessage, *nex.Error) {
	endpoint := packet.Sender().Endpoint()
	senderPID := uint64(packet.Sender().PID())
	fmt.Printf("[P2P] GetSessionURLs GID=%d demandé par PID=%d\n", uint32(gid), senderPID)

	globals.MatchmakingManager.Mutex.RLock()
	_, _, participants, _, nexError := common_mm_db.FindGatheringByID(globals.MatchmakingManager, uint32(gid))
	globals.MatchmakingManager.Mutex.RUnlock()

	fmt.Printf("[P2P] GetSessionURLs GID=%d pour PID=%d (Participants DB total=%d: %v)\n", uint32(gid), senderPID, len(participants), participants)

	if nexError != nil {
		fmt.Printf("[P2P] Erreur FindGatheringByID pour GID=%d: %v\n", uint32(gid), nexError)
		return nil, nexError
	}

	updateDashboardSession(uint32(gid), participants)

	if !containsPID(participants, senderPID) {
		fmt.Printf("[P2P] ⚠️ PID=%d n'est pas dans les participants pour GID=%d\n", senderPID, uint32(gid))
		return nil, nex.NewError(nex.ResultCodes.RendezVous.PermissionDenied, "change_error")
	}

	// Une StationURL pour CHAQUE autre participant (nécessaire pour les lobbies de
	// bowling à 3 ou 4 consoles : 6 paires indépendantes à 4 joueurs).
	stationURLs := make(nex_types.List[nex_types.StationURL], 0)
	for _, participantPID := range participants {
		if participantPID == senderPID {
			continue
		}
		conn := globals.SecureEndpoint.FindConnectionByPID(participantPID)
		if conn == nil || len(conn.StationURLs) == 0 {
			continue
		}

		// On ne modifie pas le CID : la console connaît le sien (en général 0).
		url := conn.StationURLs[len(conn.StationURLs)-1].Copy().(nex_types.StationURL)
		fmt.Printf("[P2P]   → PID=%d URL = %s\n", participantPID, url.URL())
		stationURLs = append(stationURLs, url)

		pid32 := uint32(participantPID)
		globals.SetPlayerInfo(pid32, &globals.PlayerInfo{
			PID:         pid32,
			Username:    conn.PID().String(),
			StationURLs: []string{url.URL()},
		})
	}

	fmt.Printf("[P2P] %d StationURL(s) renvoyées pour GID=%d\n", len(stationURLs), uint32(gid))

	stream := nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	stationURLs.WriteTo(stream)

	response := nex.NewRMCSuccess(endpoint, stream.Bytes())
	response.ProtocolID = 0x15
	response.MethodID = 0x29
	response.CallID = callID
	return response, nil
}

// updateDashboardSession alimente le dashboard (sport, joueurs) pour cette partie.
func updateDashboardSession(gid uint32, participants []uint64) {
	var gameMode, maxParticipants uint32
	err := globals.MatchmakingManager.Database.QueryRow(`
		SELECT ms.game_mode, g.max_participants
		FROM matchmaking.gatherings g
		JOIN matchmaking.matchmake_sessions ms ON ms.id = g.id
		WHERE g.id = $1
	`, gid).Scan(&gameMode, &maxParticipants)
	if err != nil {
		return
	}

	playerPIDs := make([]uint32, len(participants))
	for i, p := range participants {
		playerPIDs[i] = uint32(p)
	}
	globals.SetSessionInfo(gid, &globals.SessionInfo{
		GID:        gid,
		GameMode:   gameMode,
		Sport:      globals.GetSportName(gameMode),
		PlayerPIDs: playerPIDs,
		MaxPlayers: maxParticipants,
	})
}

func containsPID(pids []uint64, pid uint64) bool {
	for _, p := range pids {
		if p == pid {
			return true
		}
	}
	return false
}
