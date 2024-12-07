package main

import (
	"encoding/json"
	"os"
	"time"
)

// Config for server
type Config struct {
	Listen  string        `json:"listen"`
	Target  string        `json:"target"`
	Key     string        `json:"key"`
	Crypt   string        `json:"crypt"`
	Mode    string        `json:"mode"`
	SockBuf int           `json:"sockbuf"`
	Timeout time.Duration `json:"timeout"`
}

func parseJSONConfig(config *Config, path string) error {
	file, err := os.Open(path) // For read access.
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(config)
}
