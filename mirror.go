package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	appconfig "gitlab-api-migrate/config"
	"gitlab-api-migrate/logger"
)

func MirrorRepo(sourceCfg, targetCfg appconfig.GitLabConfig, fullRepoName, destinationPath string, projectName string, log *logger.Logger) (time.Duration, string, error) {
	start := time.Now()

	if _, err := os.Stat(destinationPath); !os.IsNotExist(err) {
		log.Info(fmt.Sprintf("Directory %s already exists, removing", destinationPath))
		os.RemoveAll(destinationPath)
	}

	if projectName != "" {
		if _, err := os.Stat(projectName); !os.IsNotExist(err) {
			os.RemoveAll(projectName)
		}
	}

	sourceURL := buildCloneURL(sourceCfg, fullRepoName)
	pushURL := buildPushURL(targetCfg, fullRepoName)

	sourceAuth, err := authFor(sourceCfg)
	if err != nil {
		return 0, "", fmt.Errorf("source auth error: %w", err)
	}

	targetAuth, err := authFor(targetCfg)
	if err != nil {
		return 0, "", fmt.Errorf("target auth error: %w", err)
	}

	log.Info(fmt.Sprintf("Cloning mirror from %s", maskURL(sourceURL)), map[string]string{"project": fullRepoName})

	repo, err := git.PlainClone(destinationPath, true, &git.CloneOptions{
		URL:      sourceURL,
		Progress: nil,
		Mirror:   true,
		Auth:     sourceAuth,
	})
	if err != nil {
		if err == transport.ErrAuthenticationRequired {
			return 0, "", fmt.Errorf("authentication required for source %s", sourceURL)
		}
		return 0, "", fmt.Errorf("clone error: %w", err)
	}

	log.Info(fmt.Sprintf("Repository cloned successfully"), map[string]string{"project": fullRepoName})

	var repoSize string
	if _, err := os.Stat(destinationPath); err == nil {
		repoSize = formatSize(dirSize(destinationPath))
	}

	err = repo.Push(&git.PushOptions{
		RemoteURL: pushURL,
		Auth:      targetAuth,
		Progress:  nil,
		RefSpecs:  []gogitconfig.RefSpec{"+refs/*:refs/*"},
	})
	if err != nil {
		return 0, "", fmt.Errorf("push error: %w", err)
	}

	elapsed := time.Since(start)
	log.Info(fmt.Sprintf("Repository migrated successfully"),
		map[string]string{
			"project":  fullRepoName,
			"duration": elapsed.Round(time.Second).String(),
			"size":     repoSize,
		})

	os.RemoveAll(destinationPath)

	return elapsed, repoSize, nil
}

func buildCloneURL(cfg appconfig.GitLabConfig, fullRepoName string) string {
	if cfg.Transport == appconfig.TransportHTTPS {
		return fmt.Sprintf("%s/%s.git", cfg.URL, fullRepoName)
	}
	port := cfg.SSHPort
	if port == 0 {
		port = 22
	}
	if port == 22 {
		return fmt.Sprintf("git@%s:%s.git", extractHost(cfg.URL), fullRepoName)
	}
	return fmt.Sprintf("ssh://git@%s:%d/%s.git", extractHost(cfg.URL), port, fullRepoName)
}

func buildPushURL(cfg appconfig.GitLabConfig, fullRepoName string) string {
	if cfg.Transport == appconfig.TransportHTTPS {
		return fmt.Sprintf("%s/%s.git", cfg.URL, fullRepoName)
	}
	port := cfg.SSHPort
	if port == 0 {
		port = 22
	}
	if port == 22 {
		return fmt.Sprintf("git@%s:%s.git", extractHost(cfg.URL), fullRepoName)
	}
	return fmt.Sprintf("ssh://git@%s:%d/%s.git", extractHost(cfg.URL), port, fullRepoName)
}

func authFor(cfg appconfig.GitLabConfig) (transport.AuthMethod, error) {
	if cfg.Transport == appconfig.TransportHTTPS {
		return &http.BasicAuth{
			Username: "gitlab-ci-token",
			Password: cfg.Token,
		}, nil
	}
	return ssh.NewPublicKeysFromFile("git", cfg.SSHKey, "")
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
}

func maskURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "****")
	}
	return u.String()
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			size += fi.Size()
		}
		return nil
	})
	return size
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
