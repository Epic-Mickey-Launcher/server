package mod

import (
	"bytes"
	"emlserver/archive"
	"emlserver/config"
	"emlserver/database"
	"emlserver/ffmpeg"
	"emlserver/helper"
	"emlserver/message"
	"emlserver/security"
	"emlserver/structs"
	"emlserver/ticket"
	"emlserver/tunnels"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var DiscordOnModUpdate func(structs.Mod)

const (
	modVerifyPath        = "tmp/mod/"
	staticModIconPath    = "static/modimg/"
	staticModArchivePath = "static/mods/"
)

const (
	UpdateAndPackage  = 4
	DownloadAndCreate = 5
)

func HandleModRepository(tunnelid string, mode int, mod string, author string) {
	switch mode { // same checks are being conducted in webserver module but always good to have security implemented in the actual function
	case DownloadAndCreate:
		mod = security.GenerateID()
	case UpdateAndPackage:
		modData, err := database.GetMod(mod)
		if err != nil {
			println("user ", author, " tried to update mod ", mod, " which does not exist.")
			return
		}

		if modData.Author != author {
			println("user ", author, " tried to update mod ", mod, " which they do not own.")
			return
		}
	}

	filereceived := false
	var archivepath string

	for !filereceived {

		println("Checking if", tunnelid, "has finished so", mod, "can proceed.")

		complete, err := tunnels.CheckTunnel(tunnelid)
		if err != nil {
			println("mod upload tunnel error")
			return
		}

		filereceived = complete

		if filereceived {
			archivepath, err = tunnels.GetTunnelFile(tunnelid)
			if err != nil {
				println("mod upload tunnel file does not exist error")
				return
			}
		}

		time.Sleep(time.Second * 3)
	}

	println("Tunnel has finished.", mod, "will be processed.")
	err := helper.CreateTemp()
	if err != nil {
		println(err.Error())
		return
	}

	path := "publishtmp/" + mod

	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		println(err.Error())
		return
	}

	err = archive.Extract(archivepath, path)
	if err != nil {
		println("extraction failed:", err.Error())
		err := message.SendMessage("0", author, fmt.Sprintf("Your mod archive file could not be extracted by the server!: %s", err.Error()))
		if err != nil {
			println(err.Error())
			return
		}
		return
	}

	err = tunnels.RemoveTunnel(tunnelid)
	if err != nil {
		println("mod upload could not delete tunnel error")
		return
	}

	metadata, err := RunValidator(path)
	if err != nil {
		print("validator failed!:", err.Error())
		err := message.SendMessage("0", author, fmt.Sprintf("Your mod upload didn't pass validation!: %s", err.Error()))
		if err != nil {
			println(err.Error())
			return
		}
		return
	}

	if mode == DownloadAndCreate {
		_, err = AddMod(metadata, true, mod, author)
		if err != nil {
			println(err.Error())
			return
		}
	}

	if mode == UpdateAndPackage || mode == DownloadAndCreate {
		println("packaging mod")
		packagePath := "static/mods/" + mod + ".tar.gz"

		var ignoreFilesBuffer []string
		emlIgnore, err := os.ReadFile(path + "/.emlignore")

		if err == nil {
			ignoreFilesBuffer = parseIgnore(string(emlIgnore))
		}

		err = archive.Package(path, packagePath, ignoreFilesBuffer)
		ffmpeg.ResizeImage(path+"/"+metadata.IconPath, 512, 512, "static/modimg/"+mod+".webp")
		if err != nil {
			println(errors.New("packaging error"))
			return
		}

	}

	err = UpdateModMeta(metadata, mod)
	if err != nil {
		println(errors.New("failed to update mod meta"))
		return
	}
	println("finished handling mod repo")

	switch mode {
	case DownloadAndCreate:
		err := message.SendMessage("0", author, fmt.Sprintf("(mod)[%s] has been uploaded successfully!", mod))
		if err != nil {
			return
		}
	case UpdateAndPackage:
		modData, err := database.GetMod(mod)
		if err != nil {
			return
		}
		err = message.SendMessage("0", author, fmt.Sprintf("(mod)[%s] has finished updating!", mod))
		if err != nil {
			return
		}
		DiscordOnModUpdate(modData)
	}
}

func RunValidator(path string) (structs.ModMetadata, error) {
	err := helper.CreateTemp()
	if err != nil {
		return structs.ModMetadata{}, err
	}
	defer helper.RemoveTemp()
	err = exec.Command(config.LoadedConfig["VALIDATOR_EXECUTABLE"], path, "result.json").Run()
	if err != nil {
		return structs.ModMetadata{}, errors.New("Validator " + err.Error())
	}
	result, err := os.ReadFile("result.json")
	if err != nil {
		return structs.ModMetadata{}, errors.New("io error (calvin)")
	}
	var data structs.ModMetadata
	err = json.NewDecoder(bytes.NewBuffer(result)).Decode(&data)
	if err != nil {
		return structs.ModMetadata{}, errors.New("failed to read result json")
	}
	return data, nil
}

func parseIgnore(buffer string) []string {
	lines := strings.Split(buffer, "\n")
	var processedLines []string
	for _, e := range lines {
		if strings.HasPrefix(e, "//") {
			continue
		}

		if strings.Contains(e, "\\") {
			continue
		}

		if strings.HasPrefix(e, "#") {
			continue
		}

		processedLines = append(processedLines, "--exclude="+e)
	}

	processedLines = append(processedLines, "--exclude=.emlignore")

	return processedLines
}

func UpdateModMeta(modMetadata structs.ModMetadata, ID string) error {
	row := database.Database.QueryRow("SELECT version FROM mods WHERE id=$1", ID)
	var version int
	err := row.Scan(&version)
	if err != nil {
		return err
	}

	_, err = database.Database.Exec("UPDATE mods SET name=$1, description=$2, game=$3, platform=$4, youtube=$5, version=$6 shortdescription=$7 WHERE id=$8", modMetadata.Name, modMetadata.Description, modMetadata.Game, strings.ToLower(modMetadata.Platform), modMetadata.Video, version+1, modMetadata.ShortDescription, ID)
	return err
}

func AddMod(modMetadata structs.ModMetadata, publish bool, id string, author string) (string, error) {
	mod := structs.Mod{
		ID:               id,
		Author:           author,
		Name:             modMetadata.Name,
		Description:      modMetadata.Description,
		ShortDescription: modMetadata.ShortDescription,
		Platform:         strings.ToLower(modMetadata.Platform),
		Game:             modMetadata.Game,
		Video:            modMetadata.Video,
		Published:        publish,
		Version:          1,
		Downloads:        0,
		Verified:         false,
	}

	err := database.CreateMod(mod)
	if err != nil {
		return "", err
	}

	if config.LoadedConfig["MANUAL_REVIEW_MODS_REQUIRED"] == "off" {
		database.VerifyMod(mod.ID)
	} else {
		ticket.AddTicket("Review of "+modMetadata.Name, "modreview", id, "", author)
		message.SendMessage("0", author, "Your mod has been uploaded successfully, but will have to be manually reviewed before being made public to the mod index for security purposes. Don't worry, this won't take too long!")
	}

	return id, nil
}

func OnModVerified() {
}

func GetModsInBulk(mods []string) ([]structs.Mod, error) {
	var modObjs []structs.Mod
	for _, e := range mods {
		modObj, err := database.GetMod(e)
		if err != nil {
			return nil, err
		}
		modObjs = append(modObjs, modObj)
	}

	return modObjs, nil
}
