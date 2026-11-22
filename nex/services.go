package nex

import (
	"fmt"

	nex "github.com/PretendoNetwork/nex-go/v2"
	common_datastore "github.com/PretendoNetwork/nex-protocols-common-go/v2/datastore"
	common_ranking "github.com/PretendoNetwork/nex-protocols-common-go/v2/ranking"
	common_utility "github.com/PretendoNetwork/nex-protocols-common-go/v2/utility"
	datastore "github.com/PretendoNetwork/nex-protocols-go/v2/datastore"
	ranking "github.com/PretendoNetwork/nex-protocols-go/v2/ranking"
	service_item "github.com/PretendoNetwork/nex-protocols-go/v2/service-item/wii-sports-club"
	service_item_types "github.com/PretendoNetwork/nex-protocols-go/v2/service-item/wii-sports-club/types"
	utility "github.com/PretendoNetwork/nex-protocols-go/v2/utility"
	"github.com/Protarium-Network/wii-sports-club/database"
	"github.com/Protarium-Network/wii-sports-club/globals"
	wscranking "github.com/Protarium-Network/wii-sports-club/nex/wscranking"
)

// Petits services : utility, datastore, classements, service item.

func registerUtility() {
	protocol := utility.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	common := common_utility.NewCommonProtocol(protocol)
	common.GenerateNEXUniqueID = GenerateNEXUniqueID
}

func registerDatastore() {
	protocol := datastore.NewProtocol()
	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
	common := common_datastore.NewCommonProtocol(protocol)
	common.GetObjectInfosByDataStoreSearchParam = database.GetObjectInfosByDataStoreSearchParam
	common.InitializeObjectByPreparePostParam = database.InitializeObjectByPreparePostParam
	common.InitializeObjectRatingWithSlot = database.InitializeObjectRatingWithSlot
	common.GetObjectInfoByDataID = database.GetObjectInfoByDataID
	common.UpdateObjectPeriodByDataIDWithPassword = database.UpdateObjectPeriodByDataIDWithPassword
	common.UpdateObjectMetaBinaryByDataIDWithPassword = database.UpdateObjectMetaBinaryByDataIDWithPassword
	common.UpdateObjectDataTypeByDataIDWithPassword = database.UpdateObjectDataTypeByDataIDWithPassword
}

// registerRanking branche les classements, avec le patch Wii Sports Club
// qui remplace GetCachedTopXRankings.
func registerRanking() {
	protocol := ranking.NewProtocol()

	wscProtocol := wscranking.NewProtocol()
	wscProtocol.QueryRankings = database.GetRankings
	wscProtocol.SetEndpoint(globals.SecureEndpoint)
	protocol.Patches = wscProtocol
	protocol.PatchedMethods = []uint32{ranking.MethodGetCachedTopXRankings}

	globals.SecureEndpoint.RegisterServiceProtocol(protocol)

	common := common_ranking.NewCommonProtocol(protocol)
	common.GetRankingsAndCountByCategoryAndRankingOrderParam = database.GetRankingsAndCountByCategoryAndRankingOrderParam
	common.GetRankingsByMode = database.GetRankings
	common.GetCommonData = database.GetCommonData
	common.UploadCommonData = database.UploadCommonData
	common.InsertRankingByPIDAndRankingScoreData = database.InsertRankingByPIDAndRankingScoreData
}

// registerServiceItem : faux "service item". On renvoie une liste vide, ce qui suffit
// à faire croire à la console qu'il n'y a pas d'achat manquant (accès aux sports).
func registerServiceItem() {
	protocol := service_item.NewProtocol(globals.SecureEndpoint)

	protocol.GetServiceItemRightRequest = func(err error, packet nex.PacketInterface, callID uint32, param service_item_types.ServiceItemGetServiceItemRightParam) (*nex.RMCMessage, *nex.Error) {
		fmt.Printf("[SERVICE ITEM] GetServiceItemRightRequest appelé pour autoriser l'accès aux sports !\n")

		stream := nex.NewByteStreamOut(globals.SecureEndpoint.LibraryVersions(), globals.SecureEndpoint.ByteStreamSettings())
		service_item_types.NewServiceItemGetServiceItemRightResponse().WriteTo(stream)

		response := nex.NewRMCSuccess(globals.SecureEndpoint, stream.Bytes())
		response.ProtocolID = service_item.ProtocolID
		response.MethodID = service_item.MethodGetServiceItemRightRequest
		response.CallID = callID
		return response, nil
	}

	globals.SecureEndpoint.RegisterServiceProtocol(protocol)
}
