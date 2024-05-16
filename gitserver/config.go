package gitserver

import (
	"os"

	"gopkg.in/yaml.v2"
)

type AccessType int

const (
	AccessRead AccessType = iota
	AccessWrite
)

//go:generate stringer -type=AccessType --trimprefix=Access

type RepoAuth = func(repoName string, userName string, password string, requestedAccess AccessType) bool
type RefAuth = func(repoName string, userName string, refName string, action string, requestedAccess AccessType) bool
type AutoInitAuth = func(repoName string, userName string) bool

// ReposConfig is the configuration of the repositories
type ReposConfig struct {
	Path     string `yaml:"path"`
	AutoInit bool   `yaml:"auto_init"`
}

// Config stores the config of the git server
type Config struct {
	Host          string       `yaml:"host"`
	EnableCORS    bool         `yaml:"cors"`
	Repos         *ReposConfig `yaml:"repos"`
	MaxPacketSize int          `yaml:"max_packet_size"`
	Auth          RepoAuth     `yaml:"-"`
	Protected     bool         `yaml:"protected"`
	RefAuth       RefAuth      `yaml:"-"`
	AutoInitAuth  AutoInitAuth `yaml:"-"`
}

// WriteToPath writes the config to the given filePath
func (config *Config) WriteToPath(filePath string) {
	data, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		panic(err)
	}
}
