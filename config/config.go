package config

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	configPath   string
	LoadedConfig map[string]string
	currentSum   string
)

func updateRoutine() {
	dur, _ := time.ParseDuration("10s")

	println("config auto updated initialized")

	for {
		time.Sleep(dur)
		sum, err := exec.Command("md5sum", configPath).Output()
		if err != nil {
			println("config auto updater has failed " + err.Error())
			return
		}

		sumstr := string(sum)
		if sumstr != currentSum && currentSum != "" {
			println("config has changed... reloading")
			loadConfigToMap()
		}
		currentSum = sumstr
	}
}

func parse(buffer string) (map[string]string, error) {
	lines := strings.Split(buffer, "\n")

	options := make(map[string]string)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		splitLine := strings.SplitN(line, "=", 2)
		if len(splitLine) > 2 {
			println(len(splitLine))
			return options, errors.New("key=value syntax not used.")
		}
		options[strings.ReplaceAll(strings.TrimSpace(splitLine[0]), " ", "_")] = splitLine[1]
	}

	return options, nil
}

func loadConfigToMap() {
	res, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	config, err := parse(string(res))
	if err != nil {
		panic(err)
	}

	LoadedConfig = config
}

func LoadConfig(path string) {
	configPath = path
	loadConfigToMap()
	go updateRoutine()
}
