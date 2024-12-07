package cmd

import (
	"encoding/json"
	"os"
	"time"
)

// Config for server
type Config struct {
	Listen  string        `json:"listen"`
	SockBuf int           `json:"sockbuf"`
	NextHop string        `json:"nexthop"`
	KI      string        `json:"ki"`
	KO      string        `json:"ko"`
	CI      string        `json:"ci"`
	CO      string        `json:"co"`
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
