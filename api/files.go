package api

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Jan-Kur/HackCLI/core"
)

func getConfigPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "HackCLI", "config.json"), nil
}

func getCachePath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "HackCLI", "cache.json"), nil
}

func LoadConfig() (core.Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return core.Config{}, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return core.Config{}, err
	}

	var cfg core.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return core.Config{}, err
	}

	return cfg, nil
}

func SaveConfig(cfg core.Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func LoadCache() *core.Cache {
	cachePath, err := getCachePath()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var cache core.Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}

	return &cache
}

func SaveCache(cache core.Cache) error {
	path, err := getCachePath()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
