package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
)

// These four callbacks are required by DataStore::CompletePostObject (the
// last step of the PreparePostObject/upload-to-S3/CompletePostObject score-
// attachment flow) - confirmed missing via a real Sochi 2014 score upload
// ("GetObjectOwnerByDataID not defined"). Nothing wired them before because
// no other title on this server had exercised this exact call.

func GetObjectOwnerByDataID(dataID types.UInt64) (uint32, *nex.Error) {
	var owner int64
	err := Postgres.QueryRow(`SELECT COALESCE(owner, 0) FROM datastore.objects WHERE data_id = $1`, int64(dataID)).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		fmt.Printf("Error selecting datastore object owner: %v\n", err)
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	return uint32(owner), nil
}

func GetObjectSizeByDataID(dataID types.UInt64) (uint32, *nex.Error) {
	var size int64
	err := Postgres.QueryRow(`SELECT COALESCE(size, 0) FROM datastore.objects WHERE data_id = $1`, int64(dataID)).Scan(&size)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		fmt.Printf("Error selecting datastore object size: %v\n", err)
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	return uint32(size), nil
}

func UpdateObjectUploadCompletedByDataID(dataID types.UInt64, uploadCompleted bool) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET upload_completed = $1 WHERE data_id = $2`, uploadCompleted, int64(dataID))
	if err != nil {
		fmt.Printf("Error updating datastore object upload_completed: %v\n", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	return nil
}

func DeleteObjectByDataID(dataID types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET deleted = true WHERE data_id = $1`, int64(dataID))
	if err != nil {
		fmt.Printf("Error deleting datastore object: %v\n", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	return nil
}
