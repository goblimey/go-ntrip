package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Config struct {
	LogEvents                  bool   `json:"log_events"`
	MessageLogDirectory        string `json:"message_log_directory"`
	DirectoryForOldMessageLogs string `json:"directory_for_old_message_logs"`
	EventLogDirectory          string `json:"event_log_directory"`
}

// GetConfig gets the config from the given file.
func GetConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		em := fmt.Sprintf("[-] Cannot open config file: %s\n", err.Error())
		slog.Error(em)
		os.Exit(1)
	}

	config, errParse := getConfigFromReader(file)

	if errParse != nil {
		return nil, errParse
	}

	return config, nil
}

// GetConfigFrom Reader gets the config from the given reader.
func getConfigFromReader(configReader io.Reader) (*Config, error) {

	data := make([]byte, 4096)
	n, errRead := configReader.Read(data)
	if errRead != nil {
		em := fmt.Sprintf("[-] Error reading config file: %s\n", errRead.Error())
		slog.Error(em)
		return nil, errRead
	}

	config, parseError := parseConfigFromBytes(data[:n])
	if parseError != nil {
		em := fmt.Sprintf("[-] Not a valid config file: %s\n", parseError.Error())
		slog.Error(em)
		return nil, parseError
	}

	return config, nil
}

func parseConfigFromBytes(data []byte) (*Config, error) {
	var config Config
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
