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

// dataTypeWildcard is the sentinel value Sochi 2014's client sends in
// DataStoreSearchParam.DataType when it isn't filtering by a single type
// (confirmed via a real SearchObject call: DataType=65535 alongside an
// empty DataTypes list, while still filtering hard by Tags).
const dataTypeWildcard = 0xFFFF

func GetObjectInfosByDataStoreSearchParam(param datastore_types.DataStoreSearchParam, pid types.PID) ([]datastore_types.DataStoreMetaInfo, uint32, *nex.Error) {
	var pqErr *pq.Error

	// Real filtering, not just "return everything" - confirmed necessary by
	// a real Sochi 2014 SearchObject call requesting Tags=["NATION239"]
	// (a country-scoped online-rankings browse): returning the 21 unrelated
	// WSC/Rio2016/Puyo objects unfiltered would either show garbage or get
	// silently discarded client-side by tag, either way not what a
	// cross-game shared table search should do.
	//
	// The `game = CurrentGame` clause below is a second, mandatory layer on
	// top of that: even a client-requested filter can be too broad (a
	// wildcard/empty SearchObject, or a tag that happens to also match
	// another game's object) to safely scope on its own. Confirmed the hard
	// way - a real Puyo Puyo Tetris SearchObject call with no filters
	// returned Wii Sports Club's own ranking objects mixed in, and the
	// console crashed trying to parse one as its own data (client-side: a
	// silent freeze, then the Wii U's fatal-exception tone).
	dataTypes := make([]int32, 0, len(param.DataTypes))
	for _, dt := range param.DataTypes {
		dataTypes = append(dataTypes, int32(dt))
	}
	tags := make([]string, 0, len(param.Tags))
	for _, tag := range param.Tags {
		tags = append(tags, tag.String())
	}
	ownerIDs := make([]int64, 0, len(param.OwnerIDs))
	for _, owner := range param.OwnerIDs {
		ownerIDs = append(ownerIDs, int64(owner))
	}

	singleDataType := int32(param.DataType)
	filterBySingleDataType := singleDataType != dataTypeWildcard

	limit := int32(param.ResultRange.Length)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := int32(param.ResultRange.Offset)

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
		WHERE upload_completed = true
			AND deleted = false
			AND (game = $8 OR game IS NULL)
			AND (NOT $1::boolean OR COALESCE(data_type, 0) = $2)
			AND (cardinality($3::int[]) = 0 OR COALESCE(data_type, 0) = ANY($3::int[]))
			AND (cardinality($4::text[]) = 0 OR tags && $4::text[])
			AND (cardinality($5::bigint[]) = 0 OR COALESCE(owner, 0) = ANY($5::bigint[]))
		ORDER BY update_date DESC
		OFFSET $6
		LIMIT $7
	`,
		filterBySingleDataType,
		singleDataType,
		pq.Array(dataTypes),
		pq.Array(tags),
		pq.Array(ownerIDs),
		offset,
		limit,
		CurrentGame,
	)
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &pqErr) && pqErr.SQLState() == "42P01") {
		return nil, 0, nil
	}
	if err != nil {
		fmt.Printf("[SearchObject] query error: %s\n", err.Error())
		return nil, 0, nil
	}

	results, err := parseObjectList(rows)
	if err != nil {
		fmt.Printf("[SearchObject] parse error: %s\n", err.Error())
		return nil, 0, nil
	}

	return results, uint32(len(results)), nil
}
