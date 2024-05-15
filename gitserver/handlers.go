package gitserver

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// AbsoluteRepoPath returns the absolute path for the given relative repository path
func AbsoluteRepoPath(relativePath string) (string, error) {
	if !strings.HasSuffix(relativePath, ".git") {
		relativePath += ".git"
	}

	path := fmt.Sprintf("%s/%s", gServerConfig.Repos.Path, relativePath)
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if strings.Contains(path, "..") {
		return "", errors.New("invalid repo path")
	}

	return absolutePath, nil
}

func getInfoRefs(route *Route, w http.ResponseWriter, r *http.Request) {
	repo, err := AbsoluteRepoPath(route.RepoPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !repoExists(repo) {
		log.Error().Str("repo", repo).Msg("Repo not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if ok, _ := checkAccess(r, w, repo, AccessRead); !ok {
		return
	}

	serviceName := getServiceName(r)

	log.Debug().
		Str("ip", r.RemoteAddr).
		Str("service", serviceName).
		Str("repo", repo).
		Msg("getInfoRefs")

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/x-git-"+serviceName+"-advertisement")

	str := "# service=git-" + serviceName
	fmt.Fprintf(w, "%.4x%s\n", len(str)+5, str)
	fmt.Fprintf(w, "0000")
	WriteGitToHTTP(w, GitCommand{Args: []string{serviceName, "--stateless-rpc", "--advertise-refs", repo}})
}

func getServiceName(r *http.Request) string {
	if len(r.Form["service"]) > 0 {
		return strings.Replace(r.Form["service"][0], "git-", "", 1)
	}

	return ""
}

func uploadPack(route *Route, w http.ResponseWriter, r *http.Request) {
	repo, err := AbsoluteRepoPath(route.RepoPath)
	if err != nil {
		return
	}
	log.Info().Str("repo", repo).Msg("uploadPack")

	if !repoExists(repo) {
		log.Error().Str("repo", repo).Msg("Repo not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if ok, _ := checkAccess(r, w, repo, AccessRead); !ok {
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Fatal().Err(err).Msg("Error")
		return
	}

	WriteGitToHTTP(w, GitCommand{ProcInput: bytes.NewReader(requestBody), Args: []string{"upload-pack", "--stateless-rpc", repo}})
}

func receivePack(route *Route, w http.ResponseWriter, r *http.Request) {
	repo, err := AbsoluteRepoPath(route.RepoPath)
	if err != nil {
		log.Error().Err(err).Msg("AbsoluteRepoPath error")
		return
	}
	log.Info().Str("repo", repo).Msg("receivePack")

	if !repoExists(repo) {
		log.Error().Str("repo", repo).Msg("Repo not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ok := false
	var user string
	if ok, user = checkAccess(r, w, repo, AccessWrite); !ok {
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Err(err).Msg("Error")
		return
	}

	log.Debug().
		Int("length", len(requestBody)).
		Msg("Unpacking packfile")

	if len(requestBody) > gServerConfig.MaxPacketSize {
		log.Error().Msg("Error: packfile too large")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ur := packp.NewReferenceUpdateRequest()
	input := bytes.NewReader(requestBody)
	err = ur.Decode(input)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error().Err(err).Msg("Error decoding packfile")
		return
	}

	// dump the commands
	for _, cmd := range ur.Commands {
		log.Debug().
			Str("action", string(cmd.Action())).
			Str("name", cmd.Name.String()).
			Str("old", cmd.Old.String()).
			Str("new", cmd.New.String()).
			Msg("Command")

		// check ref auth
		if gServerConfig.RefAuth != nil {
			log.Debug().
				Str("action", string(cmd.Action())).
				Str("ref", cmd.Name.String()).
				Msg("Checking ref auth")

			if !gServerConfig.RefAuth(repo, user, cmd.Name.String(), string(cmd.Action()), AccessWrite) {
				log.Error().
					Str("action", string(cmd.Action())).
					Str("ref", cmd.Name.String()).
					Msg("Ref auth failed")
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
	}

	WriteGitToHTTP(w, GitCommand{ProcInput: bytes.NewReader(requestBody), Args: []string{"receive-pack", "--stateless-rpc", repo}})
}

func repoExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

// the first return value is the result of the authentication
// the second one is if the server should ask for authentication
func checkAccess(r *http.Request,
	w http.ResponseWriter,
	repo string,
	access AccessType) (bool, string) {
	if !gServerConfig.Protected {
		return true, ""
	}

	log.Debug().
		Str("repo", repo).
		Str("access", access.String()).
		Msg("Checking access")

	if gServerConfig.Auth == nil {
		log.Fatal().Msg("No auth configured")
		return false, ""
	}

	user, password, ok := r.BasicAuth()
	if !ok {
		log.Debug().Msg("No basic auth values found")

		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		http.Error(w, "Access denied", http.StatusUnauthorized)
		return false, user
	}

	// repo is the repo name prefixed with the repos path, so we need to remove it
	repo = strings.Replace(repo, gServerConfig.Repos.Path+"/", "", 1)
	// remove the trailing .git
	repo = strings.TrimSuffix(repo, ".git")
	// remove the leading /
	repo = strings.TrimPrefix(repo, "/")

	if gServerConfig.Auth(repo, user, password, access) {
		return true, user
	}

	log.Debug().Msg("Access denied")
	http.Error(w, "Access denied", http.StatusForbidden)

	return false, user
}
