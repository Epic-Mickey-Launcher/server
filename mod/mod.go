package mod

import (
	"bytes"
	"emlserver/archive"
	"emlserver/config"
	"emlserver/database"
	"emlserver/ffmpeg"
	"emlserver/git"
	"emlserver/helper"
	"emlserver/security"
	"emlserver/structs"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
)

const (
	modVerifyPath        = "tmp/mod/"
	staticModIconPath    = "static/modimg/"
	staticModArchivePath = "static/mods/"
)

const (
	DOWNLOAD_AND_PACKAGE = 0
	PACKAGE              = 1
	DOWNLOAD             = 2
	UPDATE               = 3
	UPDATE_AND_PACKAGE   = 4
	DOWNLOAD_AND_CREATE  = 5
)

func HandleModRepository(url string, mode int, mod string, author string) (string, error) {
	if mode == DOWNLOAD_AND_CREATE {
		mod = security.GenerateID()
	}

	modObj, err := database.GetMod(mod)
	if err != nil && mode != DOWNLOAD_AND_CREATE {
		return "", err
	}
	path := "modrepos/" + mod
	_, exists := os.ReadDir(path)
	if mode == DOWNLOAD || mode == DOWNLOAD_AND_PACKAGE || mode == DOWNLOAD_AND_CREATE {
		println("downloading mod repo")
		if exists == nil {
			if url == modObj.RepositoryUrl {
				return "", errors.New("repository does not need to be downloaded again.")
			}
			os.RemoveAll(path)
		} else if !errors.Is(exists, os.ErrNotExist) {
			return "", exists
		}

		err = os.MkdirAll(path, 0700)
		if err != nil {
			return "", errors.New("io error (jumbo)")
		}
		println(url)
		err = git.Clone(url, path)
		if err != nil {
			return "", err
		}

		err = UpdateGitURl(mod, url)
		if err != nil {
			println(err.Error())
			return "", errors.New("failed to update git url of mod")
		}
	}
	if mode == UPDATE || mode == UPDATE_AND_PACKAGE {
		err = git.Update(path)
		if err != nil {
			return "", errors.New("git error")
		}
	}

	metadata, err := RunValidator(path)
	if err != nil {
		return "", err
	}

	if mode == DOWNLOAD_AND_CREATE {
		_, err = AddMod(metadata, true, mod, url, author)
		if err != nil {
			return "", err
		}
	}

	if mode == PACKAGE || mode == UPDATE_AND_PACKAGE || mode == DOWNLOAD_AND_PACKAGE || mode == DOWNLOAD_AND_CREATE {
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
			println(err.Error())
			return "", errors.New("packaging error")
		}

		if mode == UPDATE_AND_PACKAGE {

			err := UpdateModMeta(metadata, mod)
			if err != nil {
				println(err.Error())
				return "", errors.New("mod meta update error")
			}
		}
	}

	err = UpdateModMeta(metadata, mod)
	if err != nil {
		println(err.Error())
		return "", errors.New("failed to update mod meta")
	}
	println("finished handling mod repo")
	return mod, nil
}

func RunValidator(path string) (structs.ModMetadata, error) {
	helper.CreateTemp()
	defer helper.RemoveTemp()
	err := exec.Command(config.LoadedConfig["VALIDATOR_EXECUTABLE"], path, "result.json").Run()
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

func UpdateGitURl(id string, url string) error {
	_, err := database.Database.Exec("UPDATE mods SET repositoryurl=$1 WHERE id=$2", url, id)
	return err
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

func DeleteMod(ID string) error {
	_, err := database.Database.Exec("DELETE FROM mods WHERE id=$1", ID)
	if err != nil {
		return err
	}

	err = os.RemoveAll("modrepos/" + ID)
	if err != nil {
		println("IO ERROR: failed to remove " + ID + " from modrepos.")
	}

	err = os.Remove("static/mods/" + ID + ".tar.gz")
	if err != nil {
		println("IO ERROR: failed to remove " + ID + ".tar.gz from static/mods.")
	}

	err = os.Remove("static/modimg/" + ID + ".webp")
	if err != nil {
		println("IO ERROR: failed to remove " + ID + ".webp from static/modimg.")
	}

	return nil
}

func UpdateModMeta(modMetadata structs.ModMetadata, ID string) error {
	row := database.Database.QueryRow("SELECT version FROM mods WHERE id=$1", ID)
	var version int
	err := row.Scan(&version)
	if err != nil {
		return err
	}

	_, err = database.Database.Exec("UPDATE mods SET name=$1, description=$2, game=$3, platform=$4, youtube=$5, version=$6 WHERE id=$7", modMetadata.Name, modMetadata.Description, modMetadata.Game, modMetadata.Platform, modMetadata.Video, version+1, ID)
	return err
}

func AddMod(modMetadata structs.ModMetadata, publish bool, id string, repoUrl string, author string) (string, error) {
	mod := structs.Mod{
		ID:            id,
		Author:        author,
		Name:          modMetadata.Name,
		Description:   modMetadata.Description,
		Platform:      modMetadata.Platform,
		Game:          modMetadata.Game,
		Video:         modMetadata.Video,
		Published:     publish,
		Version:       1,
		Downloads:     0,
		RepositoryUrl: repoUrl,
	}

	err := database.CreateMod(mod)
	if err != nil {
		return "", err
	}

	return id, nil
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
