package config

import (
	"errors"
	"os"
	"strings"
)

var LoadedConfig map[string]string

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

func LoadConfig(path string) error {
	res, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	config, err := parse(string(res))

	LoadedConfig = config

	return err
}
