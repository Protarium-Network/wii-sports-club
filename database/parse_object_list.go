package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Protarium-Network/wii-sports-club/globals"
	nextypes "github.com/PretendoNetwork/nex-go/v2/types"
	"github.com/PretendoNetwork/nex-protocols-go/v2/datastore/types"
	"github.com/lib/pq"
)

func convertPIDList(list *nextypes.List[nextypes.PID]) []uint64 {
	result := make([]uint64, len(*list))

	for i, pid := range *list {
		result[i] = uint64(pid)
	}

	return result
}

func createPIDList(list *[]uint64) nextypes.List[nextypes.PID] {
	result := make([]nextypes.PID, len(*list))

	for i, u := range *list {
		result[i] = nextypes.NewPID(u)
	}

	return result
}

func convertStringList(list *nextypes.List[nextypes.String]) []string {
	result := make([]string, len(*list))

	for i, n := range *list {
		result[i] = n.String()
	}

	return result
}

func createStringList(list *[]string) nextypes.List[nextypes.String] {
	result := make([]nextypes.String, len(*list))

	for i, u := range *list {
		result[i] = nextypes.NewString(u)
	}

	return result
}

func parseObjectList(rows *sql.Rows) (nextypes.List[types.DataStoreMetaInfo], error) {
	results := nextypes.NewList[types.DataStoreMetaInfo]()

	for rows.Next() {
		result := types.NewDataStoreMetaInfo()

		var accesspwd uint64
		var updatepwd uint64
		var uploadCompleted bool
		var deleted bool
		var createdTime time.Time
		var updatedTime time.Time
		var ownerID uint64
		var permissionRecipients []uint64
		var delPermissionRecipients []uint64
		var tags []string
		var extdata []string
		var persistenceSlotID uint64

		result.ExpireTime = nextypes.NewDateTime(0x9C3f3E0000) // * 9999-12-31T00:00:00.000Z. This is what the real server sends

		err := rows.Scan(
			&result.DataID,
			&uploadCompleted,
			&deleted,
			&ownerID,
			&result.Size,
			&result.Name,
			&result.DataType,
			&result.MetaBinary,
			&result.Permission.Permission,
			pq.Array(&permissionRecipients),
			&result.DelPermission.Permission,
			pq.Array(&delPermissionRecipients),
			&result.Flag,
			&result.Period,
			&result.ReferDataID,
			pq.Array(&tags),
			&persistenceSlotID,
			pq.Array(&extdata),
			&accesspwd,
			&updatepwd,
			&createdTime,
			&updatedTime,
		)
		if err != nil {
			fmt.Printf("err line 92")
			globals.Logger.Error(err.Error())
			return nil, err
			//continue
		}

		result.OwnerID = nextypes.NewPID(ownerID)
		result.Permission.RecipientIDs = createPIDList(&permissionRecipients)
		result.DelPermission.RecipientIDs = createPIDList(&delPermissionRecipients)
		result.Tags = createStringList(&tags)
		result.CreatedTime.FromTimestamp(createdTime)
		result.UpdatedTime.FromTimestamp(updatedTime)
		result.ReferredTime.FromTimestamp(createdTime)

		results = append(results, result)
	}

	return results, rows.Err()
}
