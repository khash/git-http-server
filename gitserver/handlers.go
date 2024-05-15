package gitserver

import (
	"bytes"
	"errors"
	"fmt"
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
		w.WriteHeader(404)
		return
	}

	if gServerConfig.Repos.AutoInit && !repoExists(repo) {
		cmd := GitCommand{Args: []string{"init", "--bare", repo}}
		_, err := cmd.Run(true)
		if err != nil {
			w.WriteHeader(404)
			return
		}
	}

	log.Debug().Str("repo", repo).Msg("getInfoRefs")

	serviceName := getServiceName(r)

	//message := messageFromService(serviceName, route.RepoPath)
	//details := authy.Details{
	//	"repo": repo,
	//	"ip":   r.RemoteAddr,
	//}
	//if !approveTransaction(message, details) {
	//	w.WriteHeader(403)
	//	return
	//}

	w.WriteHeader(200)
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

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(404)
		log.Fatal().Err(err).Msg("Error")
		return
	}

	WriteGitToHTTP(w, GitCommand{ProcInput: bytes.NewReader(requestBody), Args: []string{"upload-pack", "--stateless-rpc", repo}})
}

func receivePack(route *Route, w http.ResponseWriter, r *http.Request) {
	repo, err := AbsoluteRepoPath(route.RepoPath)
	if err != nil {
		return
	}
	log.Info().Str("repo", repo).Msg("receivePack")

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(404)
		log.Fatal().Err(err).Msg("Error")
		return
	}

	WriteGitToHTTP(w, GitCommand{ProcInput: bytes.NewReader(requestBody), Args: []string{"receive-pack", "--stateless-rpc", repo}})
}

func repoExists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

//func messageFromService(service string, repo string) string {
//	message := ""
//	if service == "receive-pack" {
//		message = "Push to " + repo
//	} else if service == "upload-pack" {
//		message = "Fetch from " + repo
//	} else {
//		message = "Unknown service " + service + " for " + repo
//	}
//
//	return message
//}
