package globals

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"

	"github.com/PretendoNetwork/nex-go/v2/types"
)

func PasswordFromPID(pid types.PID) (string, uint32) {
	if len(NEXPasswordSecret) < 32 {
		return "", 1
	}

	pidBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pidBytes, uint64(pid))
	mac := hmac.New(sha256.New, NEXPasswordSecret)
	_, _ = mac.Write(pidBytes)

	// A stable per-PID password lets the account endpoint and NEX auth server
	// independently derive the same secret without storing credentials.
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), 0
}
