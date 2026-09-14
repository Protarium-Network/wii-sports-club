package main

import (
	"encoding/hex"
	"os"
	"strconv"
	"strings"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	"github.com/PretendoNetwork/plogger-go"
	"github.com/joho/godotenv"

	"github.com/Protarium-Network/wii-sports-club/database"
	"github.com/Protarium-Network/wii-sports-club/globals"
)

func init() {
	globals.Logger = plogger.NewLogger()

	var err error

	err = godotenv.Load()
	if err != nil {
		globals.Logger.Warning("Error loading .env file")
	}

	postgresURI := os.Getenv("PN_POSTGRES_URI")
	kerberosPassword := os.Getenv("PN_KERBEROS_PASSWORD")
	authenticationServerPort := os.Getenv("PN_AUTHENTICATION_SERVER_PORT")
	secureServerHost := os.Getenv("PN_SECURE_SERVER_HOST")
	secureServerPort := os.Getenv("PN_SECURE_SERVER_PORT")
	tokenAESKey := os.Getenv("PN_NEX_TOKEN_AES_KEY")
	passwordSecret := os.Getenv("PN_NEX_PASSWORD_SECRET")

	if strings.TrimSpace(postgresURI) == "" {
		globals.Logger.Error("PN_POSTGRES_URI environment variable not set")
		os.Exit(0)
	}

	if strings.TrimSpace(kerberosPassword) == "" {
		globals.Logger.Warningf("PN_KERBEROS_PASSWORD environment variable not set. Using default password: %q", globals.KerberosPassword)
	} else {
		globals.KerberosPassword = kerberosPassword
	}

	globals.AuthenticationServerAccount = nex.NewAccount(types.NewPID(1), "Quazal Authentication", globals.KerberosPassword)
	globals.SecureServerAccount = nex.NewAccount(types.NewPID(2), "Quazal Rendez-Vous", globals.KerberosPassword)

	globals.NEXTokenAESKey, err = hex.DecodeString(tokenAESKey)
	if err != nil || len(globals.NEXTokenAESKey) != 32 {
		globals.Logger.Critical("PN_NEX_TOKEN_AES_KEY must be a 64-character hexadecimal AES-256 key")
		os.Exit(1)
	}
	globals.NEXPasswordSecret, err = hex.DecodeString(passwordSecret)
	if err != nil || len(globals.NEXPasswordSecret) < 32 {
		globals.Logger.Critical("PN_NEX_PASSWORD_SECRET must contain at least 32 bytes encoded as hexadecimal")
		os.Exit(1)
	}

	if strings.TrimSpace(authenticationServerPort) == "" {
		globals.Logger.Error("PN_AUTHENTICATION_SERVER_PORT environment variable not set")
		os.Exit(0)
	}

	if port, err := strconv.Atoi(authenticationServerPort); err != nil {
		globals.Logger.Errorf("PN_AUTHENTICATION_SERVER_PORT is not a valid port. Expected 0-65535, got %s", authenticationServerPort)
		os.Exit(0)
	} else if port < 0 || port > 65535 {
		globals.Logger.Errorf("PN_AUTHENTICATION_SERVER_PORT is not a valid port. Expected 0-65535, got %s", authenticationServerPort)
		os.Exit(0)
	}

	if strings.TrimSpace(secureServerHost) == "" {
		globals.Logger.Error("PN_SECURE_SERVER_HOST environment variable not set")
		os.Exit(0)
	}

	if strings.TrimSpace(secureServerPort) == "" {
		globals.Logger.Error("PN_SECURE_SERVER_PORT environment variable not set")
		os.Exit(0)
	}

	if port, err := strconv.Atoi(secureServerPort); err != nil {
		globals.Logger.Errorf("PN_SECURE_SERVER_PORT is not a valid port. Expected 0-65535, got %s", secureServerPort)
		os.Exit(0)
	} else if port < 0 || port > 65535 {
		globals.Logger.Errorf("PN_SECURE_SERVER_PORT is not a valid port. Expected 0-65535, got %s", secureServerPort)
		os.Exit(0)
	}

	database.ConnectPostgres()
}
