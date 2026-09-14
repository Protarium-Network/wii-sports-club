package database

import (
	"database/sql"

	"github.com/PretendoNetwork/nex-go/v2/types"
)

// GetCommonData returns the requesting player's common data. A zero unique ID
// selects the most recently uploaded record, matching NEX's "own data"
// behaviour while keeping data from different players isolated.
func GetCommonData(pid types.PID, uniqueID types.UInt64) (types.Buffer, error) {
	var data []byte
	var err error

	if uniqueID == 0 {
		err = Postgres.QueryRow(`
			SELECT common_data
			FROM common_datas
			WHERE owner_pid = $1
			ORDER BY updated_at DESC
			LIMIT 1
		`, int64(pid)).Scan(&data)
	} else {
		err = Postgres.QueryRow(`
			SELECT common_data
			FROM common_datas
			WHERE owner_pid = $1 AND unique_id = $2
		`, int64(pid), int64(uniqueID)).Scan(&data)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, err
	}

	return types.NewBuffer(data), nil
}
