package matchmake_extension

import (
	"fmt"
	"time"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	notifications "github.com/PretendoNetwork/nex-protocols-go/v2/notifications"
	notifications_types "github.com/PretendoNetwork/nex-protocols-go/v2/notifications/types"
	pqextended "github.com/PretendoNetwork/pq-extended"
)

const participantSnapshotDelay = 350 * time.Millisecond

// sendParticipantSnapshotAfterResponse tells a newly joined Wii Sports Club
// client which opponents were already present. The delay is intentional: the
// game's state machine first consumes AutoMatchmake's GatheringHolder response
// and only then accepts Participation::NewParticipant notifications.
func (commonProtocol *CommonProtocol) sendParticipantSnapshotAfterResponse(endpoint *nex.PRUDPEndPoint, gatheringID uint32, joiningPID uint64, joinMessage string) {
	time.Sleep(participantSnapshotDelay)

	var participants []uint64
	err := commonProtocol.manager.Database.QueryRow(
		`SELECT participants FROM matchmaking.gatherings WHERE id=$1`,
		gatheringID,
	).Scan(pqextended.Array(&participants))
	if err != nil {
		fmt.Printf("[MATCH-SYNC] GID=%d target=%d snapshot failed: %v\n", gatheringID, joiningPID, err)
		return
	}

	uniqueParticipants := common_globals.RemoveDuplicates(participants)
	for _, participantPID := range uniqueParticipants {
		if participantPID == joiningPID {
			continue
		}

		event := newParticipantSnapshotEvent(gatheringID, joiningPID, participantPID, joinMessage)
		fmt.Printf("[MATCH-SYNC] GID=%d source=%d target=%d existing-participant=%d type=0x%X param3=1\n",
			gatheringID, joiningPID, joiningPID, participantPID, uint32(event.Type))
		common_globals.SendNotificationEvent(endpoint, event, []uint64{joiningPID})
	}

	fmt.Printf("[MATCH-SYNC] GID=%d synchronized target=%d participants=%v\n",
		gatheringID, joiningPID, uniqueParticipants)
}

func newParticipantSnapshotEvent(gatheringID uint32, joiningPID, participantPID uint64, joinMessage string) notifications_types.NotificationEvent {
	event := notifications_types.NewNotificationEvent()
	// Match Pretendo's VerboseParticipantsEx recap semantics: the source is the
	// participant performing the join, while Param2 identifies the established
	// participant being described.
	event.PIDSource = types.NewPID(joiningPID)
	event.Type = types.NewUInt32(notifications.BuildNotificationType(
		notifications.NotificationCategories.Participation,
		notifications.NotificationSubTypes.Participation.NewParticipant,
	))
	event.Param1 = types.NewUInt32(gatheringID)
	event.Param2 = types.NewUInt32(uint32(participantPID))
	event.Param3 = types.NewUInt32(1)
	event.StrParam = types.NewString(joinMessage)
	return event
}
