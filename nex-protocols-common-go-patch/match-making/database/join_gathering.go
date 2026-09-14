package database

import (
	"database/sql"
	"fmt"
	"slices"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	"github.com/PretendoNetwork/nex-protocols-common-go/v2/match-making/tracking"
	match_making "github.com/PretendoNetwork/nex-protocols-go/v2/match-making"
	notifications "github.com/PretendoNetwork/nex-protocols-go/v2/notifications"
	notifications_types "github.com/PretendoNetwork/nex-protocols-go/v2/notifications/types"
	pqextended "github.com/PretendoNetwork/pq-extended"
)

// newParticipantEventCount returns the number represented by one
// NewParticipant notification. It is the size of this join operation, not the
// resulting gathering population.
func newParticipantEventCount(vacantParticipants uint16) uint32 {
	if vacantParticipants == 0 {
		return 1
	}

	return uint32(vacantParticipants)
}

// appendParticipants keeps the gathering owner/host at the beginning of the
// participant array. PIA assigns mesh roles from the established gathering
// order; prepending newcomers can make two consoles disagree about which one
// coordinates the transition from the lobby to the game.
func appendParticipants(existing, joining []uint64) []uint64 {
	total := make([]uint64, 0, len(existing)+len(joining))
	total = append(total, existing...)
	total = append(total, joining...)
	return total
}

// JoinGathering joins participants from the same connection into a gathering. Returns the new number of participants
func JoinGathering(manager *common_globals.MatchmakingManager, gatheringID uint32, connection *nex.PRUDPConnection, vacantParticipants uint16, joinMessage string) (uint32, *nex.Error) {
	// * vacantParticipants represents the total number of participants that are joining (including the main participant)
	// * Prevent underflow below if vacantParticipants is set to zero
	if vacantParticipants == 0 {
		vacantParticipants = 1
	}

	var ownerPID uint64
	var maxParticipants uint32
	var flags uint32
	var participants []uint64
	err := manager.Database.QueryRow(`SELECT owner_pid, max_participants, flags, participants FROM matchmaking.gatherings WHERE id=$1`, gatheringID).Scan(&ownerPID, &maxParticipants, &flags, pqextended.Array(&participants))
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nex.NewError(nex.ResultCodes.RendezVous.SessionVoid, "change_error")
		} else {
			return 0, nex.NewError(nex.ResultCodes.Core.Unknown, err.Error())
		}
	}

	if maxParticipants != 0 {
		if uint32(len(participants))+uint32(vacantParticipants) > maxParticipants {
			return 0, nex.NewError(nex.ResultCodes.RendezVous.SessionFull, "change_error")
		}
	}

	if slices.Contains(participants, uint64(connection.PID())) {
		return 0, nex.NewError(nex.ResultCodes.RendezVous.AlreadyParticipatedGathering, "change_error")
	}

	var newParticipants []uint64

	// * Additional participants are represented by duplicating the main participant PID on the array
	for range vacantParticipants {
		newParticipants = append(newParticipants, uint64(connection.PID()))
	}

	totalParticipants := appendParticipants(participants, newParticipants)

	// * We have already checked that the gathering exists above, so we don't have to check the rows affected on sql.Result
	_, err = manager.Database.Exec(`UPDATE matchmaking.gatherings SET participants=$1 WHERE id=$2`, pqextended.Array(totalParticipants), gatheringID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nex.NewError(nex.ResultCodes.RendezVous.SessionVoid, "change_error")
		} else {
			return 0, nex.NewError(nex.ResultCodes.Core.Unknown, err.Error())
		}
	}

	nexError := tracking.LogJoinGathering(manager.Database, connection.PID(), gatheringID, newParticipants, totalParticipants)
	if nexError != nil {
		return 0, nexError
	}

	// Wii Sports Club keeps a local participant view in addition to the PIA
	// mesh. With the game's normal flags (0x30), the generic NEX behaviour only
	// notifies the owner. That leaves player 2 unaware of player 3/4 and produces
	// different launch state on each console. Notify every participant that was
	// already in the gathering. Do not include the joining PID: its snapshot is
	// sent after the AutoMatchmake response, when its client is ready to process
	// asynchronous notifications.
	participantJoinedTargets := common_globals.RemoveDuplicates(participants)
	if uint64(connection.PID()) == ownerPID {
		return uint32(len(totalParticipants)), nil
	}

	notificationCategory := notifications.NotificationCategories.Participation
	notificationSubtype := notifications.NotificationSubTypes.Participation.NewParticipant

	oEvent := notifications_types.NewNotificationEvent()
	oEvent.PIDSource = connection.PID()
	oEvent.Type = types.NewUInt32(notifications.BuildNotificationType(notificationCategory, notificationSubtype))
	oEvent.Param1 = types.NewUInt32(gatheringID)
	oEvent.Param2 = types.NewUInt32(uint32(connection.PID())) // TODO - This assumes a legacy client. Will not work on the Switch
	oEvent.StrParam = types.NewString(joinMessage)
	// Param3 is the number of participants introduced by this join event, not
	// the resulting gathering population. Nintendo's Wii Sports Club capture
	// sends Param3=1 when the second player joins a two-player gathering.
	oEvent.Param3 = types.NewUInt32(newParticipantEventCount(vacantParticipants))

	fmt.Printf("[NOTIFICATION] NewParticipant GID=%d source=%d participant=%d added=%d total=%d flags=0x%X targets=%v\n",
		gatheringID, connection.PID(), connection.PID(), vacantParticipants, len(totalParticipants), flags, participantJoinedTargets)
	common_globals.SendNotificationEvent(connection.Endpoint().(*nex.PRUDPEndPoint), oEvent, participantJoinedTargets)

	// * VerboseParticipantsEx retains its generic recap behaviour. Wii Sports
	// Club's normal 0x30 flags use the post-response snapshot in the
	// MatchmakeExtension handler instead.
	if flags&match_making.GatheringFlags.VerboseParticipantsEx != 0 {
		// TODO - Should this actually be deduplicated?
		for _, participant := range common_globals.RemoveDuplicates(participants) {
			notificationCategory := notifications.NotificationCategories.Participation
			notificationSubtype := notifications.NotificationSubTypes.Participation.NewParticipant

			oEvent := notifications_types.NewNotificationEvent()
			oEvent.PIDSource = connection.PID()
			oEvent.Type = types.NewUInt32(notifications.BuildNotificationType(notificationCategory, notificationSubtype))
			oEvent.Param1 = types.NewUInt32(gatheringID)
			oEvent.Param2 = types.NewUInt32(uint32(participant)) // TODO - This assumes a legacy client. Will not work on the Switch
			oEvent.StrParam = types.NewString(joinMessage)
			// This recap event describes exactly one existing participant.
			oEvent.Param3 = types.NewUInt32(1)

			// * Send the notification to the joining participant
			fmt.Printf("[NOTIFICATION] ParticipantRecap GID=%d source=%d participant=%d total=%d target=%d\n",
				gatheringID, connection.PID(), participant, len(totalParticipants), connection.PID())
			common_globals.SendNotificationEvent(connection.Endpoint().(*nex.PRUDPEndPoint), oEvent, []uint64{uint64(connection.PID())})
		}
	}

	return uint32(len(totalParticipants)), nil
}
