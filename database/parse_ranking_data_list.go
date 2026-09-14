package database

import (
	"database/sql"
	"time"

	"github.com/PretendoNetwork/nex-go/v2/types"
	ranking_types "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"
)

func parseRankingDataList(rows *sql.Rows) (types.List[ranking_types.RankingRankData], uint32, error) {
	results := types.NewList[ranking_types.RankingRankData]()
	var totalCount uint32

	for rows.Next() {
		result := ranking_types.NewRankingRankData()
		var updateDate int64
		var ownerPID int64
		var uniqueID int64
		var rankingOrder int64
		var category int64
		var score int64
		var groups []byte
		var param int64
		var commonData []byte
		var rowTotal int64

		err := rows.Scan(
			&ownerPID,
			&uniqueID,
			&rankingOrder,
			&category,
			&score,
			&groups,
			&param,
			&commonData,
			&updateDate,
			&rowTotal,
		)
		if err != nil {
			return nil, 0, err
		}

		result.PrincipalID = types.NewPID(uint64(ownerPID))
		result.UniqueID = types.NewUInt64(uint64(uniqueID))
		result.Order = types.NewUInt32(uint32(rankingOrder))
		result.Category = types.NewUInt32(uint32(category))
		result.Score = types.NewUInt32(uint32(score))
		result.Groups = types.NewBuffer(groups)
		result.Param = types.NewUInt64(uint64(param))
		result.CommonData = types.NewBuffer(commonData)
		result.UpdateTime.FromTimestamp(time.Unix(updateDate, 0))
		totalCount = uint32(rowTotal)
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return results, totalCount, nil
}
