package nex

import (
	"fmt"
	"strconv"

	nex "github.com/PretendoNetwork/nex-go/v2"
	nex_constants "github.com/PretendoNetwork/nex-go/v2/constants"
	nex_types "github.com/PretendoNetwork/nex-go/v2/types"
	common_nat_traversal "github.com/PretendoNetwork/nex-protocols-common-go/v2/nat-traversal"
	nat_traversal "github.com/PretendoNetwork/nex-protocols-go/v2/nat-traversal"
	"github.com/Protarium-Network/wii-sports-club/globals"
)

// Traversée NAT : permet aux consoles de se connecter directement entre elles (P2P).
// On garde les handlers Pretendo d'origine et on ajoute des logs et des corrections.

func registerNATTraversal() {
	protocol := nat_traversal.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	common_nat_traversal.NewCommonProtocol(protocol)

	// À sauvegarder AVANT de remplacer les handlers.
	originalResultHandler := protocol.ReportNATTraversalResult
	originalPropertiesHandler := protocol.ReportNATProperties

	protocol.SetHandlerRequestProbeInitiationExt(handleRequestProbeInitiationExt)

	protocol.SetHandlerReportNATTraversalResult(func(
		err error,
		packet nex.PacketInterface,
		callID uint32,
		cid nex_types.UInt32,
		result nex_types.Bool,
		rtt nex_types.UInt32,
	) (*nex.RMCMessage, *nex.Error) {
		logNATTraversalResult(packet, cid, result, rtt)

		if originalResultHandler != nil {
			return originalResultHandler(err, packet, callID, cid, result, rtt)
		}
		return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "ReportNATTraversalResult not implemented")
	})

	protocol.SetHandlerReportNATProperties(func(
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

		if originalPropertiesHandler == nil {
			return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "ReportNATProperties not implemented")
		}

		response, nexErr := originalPropertiesHandler(err, packet, callID, natmapping, natfiltering, rtt)
		// On force le mapping/filtrage sur TOUTES les StationURLs (publiques comprises),
		// après le passage de Pretendo.
		for _, station := range connection.StationURLs {
			station.SetNATMapping(nex_constants.NATMappingProperties(natmapping))
			station.SetNATFiltering(nex_constants.NATFilteringProperties(natfiltering))
		}
		return response, nexErr
	})
}

// handleRequestProbeInitiationExt : une console demande à "sonder" d'autres joueurs.
// On met à jour son adresse P2P, puis on prévient chaque cible avec un InitiateProbe.
func handleRequestProbeInitiationExt(
	err error,
	packet nex.PacketInterface,
	callID uint32,
	targetList nex_types.List[nex_types.String],
	stationToProbe nex_types.String,
) (*nex.RMCMessage, *nex.Error) {
	senderConn := packet.Sender().(*nex.PRUDPConnection)
	senderPID := uint64(senderConn.PID())

	fmt.Printf("[NAT] RequestProbeInitiationExt de PID=%d (RVCID=%d)\n", senderPID, uint32(senderConn.ID))
	fmt.Printf("[NAT]   stationToProbe = %s\n", string(stationToProbe))
	for i, target := range targetList {
		fmt.Printf("[NAT]   targetList[%d] = %s\n", i, string(target))
	}

	probeURL := nex_types.NewStationURL(stationToProbe)
	setProbeStationURL(senderConn, probeURL)

	// Numéro du joueur dans la partie (uniquement pour les logs).
	senderCID := 1
	if _, participants, dbErr := gatheringOfPlayer(senderPID); dbErr == nil {
		for idx, p := range participants {
			if p == senderPID {
				senderCID = idx + 1
				break
			}
		}
	}

	for _, target := range targetList {
		targetConn := findConnectionByStationURL(nex_types.NewStationURL(target))
		if targetConn == nil {
			fmt.Printf("[NAT] ⚠️ Client introuvable pour target = %s\n", string(target))
			continue
		}

		fmt.Printf("[NAT] 🚀 InitiateProbe: Sender PID=%d (CID=%d) → Target PID=%d (RVCID=%d, Station: %s)\n",
			senderPID, senderCID, uint64(targetConn.PID()), targetConn.ID, probeURL.URL())
		sendInitiateProbe(targetConn, probeURL, callID)
	}

	return newEmptyResponse(nat_traversal.ProtocolID, nat_traversal.MethodRequestProbeInitiationExt, callID), nil
}

// setProbeStationURL enregistre l'adresse P2P réelle de la console.
//
// La console ouvre un NOUVEAU socket local pour le sondage NAT (port différent de celui
// du Register) et ne donne ce port qu'ici, via stationToProbe. Sans cette mise à jour,
// GetSessionURLs renverrait toujours l'ancien port et le P2P échouerait à chaque fois.
func setProbeStationURL(conn *nex.PRUDPConnection, probeURL nex_types.StationURL) {
	url := probeURL.Copy().(nex_types.StationURL)
	if len(conn.StationURLs) < 2 {
		conn.StationURLs = append(conn.StationURLs, url)
	} else {
		conn.StationURLs[1] = url
	}
}

// findConnectionByStationURL retrouve la connexion d'un joueur à partir du PID de sa StationURL.
// (Attention : RVCID != CID PRUDP, on cherche donc bien par PID.)
func findConnectionByStationURL(station nex_types.StationURL) *nex.PRUDPConnection {
	pidStr, _ := station.ParamValue("PID")
	if pidStr == "" {
		return nil
	}
	pid, _ := strconv.ParseUint(pidStr, 10, 64)
	return globals.SecureEndpoint.FindConnectionByPID(pid)
}

// sendInitiateProbe envoie à targetConn l'ordre de contacter probeURL.
func sendInitiateProbe(targetConn *nex.PRUDPConnection, probeURL nex_types.StationURL, callID uint32) {
	stream := nex.NewByteStreamOut(targetConn.Endpoint().LibraryVersions(), targetConn.Endpoint().ByteStreamSettings())
	probeURL.WriteTo(stream)

	request := nex.NewRMCRequest(globals.SecureEndpoint)
	request.ProtocolID = nat_traversal.ProtocolID
	request.CallID = 0xFFFF0000 + callID
	request.MethodID = nat_traversal.MethodInitiateProbe
	request.Parameters = stream.Bytes()

	server := globals.SecureEndpoint.Server
	var packet nex.PRUDPPacketInterface
	if targetConn.DefaultPRUDPVersion == 0 {
		packet, _ = nex.NewPRUDPPacketV0(server, targetConn, nil)
	} else {
		packet, _ = nex.NewPRUDPPacketV1(server, targetConn, nil)
	}
	packet.SetType(nex_constants.DataPacket)
	packet.AddFlag(nex_constants.PacketFlagNeedsAck)
	packet.AddFlag(nex_constants.PacketFlagReliable)
	packet.SetSourceVirtualPortStreamType(targetConn.StreamType)
	packet.SetSourceVirtualPortStreamID(globals.SecureEndpoint.StreamID)
	packet.SetDestinationVirtualPortStreamType(targetConn.StreamType)
	packet.SetDestinationVirtualPortStreamID(targetConn.StreamID)
	packet.SetPayload(request.Bytes())

	server.Send(packet)
}

// logNATTraversalResult affiche le résultat du P2P ; en cas d'échec, donne du contexte.
func logNATTraversalResult(packet nex.PacketInterface, cid nex_types.UInt32, result nex_types.Bool, rtt nex_types.UInt32) {
	senderPID := uint64(packet.Sender().PID())

	if bool(result) {
		fmt.Printf("[NAT] ✓ P2P OK pour PID=%d (CID=%d, RTT=%dms)\n", senderPID, uint32(cid), uint32(rtt))
		return
	}

	// Un échec entre DEUX joueurs casse le mesh complet exigé par la console,
	// ce qui la ramène généralement au menu pour tout le monde.
	gid, participants, _ := gatheringOfPlayer(senderPID)
	var others []uint64
	for _, p := range participants {
		if p != senderPID {
			others = append(others, p)
		}
	}

	fmt.Printf("[NAT] ⚠️  ÉCHEC P2P pour PID=%d (CID=%d, GID=%d, autres joueurs de la session=%v)\n",
		senderPID, uint32(cid), gid, others)
	fmt.Printf("[NAT]     → Probable incompatibilité NAT entre les routeurs des joueurs concernés.\n")
	fmt.Printf("[NAT]     → Ce n'est pas un bug serveur : la connexion directe (P2P) entre CES DEUX consoles a échoué.\n")
	fmt.Printf("[NAT]     → Conséquence typique : la partie ne se lance pas correctement pour tout le monde (retour au menu),\n")
	fmt.Printf("[NAT]       car la console exige une connexion P2P complète entre TOUS les joueurs, pas seulement avec l'hôte.\n")
}
