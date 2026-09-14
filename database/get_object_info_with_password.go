package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	datastore_types "github.com/PretendoNetwork/nex-protocols-go/v2/datastore/types"
	"github.com/lib/pq"
)

// GetObjectInfoByDataIDWithPassword is used by DataStore::PrepareGetObject
// (ghost/replay download) when the client requests an object by its DataID.
// A stored access_password of 0 means the object is public - matches every
// object created by this server today (PreparePostObject/PostMetaBinary
// always write access_password=0).
func GetObjectInfoByDataIDWithPassword(dataID types.UInt64, password types.UInt64) (datastore_types.DataStoreMetaInfo, *nex.Error) {
	var storedPassword int64

	err := Postgres.QueryRow(`SELECT access_password FROM datastore.objects WHERE data_id = $1`, int64(dataID)).Scan(&storedPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		fmt.Printf("Error checking datastore object password: %v\n", err)
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	if storedPassword != 0 && storedPassword != int64(password) {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.PermissionDenied, "wrong password")
	}

	return GetObjectInfoByDataID(dataID)
}

// GetObjectInfoByPersistenceTargetWithPassword is used by
// DataStore::PrepareGetObject when the client requests an object by owner +
// persistence slot instead of a raw DataID (DataID == 0 in the request).
// Scoped to CurrentGame for the same reason SearchObject is (see that
// file's doc comment): a persistence-slot lookup, unlike a direct DataID
// lookup, returns an object the client didn't ask for by ID, so it must
// not be able to resolve to another game's row.
func GetObjectInfoByPersistenceTargetWithPassword(persistenceTarget datastore_types.DataStorePersistenceTarget, password types.UInt64) (datastore_types.DataStoreMetaInfo, *nex.Error) {
	var pqErr *pq.Error

	// Same NULL-safe explicit column list as GetObjectInfoByDataID/
	// GetObjectInfosByDataStoreSearchParam.
	rows, err := Postgres.Query(`
		SELECT
			data_id,
			upload_completed,
			deleted,
			COALESCE(owner, 0),
			COALESCE(size, 0),
			COALESCE(name, ''),
			COALESCE(data_type, 0),
			meta_binary,
			COALESCE(permission, 0),
			permission_recipients,
			COALESCE(delete_permission, 0),
			delete_permission_recipients,
			COALESCE(flag, 0),
			COALESCE(period, 0),
			COALESCE(refer_data_id, 0),
			tags,
			COALESCE(persistence_slot_id, 0),
			extra_data,
			access_password,
			update_password,
			COALESCE(creation_date, to_timestamp(0)),
			COALESCE(update_date, to_timestamp(0))
		FROM datastore.objects
		WHERE owner = $1 AND persistence_slot_id = $2 AND deleted = false
			AND (game = $3 OR game IS NULL)
		ORDER BY update_date DESC
		LIMIT 1
	`,
		int64(persistenceTarget.OwnerID),
		int64(persistenceTarget.PersistenceSlotID),
		CurrentGame,
	)
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &pqErr) && pqErr.SQLState() == "42P01") {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		fmt.Printf("Error selecting datastore object by persistence target: %v\n", err)
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	defer rows.Close()

	results, err := parseObjectList(rows)
	if err != nil || len(results) == 0 {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}

	return results[0], nil
}
