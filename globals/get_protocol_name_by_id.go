package globals

// Basic ProtocolID
var protocolNames = map[uint16]string{
	0x7B: "AA-User",
	0x19: "Account-Management",
	0x73: "Datastore",
	0x14: "Friends",
	0x70: "Ranking",
	0x0B: "Secure-Connection",
	0x0A: "Ticket-Granting",
	0x03: "NAT-Traversal",
	0x15: "Matchmaking",
	0x32: "Matchmaking-Ext",
	0x6D: "Matchmaking-Extension",
	0x6E: "Store-Manager",
}

func GetProtocolNameByID(protocolID uint16) string {
	if name, ok := protocolNames[protocolID]; ok {
		return name
	}
	return "Unknown"
}
