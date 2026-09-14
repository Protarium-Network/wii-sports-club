package database

import (
	"fmt"
	"time"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
)

func UpdateObjectPeriodByDataIDWithPassword(dataID types.UInt64, period types.UInt16, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`
		UPDATE datastore.objects SET period = $1, update_date = $2 WHERE data_id = $3
	`, uint16(period), time.Now().UTC(), int64(dataID))
	if err != nil {
		fmt.Printf("UpdateObjectPeriod err: %v\n", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "update error")
	}
	return nil
}

func UpdateObjectMetaBinaryByDataIDWithPassword(dataID types.UInt64, metaBinary types.QBuffer, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`
		UPDATE datastore.objects SET meta_binary = $1, update_date = $2 WHERE data_id = $3
	`, []byte(metaBinary), time.Now().UTC(), int64(dataID))
	if err != nil {
		fmt.Printf("UpdateObjectMetaBinary err: %v\n", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "update error")
	}
	return nil
}

func UpdateObjectDataTypeByDataIDWithPassword(dataID types.UInt64, dataType types.UInt16, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`
		UPDATE datastore.objects SET data_type = $1, update_date = $2 WHERE data_id = $3
	`, uint16(dataType), time.Now().UTC(), int64(dataID))
	if err != nil {
		fmt.Printf("UpdateObjectDataType err: %v\n", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "update error")
	}
	return nil
}
