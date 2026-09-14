package nex

import "fmt"

// protocolNames maps known NEX protocol IDs to human-readable names, so logs
// don't force the reader to cross-reference the NintendoClients wiki by hand.
var protocolNames = map[uint16]string{
	0x3:  "NATTraversal",
	0xA:  "TicketGranting",
	0xB:  "SecureConnection",
	0x15: "MatchMaking",
	0x32: "MatchMakingExt",
	0x6D: "MatchmakeExtension",
	0x6E: "Utility",
	0x70: "Ranking",
	0x73: "DataStore",
	0x77: "ServiceItem",
}

// methodNames maps [protocolID][methodID] to the method name, for the
// protocols this server actually registers.
var methodNames = map[uint16]map[uint32]string{
	0x3: { // NATTraversal
		0x1: "RequestProbeInitiation",
		0x2: "InitiateProbe",
		0x3: "RequestProbeInitiationExt",
		0x4: "ReportNATTraversalResult",
		0x5: "ReportNATProperties",
		0x6: "GetRelaySignatureKey",
		0x7: "ReportNATTraversalResultDetail",
	},
	0xA: { // TicketGranting
		0x1: "Login",
		0x2: "LoginEx",
		0x3: "RequestTicket",
		0x4: "GetPID",
		0x5: "GetName",
		0x6: "LoginWithContext",
	},
	0xB: { // SecureConnection
		0x1: "Register",
		0x2: "RequestConnectionData",
		0x3: "RequestURLs",
		0x4: "RegisterEx",
		0x5: "TestConnectivity",
		0x6: "UpdateURLs",
		0x7: "ReplaceURL",
		0x8: "SendReport",
	},
	0x15: { // MatchMaking
		0x1:  "RegisterGathering",
		0x2:  "UnregisterGathering",
		0xB:  "Participate",
		0xC:  "CancelParticipation",
		0x29: "GetSessionURLs",
		0x2A: "UpdateSessionHost",
		0x2B: "UpdateGatheringOwnership",
	},
	0x32: { // MatchMakingExt
		0x1: "EndParticipation",
		0x2: "GetParticipants",
		0x3: "GetDetailedParticipants",
		0x4: "GetParticipantsURLs",
		0x5: "GetGatheringRelations",
		0x6: "DeleteFromDeletions",
	},
	0x6D: { // MatchmakeExtension
		0x1:  "CloseParticipation",
		0x2:  "OpenParticipation",
		0x3:  "AutoMatchmakePostpone",
		0x6:  "CreateMatchmakeSession",
		0x7:  "JoinMatchmakeSession",
		0x8:  "ModifyCurrentGameAttribute",
		0xF:  "AutoMatchmakeWithSearchCriteriaPostpone",
		0x11: "CreateCommunity",
		0x1E: "JoinMatchmakeSessionEx",
		0x26: "CreateMatchmakeSessionWithParam",
		0x27: "JoinMatchmakeSessionWithParam",
		0x28: "AutoMatchmakeWithParamPostpone",
	},
	0x6E: { // Utility
		0x1: "AcquireNexUniqueID",
		0x2: "AcquireNexUniqueIDWithPassword",
	},
	0x70: { // Ranking
		0x1: "UploadScore",
		0x2: "DeleteScore",
		0x3: "DeleteAllScores",
		0x4: "UploadCommonData",
		0x5: "DeleteCommonData",
		0x6: "GetCommonData",
		0x7: "ChangeAttributes",
		0x8: "ChangeAllAttributes",
		0x9: "GetRanking",
		0xA: "GetApproxOrder",
		0xB: "GetStats",
		0xC: "GetRankingByPIDList",
		0xD: "GetRankingByUniqueIDList",
		0xE: "GetCachedTopXRanking",
		0xF: "GetCachedTopXRankingsWSC",
	},
	0x73: { // DataStore
		0x4:  "DeleteObject",
		0x8:  "GetMeta",
		0x9:  "GetMetas",
		0xA:  "PrepareUpdateObject",
		0xB:  "CompleteUpdateObject",
		0xC:  "SearchObject",
		0xF:  "RateObject",
		0x15: "PostMetaBinary",
		0x18: "PreparePostObject",
		0x19: "PrepareGetObject",
		0x1A: "CompletePostObject",
	},
	0x77: { // ServiceItem
		0x1: "Hello",
		0x2: "HTTPGetRequest",
		0x3: "HTTPGetResponse",
		0xC: "GetServiceItemRightRequest",
	},
}

// DescribeMethod returns a readable "ProtocolName::MethodName" string for a
// given protocol/method ID pair, falling back to raw hex when unknown.
func DescribeMethod(protocolID uint16, methodID uint32) string {
	protoName, knownProto := protocolNames[protocolID]
	if !knownProto {
		protoName = fmt.Sprintf("Protocol(%#x)", protocolID)
	}

	if methods, ok := methodNames[protocolID]; ok {
		if name, ok := methods[methodID]; ok {
			return fmt.Sprintf("%s::%s", protoName, name)
		}
	}

	return fmt.Sprintf("%s::Method(%#x)", protoName, methodID)
}
