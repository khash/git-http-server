package gitserver

import (
	"os"
)

var gServerConfig *Config

// Init initializes the git server.
func Init(config *Config) error {
	gServerConfig = config

	err := os.MkdirAll(gServerConfig.Repos.Path, 0700)

	return err
}
