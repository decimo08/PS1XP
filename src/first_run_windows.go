package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lxn/walk"
)

func prepareEngineResources(dir string) (bool, error) {
	if resourcesReady(dir) {
		return true, nil
	}

	message := "PS1XP needs three boot resources from the official PSXPackager v1.8.0 ZIP.\r\n\r\n" +
		"Yes: download and import the ZIP from GitHub.\r\n" +
		"No: select an official ZIP you already downloaded."
	switch walk.MsgBox(nil, "One-time PSXPackager setup", message, walk.MsgBoxYesNoCancel|walk.MsgBoxIconInformation) {
	case walk.DlgCmdYes:
		client := http.Client{Timeout: 35 * time.Second}
		response, err := client.Get(upstreamResourceZip)
		if err == nil {
			if response.StatusCode == http.StatusOK {
				err = importEngineResources(dir, response.Body)
			} else {
				err = fmt.Errorf("GitHub returned %s", response.Status)
			}
			response.Body.Close()
		}
		if err == nil {
			return true, nil
		}
		walk.MsgBox(nil, "Download failed", fmt.Sprintf("%s\r\n\r\nSelect the official v1.8.0 ZIP if you have it.", err), walk.MsgBoxIconWarning)
	case walk.DlgCmdNo:
	default:
		return false, nil
	}

	dialog := walk.FileDialog{Title: "Select the official PSXPackager v1.8.0 ZIP", Filter: "ZIP archive (*.zip)|*.zip"}
	opened, err := dialog.ShowOpen(nil)
	if err != nil || !opened {
		return false, err
	}
	file, err := os.Open(dialog.FilePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := importEngineResources(dir, file); err != nil {
		return false, err
	}
	return true, nil
}
