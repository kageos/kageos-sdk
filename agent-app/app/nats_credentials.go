package app

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
)

const (
	defaultAppNATSCredentialsFile = "/run/secrets/kageos-nats"
	appNATSCredentialsFileEnv     = "KAGEOS_NATS_CREDENTIALS_FILE"
)

type appNATSAuthMode uint8

const (
	appNATSAuthNone appNATSAuthMode = iota
	appNATSAuthUserCredentials
	appNATSAuthNKeySeed
)

type appNATSConnectionConfig struct {
	url             string
	authMode        appNATSAuthMode
	credentialsFile string
}

func resolveAppNATSConnectionConfig() (appNATSConnectionConfig, error) {
	credentialsFile, required := resolveAppNATSCredentialsFile()
	return loadAppNATSConnectionConfig(resolveNATSURL(), credentialsFile, required)
}

func resolveAppNATSCredentialsFile() (path string, required bool) {
	if path := strings.TrimSpace(os.Getenv(appNATSCredentialsFileEnv)); path != "" {
		return path, true
	}
	return defaultAppNATSCredentialsFile, false
}

func loadAppNATSConnectionConfig(endpoint, credentialsFile string, required bool) (appNATSConnectionConfig, error) {
	endpoint = strings.TrimSpace(endpoint)
	contents, err := os.ReadFile(credentialsFile)
	if err != nil {
		if os.IsNotExist(err) && !required {
			// Older deployments keep authentication in NATS_URL. Preserve that
			// behavior only when the runtime secret has not been provisioned.
			return appNATSConnectionConfig{url: endpoint}, nil
		}
		return appNATSConnectionConfig{}, fmt.Errorf("read app NATS credentials file: %w", err)
	}

	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "" {
		return appNATSConnectionConfig{}, fmt.Errorf("app NATS credentials file is empty")
	}

	if isNATSUserCredentials(contents) {
		return appNATSConnectionConfig{
			url:             stripNATSURLUserInfo(endpoint),
			authMode:        appNATSAuthUserCredentials,
			credentialsFile: credentialsFile,
		}, nil
	}

	if isNATSUserNKeySeed(contents) {
		if _, err := nats.NkeyOptionFromSeed(credentialsFile); err != nil {
			return appNATSConnectionConfig{}, fmt.Errorf("load app NATS nkey seed: %w", err)
		}
		return appNATSConnectionConfig{
			url:             stripNATSURLUserInfo(endpoint),
			authMode:        appNATSAuthNKeySeed,
			credentialsFile: credentialsFile,
		}, nil
	}

	if err := validateNATSServerURLs(trimmed); err != nil {
		return appNATSConnectionConfig{}, fmt.Errorf("unsupported app NATS credentials file format: %w", err)
	}

	// Basic user/password and token authentication are stored as a private
	// NATS URL in the mounted secret. The public NATS_URL environment variable
	// remains an endpoint without userinfo.
	return appNATSConnectionConfig{url: trimmed}, nil
}

func (c appNATSConnectionConfig) natsOptions() ([]nats.Option, error) {
	switch c.authMode {
	case appNATSAuthNone:
		return nil, nil
	case appNATSAuthUserCredentials:
		return []nats.Option{nats.UserCredentials(c.credentialsFile)}, nil
	case appNATSAuthNKeySeed:
		option, err := nats.NkeyOptionFromSeed(c.credentialsFile)
		if err != nil {
			return nil, err
		}
		return []nats.Option{option}, nil
	default:
		return nil, fmt.Errorf("unknown app NATS authentication mode: %d", c.authMode)
	}
}

func isNATSUserCredentials(contents []byte) bool {
	text := string(contents)
	return strings.Contains(text, "-----BEGIN NATS USER JWT-----") &&
		strings.Contains(text, "-----BEGIN USER NKEY SEED-----")
}

func isNATSUserNKeySeed(contents []byte) bool {
	text := strings.TrimSpace(string(contents))
	return strings.HasPrefix(text, "SU") ||
		(strings.Contains(text, "-----BEGIN USER NKEY SEED-----") &&
			!strings.Contains(text, "-----BEGIN NATS USER JWT-----"))
}

func validateNATSServerURLs(raw string) error {
	servers := strings.Split(raw, ",")
	if len(servers) == 0 {
		return fmt.Errorf("empty NATS URL")
	}
	for _, server := range servers {
		parsed, err := url.Parse(strings.TrimSpace(server))
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("invalid NATS URL")
		}
		switch strings.ToLower(parsed.Scheme) {
		case "nats", "tls", "ws", "wss":
		default:
			return fmt.Errorf("invalid NATS URL scheme")
		}
	}
	return nil
}

func stripNATSURLUserInfo(raw string) string {
	servers := strings.Split(raw, ",")
	for i, server := range servers {
		server = strings.TrimSpace(server)
		parsed, err := url.Parse(server)
		if err != nil || parsed.Host == "" {
			servers[i] = server
			continue
		}
		parsed.User = nil
		servers[i] = parsed.String()
	}
	return strings.Join(servers, ",")
}
