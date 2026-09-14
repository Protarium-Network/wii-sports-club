package database

import (
	"fmt"

	"github.com/PretendoNetwork/nex-go/v2/types"
	ranking_constants "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/constants"
	ranking_types "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"
)

const (
	rankingModeRange = 0
	rankingModeUser  = 4
	maxRankingRows   = 1000
)

func GetRankingsAndCountByCategoryAndRankingOrderParam(category types.UInt32, rankingOrderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	return GetRankings(
		types.NewUInt8(rankingModeRange),
		types.NewPID(0),
		types.NewUInt64(0),
		category,
		rankingOrderParam,
	)
}

// GetRankings retrieves a correctly filtered and ranked leaderboard. Wii
// Sports Club has only been observed using global (0) and user (4) modes for
// its multi-category request; unsupported modes are rejected instead of
// silently leaking a global leaderboard.
func GetRankings(
	rankingMode types.UInt8,
	principalID types.PID,
	uniqueID types.UInt64,
	category types.UInt32,
	rankingOrderParam ranking_types.RankingOrderParam,
) (types.List[ranking_types.RankingRankData], uint32, error) {
	mode := uint8(rankingMode)
	if mode != rankingModeRange && mode != rankingModeUser {
		return nil, 0, fmt.Errorf("ranking mode %d is not supported", mode)
	}

	groupIndex := uint8(rankingOrderParam.GroupIndex)
	if groupIndex != uint8(ranking_constants.FilterGroupIndexNone) && groupIndex > 3 {
		return nil, 0, fmt.Errorf("invalid ranking group index %d", groupIndex)
	}

	orderCalculation := uint8(rankingOrderParam.OrderCalculation)
	if orderCalculation > 1 {
		return nil, 0, fmt.Errorf("invalid ranking order calculation %d", orderCalculation)
	}

	limit := uint32(rankingOrderParam.Length)
	if limit == 0 {
		// User-mode requests in the official Wii Sports Club captures use a
		// length of zero. In that mode it means "all of this user's rows".
		limit = maxRankingRows
	}
	if limit > maxRankingRows {
		limit = maxRankingRows
	}

	offset := uint32(rankingOrderParam.Offset)
	if mode == rankingModeUser {
		offset = 0
	}

	rows, err := Postgres.Query(`
		WITH base AS (
			SELECT
				r.owner_pid,
				r.unique_id,
				r.category,
				r.score,
				r.groups,
				r.param,
				r.updated_at,
				COALESCE(cd.common_data, ''::bytea) AS common_data,
				COALESCE(rc.order_by, r.order_by, 1) AS category_order
			FROM rankings AS r
			LEFT JOIN ranking_categories AS rc
				ON rc.category = r.category
			LEFT JOIN common_datas AS cd
				ON cd.owner_pid = r.owner_pid AND cd.unique_id = r.unique_id
			WHERE r.category = $1
				AND CASE
					WHEN $2::integer = 255 THEN TRUE
					ELSE octet_length(r.groups) > $2::integer
						AND get_byte(r.groups, $2::integer) = $3::integer
				END
		), ranked AS (
			SELECT
				base.*,
				CASE
					WHEN $4::integer = 0 THEN RANK() OVER (
						ORDER BY
							CASE WHEN category_order = 1 THEN score END DESC,
							CASE WHEN category_order = 0 THEN score END ASC
					)
					ELSE ROW_NUMBER() OVER (
						ORDER BY
							CASE WHEN category_order = 1 THEN score END DESC,
							CASE WHEN category_order = 0 THEN score END ASC,
							updated_at ASC,
							owner_pid ASC,
							unique_id ASC
					)
				END AS ranking_order
			FROM base
		), selected AS (
			SELECT ranked.*, COUNT(*) OVER () AS total_count
			FROM ranked
			WHERE $5::integer = 0
				OR (
					$5::integer = 4
					AND owner_pid = $6
					AND ($7::bigint = 0 OR unique_id = $7)
				)
		)
		SELECT
			owner_pid,
			unique_id,
			ranking_order,
			category,
			score,
			groups,
			param,
			common_data,
			updated_at,
			total_count
		FROM selected
		ORDER BY ranking_order, owner_pid, unique_id
		OFFSET $8
		LIMIT $9
	`,
		int64(category),
		int(groupIndex),
		int(uint8(rankingOrderParam.GroupNum)),
		int(orderCalculation),
		int(mode),
		int64(principalID),
		int64(uniqueID),
		int64(offset),
		int64(limit),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query category %d: %w", uint32(category), err)
	}
	defer rows.Close()

	results, totalCount, err := parseRankingDataList(rows)
	if err != nil {
		return nil, 0, fmt.Errorf("parse category %d: %w", uint32(category), err)
	}

	return results, totalCount, nil
}
