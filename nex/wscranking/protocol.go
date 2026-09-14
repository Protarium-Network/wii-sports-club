// Package wscranking implements the Wii Sports Club-specific overload of
// Ranking::GetCachedTopXRankings (protocol 0x70, method 0x0F).
//
// Nintendo's Wii Sports Club server does not use the standard NEX signature
// for this method. It sends one RankingMode, a list of categories, one shared
// RankingOrderParam, a unique ID and a PID, and expects a list of
// RankingResult values. The structures in this one method also carry NEX
// structure headers even though the rest of Wii Sports Club (NEX 3.4.7) does
// not. Both details are confirmed by the official captures in the project.
package wscranking

import (
	"fmt"

	nex "github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	protocolglobals "github.com/PretendoNetwork/nex-protocols-go/v2/globals"
	ranking "github.com/PretendoNetwork/nex-protocols-go/v2/ranking"
	rankingtypes "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"
)

const maxCategoriesPerRequest = 64

// QueryRankingsFunc retrieves one leaderboard. rankingMode values follow the
// standard NEX RankingMode enum (0=global, 4=user).
type QueryRankingsFunc func(
	rankingMode types.UInt8,
	principalID types.PID,
	uniqueID types.UInt64,
	category types.UInt32,
	orderParam rankingtypes.RankingOrderParam,
) (types.List[rankingtypes.RankingRankData], uint32, error)

// Request is the Wii Sports Club method 0x0F request body.
type Request struct {
	RankingMode types.UInt8
	Categories  types.List[types.UInt32]
	OrderParam  rankingtypes.RankingOrderParam
	UniqueID    types.UInt64
	PrincipalID types.PID
}

// Protocol handles the Wii Sports Club-specific Ranking method.
type Protocol struct {
	endpoint      nex.EndpointInterface
	QueryRankings QueryRankingsFunc
}

// NewProtocol returns an initialized Wii Sports Club Ranking protocol patch.
func NewProtocol() *Protocol {
	return &Protocol{}
}

// Endpoint returns the endpoint implementing this protocol patch.
func (protocol *Protocol) Endpoint() nex.EndpointInterface {
	return protocol.endpoint
}

// SetEndpoint sets the endpoint implementing this protocol patch.
func (protocol *Protocol) SetEndpoint(endpoint nex.EndpointInterface) {
	protocol.endpoint = endpoint
}

func structureHeaderSettings(settings *nex.ByteStreamSettings) *nex.ByteStreamSettings {
	copy := nex.NewByteStreamSettings()
	if settings != nil {
		*copy = *settings
	}
	copy.UseStructureHeader = true
	return copy
}

func decodeRequest(parameters []byte, versions *nex.LibraryVersions, settings *nex.ByteStreamSettings) (Request, error) {
	request := Request{
		Categories: types.NewList[types.UInt32](),
		OrderParam: rankingtypes.NewRankingOrderParam(),
	}

	stream := nex.NewByteStreamIn(parameters, versions, structureHeaderSettings(settings))
	if err := request.RankingMode.ExtractFrom(stream); err != nil {
		return request, fmt.Errorf("read ranking mode: %w", err)
	}
	if err := request.Categories.ExtractFrom(stream); err != nil {
		return request, fmt.Errorf("read categories: %w", err)
	}
	if len(request.Categories) == 0 || len(request.Categories) > maxCategoriesPerRequest {
		return request, fmt.Errorf("invalid category count %d", len(request.Categories))
	}
	if err := request.OrderParam.ExtractFrom(stream); err != nil {
		return request, fmt.Errorf("read ranking order parameter: %w", err)
	}
	if err := request.UniqueID.ExtractFrom(stream); err != nil {
		return request, fmt.Errorf("read unique ID: %w", err)
	}
	if err := request.PrincipalID.ExtractFrom(stream); err != nil {
		return request, fmt.Errorf("read principal ID: %w", err)
	}
	if stream.Remaining() != 0 {
		return request, fmt.Errorf("unexpected trailing request data: %d bytes", stream.Remaining())
	}

	return request, nil
}

func encodeResults(results types.List[rankingtypes.RankingResult], versions *nex.LibraryVersions, settings *nex.ByteStreamSettings) []byte {
	stream := nex.NewByteStreamOut(versions, structureHeaderSettings(settings))
	results.WriteTo(stream)
	return stream.Bytes()
}

// HandlePacket handles only method 0x0F. It is installed through the upstream
// Ranking protocol's Patches mechanism, so all other Ranking methods continue
// to use the standard implementation.
func (protocol *Protocol) HandlePacket(packet nex.PacketInterface) {
	message := packet.RMCMessage()
	if message == nil || !message.IsRequest || message.ProtocolID != ranking.ProtocolID || message.MethodID != ranking.MethodGetCachedTopXRankings {
		return
	}

	if protocol.QueryRankings == nil {
		err := nex.NewError(nex.ResultCodes.Core.NotImplemented, "Wii Sports Club ranking query is not configured")
		protocolglobals.RespondError(packet, ranking.ProtocolID, err)
		return
	}

	endpoint := packet.Sender().Endpoint()
	request, err := decodeRequest(message.Parameters, endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	if err != nil {
		protocolglobals.Logger.Errorf("Ranking::GetCachedTopXRankings (WSC): %v", err)
		protocolglobals.RespondError(packet, ranking.ProtocolID, nex.NewError(nex.ResultCodes.Ranking.InvalidArgument, err.Error()))
		return
	}

	principalID := request.PrincipalID
	if principalID == 0 {
		principalID = packet.Sender().PID()
	}

	results := types.NewList[rankingtypes.RankingResult]()
	for _, category := range request.Categories {
		rankData, totalCount, queryErr := protocol.QueryRankings(
			request.RankingMode,
			principalID,
			request.UniqueID,
			category,
			request.OrderParam,
		)
		if queryErr != nil {
			protocolglobals.Logger.Errorf("Ranking::GetCachedTopXRankings (WSC), category %d: %v", uint32(category), queryErr)
			protocolglobals.RespondError(packet, ranking.ProtocolID, nex.NewError(nex.ResultCodes.Ranking.Unknown, queryErr.Error()))
			return
		}

		result := rankingtypes.NewRankingResult()
		result.RankDataList = rankData
		result.TotalCount = types.NewUInt32(totalCount)
		result.SinceTime = types.NewDateTime(0x1F40420000) // 2000-01-01, as sent by Nintendo
		results = append(results, result)
	}

	body := encodeResults(results, endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	response := nex.NewRMCSuccess(endpoint, body)
	response.ProtocolID = ranking.ProtocolID
	response.MethodID = ranking.MethodGetCachedTopXRankings
	response.CallID = message.CallID

	fmt.Printf("[RANKING] WSC multi-query mode=%d PID=%d categories=%v results=%d\n",
		uint8(request.RankingMode), uint64(principalID), request.Categories, len(results))
	protocolglobals.Respond(packet, response)
}
