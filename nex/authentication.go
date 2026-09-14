package nex

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Protarium-Network/wii-sports-club/globals"
	"github.com/PretendoNetwork/nex-go/v2"
)

var serverBuildString string

func StartAuthenticationServer() {
	globals.AuthenticationServer = nex.NewPRUDPServer()

	globals.AuthenticationEndpoint = nex.NewPRUDPEndPoint(1)
	globals.AuthenticationEndpoint.ServerAccount = globals.AuthenticationServerAccount
	globals.AuthenticationEndpoint.AccountDetailsByPID = globals.AccountDetailsByPID
	globals.AuthenticationEndpoint.AccountDetailsByUsername = globals.AccountDetailsByUsername
	globals.AuthenticationServer.BindPRUDPEndPoint(globals.AuthenticationEndpoint)
	// globals.AuthenticationServer.ByteStreamSettings.UseStructureHeader = true

	globals.AuthenticationServer.LibraryVersions.SetDefault(nex.NewLibraryVersion(3, 4, 7))
	globals.AuthenticationServer.AccessKey = "4d324052"

	globals.AuthenticationEndpoint.OnData(func(packet nex.PacketInterface) {
		request := packet.RMCMessage()
		if request == nil {
			return
		}

		pid := uint64(packet.Sender().PID())
		fmt.Printf("[Auth] PID=%d %s\n", pid, DescribeMethod(request.ProtocolID, request.MethodID))
	})

	registerCommonAuthenticationServerProtocols()

	port, _ := strconv.Atoi(os.Getenv("PN_AUTHENTICATION_SERVER_PORT"))

	globals.AuthenticationServer.Listen(port)
}
