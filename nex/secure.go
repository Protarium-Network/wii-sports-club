package nex

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Protarium-Network/wii-sports-club/database"
	"github.com/Protarium-Network/wii-sports-club/globals"
	nex "github.com/PretendoNetwork/nex-go/v2"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
)

func StartSecureServer() {
	globals.SecureServer = nex.NewPRUDPServer()

	globals.SecureEndpoint = nex.NewPRUDPEndPoint(1)
	globals.SecureEndpoint.IsSecureEndPoint = true
	globals.SecureEndpoint.ServerAccount = globals.SecureServerAccount
	globals.SecureEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.SecureEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername
	globals.SecureServer.BindPRUDPEndPoint(globals.SecureEndpoint)
	// globals.SecureServer.ByteStreamSettings.UseStructureHeader = true

	globals.SecureServer.LibraryVersions.SetDefault(nex.NewLibraryVersion(3, 4, 7))
	globals.SecureServer.AccessKey = "4d324052"

	globals.SecureEndpoint.OnData(func(packet nex.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil {
			return
		}

		pid := uint64(packet.Sender().PID())
		fmt.Printf("[Secure] PID=%d %s\n", pid, DescribeMethod(request.ProtocolID, request.MethodID))
	})

	globals.MatchmakingManager = common_globals.NewMatchmakingManager(globals.SecureEndpoint, database.Postgres)

	registerCommonSecureServerProtocols()

	port, _ := strconv.Atoi(os.Getenv("PN_SECURE_SERVER_PORT"))

	globals.SecureServer.Listen(port)
}
