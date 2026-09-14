package ranking

import (
	"encoding/hex"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
	ranking "github.com/PretendoNetwork/nex-protocols-go/v2/ranking"
	ranking_types "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"
)

func (commonProtocol *CommonProtocol) getRanking(err error, packet nex.PacketInterface, callID uint32, rankingMode types.UInt8, category types.UInt32, orderParam ranking_types.RankingOrderParam, uniqueID types.UInt64, principalID types.PID) (*nex.RMCMessage, *nex.Error) {
	if commonProtocol.GetRankingsByMode == nil && commonProtocol.GetRankingsAndCountByCategoryAndRankingOrderParam == nil {
		common_globals.Logger.Warning("Ranking::GetRanking missing GetRankingsAndCountByCategoryAndRankingOrderParam!")
		return nil, nex.NewError(nex.ResultCodes.Core.NotImplemented, "change_error")
	}

	if err != nil {
		common_globals.Logger.Error(err.Error())
		return nil, nex.NewError(nex.ResultCodes.Ranking.InvalidArgument, "change_error")
	}

	connection := packet.Sender()
	endpoint := connection.Endpoint()

	callerPID := principalID
	if callerPID == 0 {
		callerPID = connection.PID()
	}

	common_globals.Logger.Infof("[RANKING-DEBUG] GetRanking call: mode=%d category=%d callerPID=%d uniqueID=%d groupIndex=%d groupNum=%d orderCalc=%d offset=%d length=%d",
		uint8(rankingMode), uint32(category), uint64(callerPID), uint64(uniqueID),
		uint8(orderParam.GroupIndex), uint8(orderParam.GroupNum), uint8(orderParam.OrderCalculation), uint32(orderParam.Offset), uint32(orderParam.Length))

	var rankDataList types.List[ranking_types.RankingRankData]
	var totalCount uint32
	if commonProtocol.GetRankingsByMode != nil {
		rankDataList, totalCount, err = commonProtocol.GetRankingsByMode(rankingMode, callerPID, uniqueID, category, orderParam)
	} else {
		rankDataList, totalCount, err = commonProtocol.GetRankingsAndCountByCategoryAndRankingOrderParam(category, orderParam)
	}
	if err != nil {
		common_globals.Logger.Critical("[RANKING-DEBUG] GetRanking error: " + err.Error())
		return nil, nex.NewError(nex.ResultCodes.Ranking.Unknown, "change_error")
	}

	common_globals.Logger.Infof("[RANKING-DEBUG] GetRanking result: mode=%d category=%d totalCount=%d rows=%d", uint8(rankingMode), uint32(category), totalCount, len(rankDataList))

	if totalCount == 0 || len(rankDataList) == 0 {
		common_globals.Logger.Infof("[RANKING-DEBUG] GetRanking empty: mode=%d category=%d pid=%d uniqueID=%d - returning empty result instead of NotFound", uint8(rankingMode), uint32(category), uint64(callerPID), uint64(uniqueID))
		// An empty category (nobody has submitted a score yet) is a normal,
		// expected state - not an error. Returning NotFound here made at
		// least one real client (Mario & Sonic Rio 2016) treat its own
		// first-ever score submission flow as a hard failure, since it
		// queries the leaderboard before/around uploading. A successful
		// empty response matches ordinary "query an empty collection"
		// semantics instead.
		rankDataList = types.NewList[ranking_types.RankingRankData]()
		totalCount = 0
	}

	pResult := ranking_types.NewRankingResult()

	pResult.RankDataList = rankDataList
	pResult.TotalCount = types.NewUInt32(totalCount)
	pResult.SinceTime = types.NewDateTime(0x1F40420000) // * 2000-01-01T00:00:00.000Z, this is what the real server sends back

	rmcResponseStream := nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())

	pResult.WriteTo(rmcResponseStream)

	rmcResponseBody := rmcResponseStream.Bytes()

	common_globals.Logger.Infof("[RANKING-DEBUG] GetRanking RAW response (%d bytes): %s", len(rmcResponseBody), hex.EncodeToString(rmcResponseBody))

	rmcResponse := nex.NewRMCSuccess(endpoint, rmcResponseBody)
	rmcResponse.ProtocolID = ranking.ProtocolID
	rmcResponse.MethodID = ranking.MethodGetRanking
	rmcResponse.CallID = callID

	if commonProtocol.OnAfterGetRanking != nil {
		go commonProtocol.OnAfterGetRanking(packet, rankingMode, category, orderParam, uniqueID, principalID)
	}

	return rmcResponse, nil
}
