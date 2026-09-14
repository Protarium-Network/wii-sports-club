package nex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Protarium-Network/wii-sports-club/database"
	globals "github.com/Protarium-Network/wii-sports-club/globals"
	wscranking "github.com/Protarium-Network/wii-sports-club/nex/wscranking"
	nex "github.com/PretendoNetwork/nex-go/v2"
	nex_constants "github.com/PretendoNetwork/nex-go/v2/constants"
	nex_types "github.com/PretendoNetwork/nex-go/v2/types"
	common_datastore "github.com/PretendoNetwork/nex-protocols-common-go/v2/datastore"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	common_mm "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making"
	common_mm_ext "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making-ext"
	common_mm_db "github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making/database"
	common_mmext "github.com/PretendoNetwork/nex-protocols-common-go/v2/matchmake-extension"
	common_nat_traversal "github.com/PretendoNetwork/nex-protocols-common-go/v2/nat-traversal"
	common_ranking "github.com/PretendoNetwork/nex-protocols-common-go/v2/ranking"
	common_secure "github.com/PretendoNetwork/nex-protocols-common-go/v2/secure-connection"
	common_utility "github.com/PretendoNetwork/nex-protocols-common-go/v2/utility"
	datastore "github.com/PretendoNetwork/nex-protocols-go/v2/datastore"
	matchmake "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	match_making_ext "github.com/PretendoNetwork/nex-protocols-go/v2/match-making-ext"
	match_making_types "github.com/PretendoNetwork/nex-protocols-go/v2/match-making/types"
	matchmake_extension "github.com/PretendoNetwork/nex-protocols-go/v2/matchmake-extension"
	nat_traversal "github.com/PretendoNetwork/nex-protocols-go/v2/nat-traversal"
	notifications "github.com/PretendoNetwork/nex-protocols-go/v2/notifications"
	notifications_types "github.com/PretendoNetwork/nex-protocols-go/v2/notifications/types"
	ranking "github.com/PretendoNetwork/nex-protocols-go/v2/ranking"
	secure "github.com/PretendoNetwork/nex-protocols-go/v2/secure-connection"
	service_item "github.com/PretendoNetwork/nex-protocols-go/v2/service-item/wii-sports-club"
	service_item_types "github.com/PretendoNetwork/nex-protocols-go/v2/service-item/wii-sports-club/types"
	utility "github.com/PretendoNetwork/nex-protocols-go/v2/utility"
	pqextended "github.com/PretendoNetwork/pq-extended"
)

func registerCommonSecureServerProtocols() {
	fmt.Println("[BUILD] WSC matchmaking sync=host-first-v3")
	secureProtocol := secure.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(secureProtocol)
	secureCommon := common_secure.NewCommonProtocol(secureProtocol)
	// Wii Sports Club uses SecureConnection::Register (method 1), as confirmed
	// by console captures. Authentication still happens on TicketGranting first.
	secureCommon.EnableInsecureRegister()
	secureCommon.CreateReportDBRecord = database.CreateReportDBRecord

	// =====================================================================
	// LOGGING REGISTRATION — Logguer les StationURLs à chaque connexion
	// pour diagnostiquer les problèmes de P2P (IP privée vs publique)
	// =====================================================================
	secureCommon.OnAfterRegister = func(packet nex.PacketInterface, vecMyURLs nex_types.List[nex_types.StationURL]) {
		conn := packet.Sender().(*nex.PRUDPConnection)
		urls := make([]string, len(conn.StationURLs))
		for i, url := range conn.StationURLs {
			urls[i] = url.URL()
		}
		fmt.Printf("[REGISTER] PID=%d RVCID=%d UDP=%s URLs=%v\n", uint64(conn.PID()), conn.ID, conn.Address().String(), urls)
	}

	secureCommon.OnAfterRegisterEx = func(packet nex.PacketInterface, vecMyURLs nex_types.List[nex_types.StationURL], hCustomData nex_types.DataHolder) {
		conn := packet.Sender().(*nex.PRUDPConnection)
		urls := make([]string, len(conn.StationURLs))
		for i, url := range conn.StationURLs {
			urls[i] = url.URL()
		}
		fmt.Printf("[REGISTER-EX] PID=%d RVCID=%d UDP=%s URLs=%v\n", uint64(conn.PID()), conn.ID, conn.Address().String(), urls)
	}

	// Suivi des déconnexions + ré-ouverture des sessions libérées
	// IMPORTANT : ce callback s'exécute APRÈS DisconnectParticipant de Pretendo
	// (car Pretendo ajoute son handler en premier via NewCommonProtocol)
	globals.SecureEndpoint.OnConnectionEnded(func(connection *nex.PRUDPConnection) {
		pid := uint32(connection.PID())
		fmt.Printf("[SERVER] Joueur déconnecté PID=%d\n", pid)
		globals.RemovePlayer(pid)

		// =========================================================
		// CORRECTION : Ré-ouvrir la participation pour les sessions
		// qui ont maintenant de la place (après que Pretendo a retiré
		// le participant). Sans ça, le 2e joueur ne peut plus trouver
		// la session du 1er lors de la prochaine recherche.
		// =========================================================
		go func() {
			// Attendre 200ms pour que Pretendo finisse ses DB queries
			time.Sleep(200 * time.Millisecond)

			if globals.MatchmakingManager == nil || globals.MatchmakingManager.Database == nil {
				return
			}

			// Re-ouvrir toutes les sessions enregistrées qui ont de la place
			// open_participation est dans matchmake_sessions, participants/max dans gatherings
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
			} else {
				if rows, _ := result.RowsAffected(); rows > 0 {
					fmt.Printf("[SERVER] ✓ %d session(s) ré-ouverte(s) après déconnexion PID=%d\n", rows, pid)
				}
			}

			// Nettoyer les sessions vides du dashboard (plus aucun participant)
			emptyRows, err := globals.MatchmakingManager.Database.Query(`
				SELECT id FROM matchmaking.gatherings
				WHERE registered = true AND COALESCE(array_length(participants, 1), 0) = 0
			`)
			if err == nil {
				var emptyGIDs []uint32
				for emptyRows.Next() {
					var gid uint32
					if emptyRows.Scan(&gid) == nil {
						emptyGIDs = append(emptyGIDs, gid)
					}
				}
				emptyRows.Close()
				for _, gid := range emptyGIDs {
					globals.RemoveSession(gid)
				}
			}
		}()
	})

	utilityProtocol := utility.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(utilityProtocol)
	utility_protocol := common_utility.NewCommonProtocol(utilityProtocol)
	utility_protocol.GenerateNEXUniqueID = GenerateNEXUniqueID

	datastoreProtocol := datastore.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(datastoreProtocol)
	datastore_protocol := common_datastore.NewCommonProtocol(datastoreProtocol)
	datastore_protocol.GetObjectInfosByDataStoreSearchParam = database.GetObjectInfosByDataStoreSearchParam
	datastore_protocol.InitializeObjectByPreparePostParam = database.InitializeObjectByPreparePostParam
	datastore_protocol.InitializeObjectRatingWithSlot = database.InitializeObjectRatingWithSlot
	datastore_protocol.GetObjectInfoByDataID = database.GetObjectInfoByDataID
	datastore_protocol.UpdateObjectPeriodByDataIDWithPassword = database.UpdateObjectPeriodByDataIDWithPassword
	datastore_protocol.UpdateObjectMetaBinaryByDataIDWithPassword = database.UpdateObjectMetaBinaryByDataIDWithPassword
	datastore_protocol.UpdateObjectDataTypeByDataIDWithPassword = database.UpdateObjectDataTypeByDataIDWithPassword

	natTraversalProtocol := nat_traversal.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(natTraversalProtocol)
	common_nat_traversal.NewCommonProtocol(natTraversalProtocol)

	// =====================================================================
	// LOGGING NAT TRAVERSAL — Logguer les requêtes P2P pour diagnostiquer
	// =====================================================================

	// Sauvegarder les handlers Pretendo originaux
	origProbeHandler := natTraversalProtocol.RequestProbeInitiationExt
	_ = origProbeHandler
	origResultHandler := natTraversalProtocol.ReportNATTraversalResult

	natTraversalProtocol.SetHandlerRequestProbeInitiationExt(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		targetList nex_types.List[nex_types.String],
		stationToProbe nex_types.String,
	) (*nex.RMCMessage, *nex.Error) {
		senderConn := packet.Sender().(*nex.PRUDPConnection)
		senderPID := uint64(senderConn.PID())
		senderRVCID := uint32(senderConn.ID)
		fmt.Printf("[NAT] RequestProbeInitiationExt de PID=%d (RVCID=%d)\n", senderPID, senderRVCID)
		fmt.Printf("[NAT]   stationToProbe = %s\n", string(stationToProbe))
		for i, target := range targetList {
			fmt.Printf("[NAT]   targetList[%d] = %s\n", i, string(target))
		}

		// Update senderConn.StationURLs with the exact P2P probe station URL.
		// BUG FIXED: this was never actually done before (only commented as intent).
		// GetSessionURLs hands out conn.StationURLs[0] to peers, which until now stayed
		// frozen at the stale Register-time port forever. The console opens a FRESH local
		// socket specifically for NAT probing (a different port than the one registered at
		// connect time), and only reports that real port here via stationToProbe. Without
		// this update, every retry of GetSessionURLs kept returning the same stale port,
		// so peers kept punching to a dead socket and P2P failed deterministically on every
		// attempt for the same pair — not randomly, and not fixable by retrying.
		probeURL := nex_types.NewStationURL(stationToProbe)
		if len(senderConn.StationURLs) == 0 {
			senderConn.StationURLs = append(senderConn.StationURLs, probeURL.Copy().(nex_types.StationURL))
		} else if len(senderConn.StationURLs) == 1 {
			senderConn.StationURLs = append(senderConn.StationURLs, probeURL.Copy().(nex_types.StationURL))
		} else {
			senderConn.StationURLs[1] = probeURL.Copy().(nex_types.StationURL)
		}

		// Broadcast InitiateProbe to ALL other connected players in the gathering
		var gatheringID uint32
		errDB := globals.MatchmakingManager.Database.QueryRow(
			`SELECT id FROM matchmaking.gatherings WHERE $1 = ANY(participants) ORDER BY id DESC LIMIT 1`,
			uint64(senderPID),
		).Scan(&gatheringID)

		senderCID := 1
		if errDB == nil {
			var participants []uint64
			_ = globals.MatchmakingManager.Database.QueryRow(
				`SELECT participants FROM matchmaking.gatherings WHERE id=$1`, gatheringID,
			).Scan(pqextended.Array(&participants))

			// Calculate sender's CID (index + 1 in gathering participants)
			for idx, p := range participants {
				if p == senderPID {
					senderCID = idx + 1
					break
				}
			}
		}

		for _, targetStr := range targetList {
			targetStation := nex_types.NewStationURL(targetStr)

			// BUGFIX: RVCID != PRUDP CID !
			var targetConn *nex.PRUDPConnection
			targetPIDStr, _ := targetStation.ParamValue("PID")
			if targetPIDStr != "" {
				targetPIDInt, _ := strconv.ParseUint(targetPIDStr, 10, 64)
				targetConn = globals.SecureEndpoint.FindConnectionByPID(targetPIDInt)
			}

			if targetConn != nil {
				tPID := uint64(targetConn.PID())

				fmt.Printf("[NAT] 🚀 InitiateProbe: Sender PID=%d (CID=%d) → Target PID=%d (RVCID=%d, Station: %s)\n",
					senderPID, senderCID, tPID, targetConn.ID, probeURL.URL())

				rmcRequestStream := nex.NewByteStreamOut(targetConn.Endpoint().LibraryVersions(), targetConn.Endpoint().ByteStreamSettings())
				probeURL.WriteTo(rmcRequestStream)
				rmcRequestBody := rmcRequestStream.Bytes()

				rmcRequest := nex.NewRMCRequest(globals.SecureEndpoint)
				rmcRequest.ProtocolID = nat_traversal.ProtocolID
				rmcRequest.CallID = 0xFFFF0000 + callID
				rmcRequest.MethodID = nat_traversal.MethodInitiateProbe
				rmcRequest.Parameters = rmcRequestBody
				rmcRequestBytes := rmcRequest.Bytes()

				server := globals.SecureEndpoint.Server
				var messagePacket nex.PRUDPPacketInterface
				if targetConn.DefaultPRUDPVersion == 0 {
					messagePacket, _ = nex.NewPRUDPPacketV0(server, targetConn, nil)
				} else {
					messagePacket, _ = nex.NewPRUDPPacketV1(server, targetConn, nil)
				}
				messagePacket.SetType(nex_constants.DataPacket)
				messagePacket.AddFlag(nex_constants.PacketFlagNeedsAck)
				messagePacket.AddFlag(nex_constants.PacketFlagReliable)
				messagePacket.SetSourceVirtualPortStreamType(targetConn.StreamType)
				messagePacket.SetSourceVirtualPortStreamID(globals.SecureEndpoint.StreamID)
				messagePacket.SetDestinationVirtualPortStreamType(targetConn.StreamType)
				messagePacket.SetDestinationVirtualPortStreamID(targetConn.StreamID)
				messagePacket.SetPayload(rmcRequestBytes)

				server.Send(messagePacket)
			} else {
				fmt.Printf("[NAT] ⚠️ Client introuvable pour target = %s\n", string(targetStr))
			}
		}

		rmcResponse := nex.NewRMCSuccess(globals.SecureEndpoint, nil)
		rmcResponse.ProtocolID = nat_traversal.ProtocolID
		rmcResponse.MethodID = nat_traversal.MethodRequestProbeInitiationExt
		rmcResponse.CallID = callID
		return rmcResponse, nil
	})

	natTraversalProtocol.SetHandlerReportNATTraversalResult(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		cid nex_types.UInt32,
		result nex_types.Bool,
		rtt nex_types.UInt32,
	) (*nex.RMCMessage, *nex.Error) {
		senderConn := packet.Sender().(*nex.PRUDPConnection)
		senderPID := uint64(senderConn.PID())
		success := bool(result)

		if success {
			fmt.Printf("[NAT] ✓ P2P OK pour PID=%d (CID=%d, RTT=%dms)\n", senderPID, uint32(cid), uint32(rtt))
		} else {
			// Cherche les autres participants de la session pour donner du contexte :
			// un échec P2P entre DEUX joueurs précis casse le mesh complet requis par
			// la console, ce qui la fait généralement revenir au menu pour tout le monde.
			var gatheringID uint32
			var otherParticipants []uint64
			errDB := globals.MatchmakingManager.Database.QueryRow(
				`SELECT id FROM matchmaking.gatherings WHERE $1 = ANY(participants) ORDER BY id DESC LIMIT 1`,
				senderPID,
			).Scan(&gatheringID)
			if errDB == nil {
				var participants []uint64
				_ = globals.MatchmakingManager.Database.QueryRow(
					`SELECT participants FROM matchmaking.gatherings WHERE id=$1`, gatheringID,
				).Scan(pqextended.Array(&participants))
				for _, p := range participants {
					if p != senderPID {
						otherParticipants = append(otherParticipants, p)
					}
				}
			}
			fmt.Printf("[NAT] ⚠️  ÉCHEC P2P pour PID=%d (CID=%d, GID=%d, autres joueurs de la session=%v)\n",
				senderPID, uint32(cid), gatheringID, otherParticipants)
			fmt.Printf("[NAT]     → Probable incompatibilité NAT entre les routeurs des joueurs concernés.\n")
			fmt.Printf("[NAT]     → Ce n'est pas un bug serveur : la connexion directe (P2P) entre CES DEUX consoles a échoué.\n")
			fmt.Printf("[NAT]     → Conséquence typique : la partie ne se lance pas correctement pour tout le monde (retour au menu),\n")
			fmt.Printf("[NAT]       car la console exige une connexion P2P complète entre TOUS les joueurs, pas seulement avec l'hôte.\n")
		}
		if origResultHandler != nil {
			return origResultHandler(err, packet, callID, cid, result, rtt)
		}
		return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "ReportNATTraversalResult not implemented")
	})

	origReportNATPropertiesHandler := natTraversalProtocol.ReportNATProperties
	natTraversalProtocol.SetHandlerReportNATProperties(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		natmapping nex_types.UInt32,
		natfiltering nex_types.UInt32,
		rtt nex_types.UInt32,
	) (*nex.RMCMessage, *nex.Error) {
		connection := packet.Sender().(*nex.PRUDPConnection)

		fmt.Printf("[NAT] ReportNATProperties de PID=%d : mapping=%d, filtering=%d, rtt=%dms\n",
			connection.PID(), uint32(natmapping), uint32(natfiltering), uint32(rtt))

		if origReportNATPropertiesHandler != nil {
			res, nexErr := origReportNATPropertiesHandler(err, packet, callID, natmapping, natfiltering, rtt)
			// Force updating ALL station URLs including public ones after Pretendo did its thing
			for _, station := range connection.StationURLs {
				station.SetNATMapping(nex_constants.NATMappingProperties(natmapping))
				station.SetNATFiltering(nex_constants.NATFilteringProperties(natfiltering))
			}
			return res, nexErr
		}
		return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "ReportNATProperties not implemented")
	})

	mmProtocol := matchmake.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(mmProtocol)
	commonMatchMakingProtocol := common_mm.NewCommonProtocol(mmProtocol)
	commonMatchMakingProtocol.SetManager(globals.MatchmakingManager)

	mmExtProtocol := matchmake_extension.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(mmExtProtocol)
	commonMatchMakingExtProtocol := common_mmext.NewCommonProtocol(mmExtProtocol)
	commonMatchMakingExtProtocol.SetManager(globals.MatchmakingManager)
	officialCloseParticipation := mmExtProtocol.CloseParticipation

	// MatchMakingExt (protocol 0x32) — DIFFERENT protocol from MatchmakeExtension (0x6D) above,
	// despite the near-identical name. Wii Sports Club calls MatchMakingExt::EndParticipation
	// when a match ends. This was never registered, so the console got zero response to that
	// call (every other protocol's HandlePacket silently ignores IDs that aren't its own),
	// causing the request to time out client-side — a likely cause of players getting kicked
	// back to the menu after a match, independent of whether P2P/NAT traversal succeeded.
	mmMatchMakingExtProtocol := match_making_ext.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(mmMatchMakingExtProtocol)
	commonMMExtProtocol := common_mm_ext.NewCommonProtocol(mmMatchMakingExtProtocol)
	commonMMExtProtocol.SetManager(globals.MatchmakingManager)
	commonMMExtProtocol.OnAfterEndParticipation = func(packet nex.PacketInterface, idGathering nex_types.UInt32, strMessage nex_types.String) {
		pid := uint64(packet.Sender().PID())
		fmt.Printf("[MATCH-END] PID=%d a quitté GID=%d (message=%q)\n", pid, uint32(idGathering), string(strMessage))
	}

	mmExtProtocol.SetHandlerCloseParticipation(func(err error, packet nex.PacketInterface, callID uint32, gid nex_types.UInt32) (*nex.RMCMessage, *nex.Error) {
		// Keep Wii Sports Club aligned with Pretendo's reference implementation.
		// SwitchGathering is an "additional participant changed gathering" event,
		// not a match-start signal. The previous experiment sent it to every
		// non-host here and could desynchronise the consoles at match start.
		if officialCloseParticipation != nil {
			requesterPID := uint64(packet.Sender().PID())
			fmt.Printf("[MATCH] CloseParticipation GID=%d requester=%d (Pretendo reference handler)\n",
				uint32(gid), requesterPID)

			response, nexError := officialCloseParticipation(err, packet, callID, gid)
			if nexError != nil {
				fmt.Printf("[MATCH] CloseParticipation GID=%d rejected for PID=%d: %v\n",
					uint32(gid), requesterPID, nexError)
				return nil, nexError
			}

			fmt.Printf("[MATCH] Lobby GID=%d closed by owner PID=%d\n", uint32(gid), requesterPID)
			return response, nil
		}

		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[CRASH PREVENTED] CloseParticipation panic: %v\n", r)
			}
		}()

		if err != nil {
			return nil, nex.NewError(nex.ResultCodes.Core.InvalidArgument, "change_error")
		}

		connection := packet.Sender().(*nex.PRUDPConnection)
		endpoint := globals.SecureEndpoint
		closerPID := uint64(connection.PID())
		gatheringID := uint32(gid)

		globals.MatchmakingManager.Mutex.Lock()
		defer globals.MatchmakingManager.Mutex.Unlock()

		var ownerPID uint64
		var participants []uint64
		errDB := globals.MatchmakingManager.Database.QueryRow(
			`SELECT owner_pid, participants FROM matchmaking.gatherings WHERE id=$1`, gatheringID,
		).Scan(&ownerPID, pqextended.Array(&participants))

		if errDB != nil {
			return nil, nex.NewError(nex.ResultCodes.RendezVous.SessionVoid, "change_error")
		}

		fmt.Printf("[CLOSE-PARTICIPATION] Requête reçue pour GID=%d par PID=%d (Hôte actuel DB=%d, Total Joueurs DB=%d: %v)\n",
			gatheringID, closerPID, ownerPID, len(participants), participants)

		if closerPID != ownerPID {
			fmt.Printf("[CLOSE-PARTICIPATION] ℹ️ Ignoré (non-hôte): CloseParticipation pour GID=%d demandé par non-hôte PID=%d (Hôte officiel: %d)\n", gatheringID, closerPID, ownerPID)
			rmcResponse := nex.NewRMCSuccess(endpoint, nil)
			rmcResponse.ProtocolID = matchmake_extension.ProtocolID
			rmcResponse.MethodID = matchmake_extension.MethodCloseParticipation
			rmcResponse.CallID = callID
			return rmcResponse, nil
		}

		fmt.Printf("[CLOSE-PARTICIPATION] 🚀 HÔTE AUTORISÉ : Fermeture du lobby GID=%d par Hôte PID=%d (%d participants: %v)\n",
			gatheringID, ownerPID, len(participants), participants)

		_, errDB = globals.MatchmakingManager.Database.Exec(`UPDATE matchmaking.matchmake_sessions SET open_participation=$1 WHERE id=$2`, false, gatheringID)
		if errDB != nil {
			fmt.Printf("[CLOSE-PARTICIPATION] ❌ Erreur SQL update open_participation: %v\n", errDB)
			return nil, nex.NewError(nex.ResultCodes.Core.Unknown, errDB.Error())
		}

		// EXPERIMENT: tests showed the P2P connection itself works reliably
		// (real bidirectional traffic confirmed both ways), yet non-host participants
		// consistently never transition into the actual match while the host always
		// does — deterministically, regardless of which physical console hosts.
		// The host's own CloseParticipation call is presumably what its own client
		// uses internally as the "start now" trigger; non-host participants have no
		// equivalent trigger from us. Try giving them one via a notification event.
		for _, participantPID := range participants {
			if participantPID == ownerPID {
				continue
			}

			oEvent := notifications_types.NewNotificationEvent()
			oEvent.PIDSource = nex_types.NewPID(ownerPID)
			oEvent.Type = nex_types.NewUInt32(notifications.BuildNotificationType(notifications.NotificationCategories.SwitchGathering, notifications.NotificationSubTypes.SwitchGathering.None))
			oEvent.Param1 = nex_types.NewUInt32(gatheringID)

			fmt.Printf("[CLOSE-PARTICIPATION] 📢 Envoi signal de lancement (SwitchGathering) à PID=%d pour GID=%d\n", participantPID, gatheringID)
			common_globals.SendNotificationEvent(endpoint, oEvent, []uint64{participantPID})
		}

		rmcResponse := nex.NewRMCSuccess(endpoint, nil)
		rmcResponse.ProtocolID = matchmake_extension.ProtocolID
		rmcResponse.MethodID = matchmake_extension.MethodCloseParticipation
		rmcResponse.CallID = callID

		return rmcResponse, nil
	})

	// Empêche une erreur fatale sur le callback de nettoyage
	commonMatchMakingExtProtocol.CleanupMatchmakeSessionSearchCriterias = func(searchCriterias nex_types.List[match_making_types.MatchmakeSessionSearchCriteria]) {}

	// Sauvegarde du handler Pretendo original
	originalHandler := mmExtProtocol.AutoMatchmakeWithSearchCriteriaPostpone

	// =====================================================================
	// CORRECTION MATCHMAKING — Analyse complète du code source Pretendo
	// =====================================================================
	//
	// PROBLÈME 1 — CheckValidMatchmakeSession (globals/utils.go:52) :
	//   if len(matchmakeSession.Attributes) != 6 { return false }
	//   → La session DOIT avoir exactement 6 attribs. On ne peut pas en mettre 9.
	//
	// PROBLÈME 2 — SQL (find_matchmake_session_by_search_criteria.go:68) :
	//   array_length(ms.attribs, 1)=$3 où $3=len(searchCriteria.Attribs)
	//   → Wii Sports Club envoie 9 attribs dans les critères, mais la session en a 6.
	//   → $3=9 ≠ array_length=6 → AUCUNE session trouvée → les deux joueurs créent
	//     chacun une session séparée → attente infinie.
	//
	// SOLUTION :
	//   1. NE PAS modifier le nombre d'attribs de la session (garder 6)
	//   2. Tronquer les critères de recherche à 6 attribs → SQL $3=6 = array_length=6 ✓
	//   3. Forcer open_participation=true pour que le 2e joueur trouve la session
	// =====================================================================

	mmExtProtocol.SetHandlerAutoMatchmakeWithSearchCriteriaPostpone(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		lstSearchCriteria nex_types.List[match_making_types.MatchmakeSessionSearchCriteria],
		anyGathering match_making_types.GatheringHolder,
		strMessage nex_types.String,
	) (*nex.RMCMessage, *nex.Error) {

		// Étape 1 : Forcer OpenParticipation=true sur la session (le jeu envoie déjà MaxParticipants=4)
		if anyGathering.Object != nil {
			if matchmakeSession, ok := anyGathering.Object.(match_making_types.MatchmakeSession); ok {
				originalMax := uint16(matchmakeSession.Gathering.MaximumParticipants)
				originalMin := uint16(matchmakeSession.Gathering.MinimumParticipants)
				originalFlags := uint32(matchmakeSession.Gathering.Flags)
				// Preserve the game's original gathering flags. Official Wii
				// Sports Club captures use flags=0x30 and notify the owner only.
				// VerboseParticipantsEx does not belong to this title's flow.
				fmt.Printf("[MATCHMAKING] Session reçue: %d attribs, GameMode=%d, MaxParticipants=%d, MinParticipants=%d, OpenParticipation=%v, Flags=0x%X\n",
					len(matchmakeSession.Attributes), uint32(matchmakeSession.GameMode), originalMax, originalMin,
					bool(matchmakeSession.OpenParticipation), originalFlags)
			}
		}

		// Étape 2 : Tronquer les critères à 6 attribs
		sanitizedCriteria := make(nex_types.List[match_making_types.MatchmakeSessionSearchCriteria], 0, len(lstSearchCriteria))
		for _, criteria := range lstSearchCriteria {
			sanitized := criteria
			rawLen := len(sanitized.Attribs)
			targetLen := rawLen
			if targetLen > 6 {
				targetLen = 6
			}
			newAttribs := append(nex_types.List[nex_types.String](nil), sanitized.Attribs[:targetLen]...)
			sanitized.Attribs = newAttribs
			sanitizedCriteria = append(sanitizedCriteria, sanitized)
			fmt.Printf("[MATCHMAKING] Critères: %d→%d attribs, ExcludeLocked=%v, GameMode=%q\n",
				rawLen, targetLen, bool(sanitized.ExcludeLocked), string(sanitized.GameMode))
		}

		if originalHandler != nil {
			rmcMessage, rmcError := originalHandler(err, packet, callID, sanitizedCriteria, anyGathering, strMessage)
			if rmcError != nil {
				fmt.Printf("[MATCHMAKING] Erreur: %v\n", rmcError)
				return nil, rmcError
			}
			fmt.Printf("[MATCHMAKING] ✓ Succès\n")
			return rmcMessage, nil
		}

		return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "AutoMatchmakeWithSearchCriteriaPostpone not implemented")
	})

	// GetSessionURLs — renvoie les StationURLs de tous les autres joueurs connectés de la session
	mmProtocol.SetHandlerGetSessionURLs(func(err error, packet nex.PacketInterface, callID uint32, gid nex_types.UInt32) (*nex.RMCMessage, *nex.Error) {
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

		// Alimente le dashboard avec les infos de la session (sport, joueurs)
		var gameMode uint32
		var maxParticipants uint32
		errSession := globals.MatchmakingManager.Database.QueryRow(`
			SELECT ms.game_mode, g.max_participants
			FROM matchmaking.gatherings g
			JOIN matchmaking.matchmake_sessions ms ON ms.id = g.id
			WHERE g.id = $1
		`, uint32(gid)).Scan(&gameMode, &maxParticipants)
		if errSession == nil {
			playerPIDs := make([]uint32, len(participants))
			for i, p := range participants {
				playerPIDs[i] = uint32(p)
			}
			globals.SetSessionInfo(uint32(gid), &globals.SessionInfo{
				GID:        uint32(gid),
				GameMode:   gameMode,
				Sport:      globals.GetSportName(gameMode),
				PlayerPIDs: playerPIDs,
				MaxPlayers: maxParticipants,
			})
		}

		foundSender := false
		for _, p := range participants {
			if p == senderPID {
				foundSender = true
				break
			}
		}
		if !foundSender {
			fmt.Printf("[P2P] ⚠️ PID=%d n'est pas dans les participants pour GID=%d\n", senderPID, uint32(gid))
			return nil, nex.NewError(nex.ResultCodes.RendezVous.PermissionDenied, "change_error")
		}

		rmcResponseStream := nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
		stationURLs := make(nex_types.List[nex_types.StationURL], 0)

		for _, participantPID := range participants {
			if participantPID == senderPID {
				continue
			}
			// Return one StationURL for every other participant. This is required
			// for bowling lobbies with 3 or 4 consoles (six independent pairs at
			// four players).
			conn := globals.SecureEndpoint.FindConnectionByPID(participantPID)
			if conn != nil && len(conn.StationURLs) > 0 {
				baseURL := conn.StationURLs[len(conn.StationURLs)-1]
				url := baseURL.Copy().(nex_types.StationURL)
				// We do NOT modify the CID here. The console knows its own CID (usually CID=0 for PRUDP connections to peers).
				fmt.Printf("[P2P]   → PID=%d URL = %s\n", participantPID, url.URL())
				stationURLs = append(stationURLs, url)

				pid32 := uint32(participantPID)
				globals.SetPlayerInfo(pid32, &globals.PlayerInfo{
					PID:         pid32,
					Username:    conn.PID().String(),
					StationURLs: []string{url.URL()},
				})
			}
		}

		fmt.Printf("[P2P] %d StationURL(s) renvoyées pour GID=%d\n", len(stationURLs), uint32(gid))
		stationURLs.WriteTo(rmcResponseStream)

		rmcResponseBody := rmcResponseStream.Bytes()
		rmcResponse := nex.NewRMCSuccess(endpoint, rmcResponseBody)
		rmcResponse.ProtocolID = 0x15
		rmcResponse.MethodID = 0x29
		rmcResponse.CallID = callID

		return rmcResponse, nil
	})

	rankingProtocol := ranking.NewProtocol()
	wscRankingProtocol := wscranking.NewProtocol()
	wscRankingProtocol.QueryRankings = database.GetRankings
	wscRankingProtocol.SetEndpoint(globals.SecureEndpoint)
	rankingProtocol.Patches = wscRankingProtocol
	rankingProtocol.PatchedMethods = []uint32{ranking.MethodGetCachedTopXRankings}
	globals.SecureEndpoint.RegisterServiceProtocol(rankingProtocol)
	ranking_protocol := common_ranking.NewCommonProtocol(rankingProtocol)
	ranking_protocol.GetRankingsAndCountByCategoryAndRankingOrderParam = database.GetRankingsAndCountByCategoryAndRankingOrderParam
	ranking_protocol.GetRankingsByMode = database.GetRankings
	ranking_protocol.GetCommonData = database.GetCommonData
	ranking_protocol.UploadCommonData = database.UploadCommonData
	ranking_protocol.InsertRankingByPIDAndRankingScoreData = database.InsertRankingByPIDAndRankingScoreData

	// --- Mock Service Item (Wii Sports Club) ---
	serviceItemProtocol := service_item.NewProtocol(globals.SecureEndpoint)
	serviceItemProtocol.GetServiceItemRightRequest = func(err error, packet nex.PacketInterface, callID uint32, getServiceItemRightParam service_item_types.ServiceItemGetServiceItemRightParam) (*nex.RMCMessage, *nex.Error) {
		fmt.Printf("[SERVICE ITEM] GetServiceItemRightRequest appelé pour autoriser l'accès aux sports !\n")

		// On renvoie une liste vide, ce qui dans la plupart des cas suffit pour faire croire
		// à la console qu'il n'y a pas d'erreur ou d'achat manquant.
		response := service_item_types.NewServiceItemGetServiceItemRightResponse()

		rmcResponseStream := nex.NewByteStreamOut(globals.SecureEndpoint.LibraryVersions(), globals.SecureEndpoint.ByteStreamSettings())
		response.WriteTo(rmcResponseStream)
		rmcResponseBody := rmcResponseStream.Bytes()

		rmcResponse := nex.NewRMCSuccess(globals.SecureEndpoint, rmcResponseBody)
		rmcResponse.ProtocolID = service_item.ProtocolID
		rmcResponse.MethodID = service_item.MethodGetServiceItemRightRequest
		rmcResponse.CallID = callID

		return rmcResponse, nil
	}
	globals.SecureEndpoint.RegisterServiceProtocol(serviceItemProtocol)
	// -------------------------------------------

	startDashboardAPI()
}

// DashboardData structure JSON retournée par l'API
type DashboardData struct {
	Players  []globals.PlayerInfo  `json:"players"`
	Sessions []globals.SessionInfo `json:"sessions"`
}

func startDashboardAPI() {
	http.HandleFunc("/api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		players := globals.GetAllPlayers()
		sessions := globals.GetAllSessions()

		playerList := make([]globals.PlayerInfo, 0, len(players))
		for _, p := range players {
			playerList = append(playerList, *p)
		}
		sessionList := make([]globals.SessionInfo, 0, len(sessions))
		for _, s := range sessions {
			sessionList = append(sessionList, *s)
		}

		data := DashboardData{
			Players:  playerList,
			Sessions: sessionList,
		}
		json.NewEncoder(w).Encode(data)
	})

	http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		content, err := os.ReadFile("log/all.log")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"` + err.Error() + `"}`))
			return
		}

		lines := strings.Split(string(content), "\n")
		var cleanLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				cleanLines = append(cleanLines, trimmed)
			}
		}

		// Renvoyer les 150 dernières lignes de log
		start := 0
		if len(cleanLines) > 150 {
			start = len(cleanLines) - 150
		}
		lastLines := cleanLines[start:]

		json.NewEncoder(w).Encode(lastLines)
	})

	http.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"restarting"}`))

		fmt.Println("[DASHBOARD] Redémarrage demandé via l'interface web...")
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}()
	})

	fmt.Println("[DASHBOARD] API disponible sur http://127.0.0.1:8080/api/dashboard")
}
