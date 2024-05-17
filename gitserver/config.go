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
type PostCommitHook = func(repoName string, userName string)

// ReposConfig is the configuration of the repositories
type ReposConfig struct {
	Path     string `yaml:"path"`
	AutoInit bool   `yaml:"auto_init"`
}

// Config stores the config of the git server
type Config struct {
	Host           string
	EnableCORS     bool
	Repos          *ReposConfig
	MaxPacketSize  int
	Auth           RepoAuth
	Protected      bool
	RefAuth        RefAuth
	AutoInitAuth   AutoInitAuth
	PostCommitHook PostCommitHook
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
