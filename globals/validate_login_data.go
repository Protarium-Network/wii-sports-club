package globals

import (
	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	common_globals "github.com/PretendoNetwork/nex-protocols-common-go/v2/globals"
)

func ValidateLoginData(pid types.PID, loginData types.DataHolder) *nex.Error {
	return common_globals.ValidatePretendoLoginData(pid, loginData, NEXTokenAESKey)
}
