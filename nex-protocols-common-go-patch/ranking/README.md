# Ranking Patch-01

Modification of the Ranking system to add `GetRankingByMode` to the `CommonProtocol` interface.
```go
type CommonProtocol struct {
...
GetRankingsByMode                                 func(rankingMode types.UInt8, principalID types.PID, uniqueID types.UInt64, category types.UInt32, rankingOrderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error)
}
```
Also added the `GetRankingsByMode` function to the `GetRanking` method; if defined (taking the mode, player, and unique identifier as arguments), it retrieves scores for a specific ranking—otherwise, it falls back to using `GetRankingsAndCountByCategoryAndRankingOrderParam`, based on the category and ordering parameters.
```go
...
if commonProtocol.GetRankingsByMode != nil {
	rankDataList, totalCount, err = commonProtocol.GetRankingsByMode(rankingMode, callerPID, uniqueID, category, orderParam)
} else {
	rankDataList, totalCount, err = commonProtocol.GetRankingsAndCountByCategoryAndRankingOrderParam(category, orderParam)
}
```
And, Modification to `totalCount`: If `totalCount == 0` or `len(rankDataList) == 0`, the function now returns an empty list and sets `totalCount = 0`, rather than throwing a `NotFound` error.
Returning `NotFound` for an empty leaderboard caused the initial score submission to crash or hang in certain games (e.g., *Mario & Sonic at the Rio 2016 Olympic Games*).
