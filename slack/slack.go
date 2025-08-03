package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slack-go/slack"
)

type Config struct {
	Token string `json:"token"`
}

func IsLoggedIn() bool {
	configLocation, err := getConfigPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(configLocation)
	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		return false
	}

	data, err := os.ReadFile(configLocation)
	if err != nil {
		return false
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	client := slack.New(config.Token)
	_, err = client.AuthTest()

	return err == nil
}

func getConfigPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "HackCLI", "config.json"), nil
}

func GetToken() (string, error) {
	configLocation, err := getConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configLocation)
	if err != nil {
		return "", err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return "", err
	}

	if config.Token == "" {
		return "", fmt.Errorf("no token found in config")
	}

	return config.Token, nil
}
