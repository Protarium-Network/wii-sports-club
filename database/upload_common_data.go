package database

import (
	"time"

	"github.com/PretendoNetwork/nex-go/v2/types"
)

func UploadCommonData(pid types.PID, uniqueID types.UInt64, commonData types.Buffer) error {
	now := time.Now().Unix()
	_, err := Postgres.Exec(`
		INSERT INTO common_datas (common_data, unique_id, owner_pid, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (unique_id, owner_pid) DO UPDATE SET
			common_data = EXCLUDED.common_data,
			updated_at = EXCLUDED.updated_at
	`,
		[]byte(commonData),
		int64(uniqueID),
		int64(pid),
		now,
	)
	return err
}
