package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PretendoNetwork/nex-go/v2/types"
)

func CreateReportDBRecord(pid types.PID, reportID types.UInt32, reportData types.QBuffer) error {
	// Créer le dossier "reports" s'il n'existe pas
	reportsDir := "reports"
	if err := os.MkdirAll(reportsDir, os.ModePerm); err != nil {
		fmt.Printf("Erreur lors de la création du dossier reports: %v\n", err)
		return err
	}

	// Créer un nom de fichier unique basé sur le PID, l'ID du rapport et la date
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("report_pid%d_id%d_%s.bin", pid, reportID, timestamp)
	filepath := filepath.Join(reportsDir, filename)

	// Écrire les données brutes dans le fichier
	if err := os.WriteFile(filepath, []byte(reportData), 0644); err != nil {
		fmt.Printf("Erreur lors de la sauvegarde du rapport: %v\n", err)
		return err
	}

	fmt.Printf("[INFO] Nouveau rapport de la console sauvegardé sur le PC : %s\n", filepath)
	return nil
}
