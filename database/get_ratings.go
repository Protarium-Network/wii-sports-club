package database

import (
	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	datastore "github.com/PretendoNetwork/nex-protocols-go/v2/datastore"
	datastore_types "github.com/PretendoNetwork/nex-protocols-go/v2/datastore/types"
)

// GetRatings is a raw DataStore::GetRatings protocol handler (this method
// has no default implementation in the local nex-protocols-common-go fork,
// unlike RateObject/RateObjects which do). No object rating storage exists
// yet anywhere on this server (InitializeObjectRatingWithSlot is a TODO
// stub too), so this returns a valid DataStoreRatingInfo (Count=1,
// TotalValue=0 - Count=0 makes some clients compute TotalValue/Count for
// display and divide by zero, confirmed via a full console hang on Animal
// Crossing Plaza) per requested data ID instead of NotImplemented - same
// "empty is a valid answer" reasoning as the Rio2016 empty-ranking-category
// fix, rather than hard-failing the whole call. Only wire this up for a
// title if a real capture confirms it actually gets a usable response back
// - real Nintendo servers themselves returned NotImplemented for this call
// on most titles tested (https://tgrcode.com/posts/wiiu_3ds_scraping_leaderboards),
// Mario & Sonic 2014/2016 included, and at least one other title
// (Animal Crossing Plaza) got a client-side 106-0116 BufferOverflow from
// any concrete response tried here, headers on or off.
func GetRatings(err error, packet nex.PacketInterface, callID uint32, dataIDs types.List[types.UInt64], accessPassword types.UInt64) (*nex.RMCMessage, *nex.Error) {
	if err != nil {
		return nil, nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	endpoint := packet.Sender().Endpoint()

	ratings := types.NewList[datastore_types.DataStoreRatingInfo]()
	for range dataIDs {
		rating := datastore_types.NewDataStoreRatingInfo()
		rating.Count = types.NewUInt32(1)
		ratings = append(ratings, rating)
	}

	rmcResponseStream := nex.NewByteStreamOut(endpoint.LibraryVersions(), endpoint.ByteStreamSettings())
	ratings.WriteTo(rmcResponseStream)
	rmcResponseBody := rmcResponseStream.Bytes()

	rmcResponse := nex.NewRMCSuccess(endpoint, rmcResponseBody)
	rmcResponse.ProtocolID = datastore.ProtocolID
	rmcResponse.MethodID = datastore.MethodGetRatings
	rmcResponse.CallID = callID

	return rmcResponse, nil
}
