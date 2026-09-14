package database

import (
	"time"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	datastore_types "github.com/PretendoNetwork/nex-protocols-go/v2/datastore/types"
	"github.com/lib/pq"
)

func InitializeObjectByPreparePostParam(ownerPID types.PID, param datastore_types.DataStorePreparePostParam) (uint64, *nex.Error) {
	// we still need to figure this out
	// globals.Logger.Info(param.FormatToString(1))
	now := time.Now()

	var dataID uint64

	err := Postgres.QueryRow(`
		INSERT INTO datastore.objects (
		upload_completed,
		deleted,
		owner,
		size,
		name,
		data_type,
		meta_binary,
		permission,
		permission_recipients,
		delete_permission,
		delete_permission_recipients,
		flag,
		period,
		refer_data_id,
		tags,
		persistence_slot_id,
		extra_data,
		access_password,
		update_password,
		creation_date,
		update_date,
		game
		)
		VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22) RETURNING data_id
	`,
		false,
		false,
		ownerPID,
		param.Size,
		param.Name,
		param.DataType,
		param.MetaBinary,
		param.Permission.Permission,
		pq.Array(convertPIDList(&param.Permission.RecipientIDs)),
		param.DelPermission.Permission,
		pq.Array(convertPIDList(&param.DelPermission.RecipientIDs)),
		param.Flag,
		param.Period,
		param.ReferDataID,
		pq.Array(convertStringList(&param.Tags)),
		param.PersistenceInitParam.PersistenceSlotID,
		pq.Array(convertStringList(&param.ExtraData)),
		0,
		0,
		now,
		now,
		CurrentGame,
	).Scan(&dataID)
	if err != nil {
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "insert error")
	}

	return dataID, nil
}
