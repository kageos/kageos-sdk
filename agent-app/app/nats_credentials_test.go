package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppNATSConnectionConfigUsesMountedSecretURL(t *testing.T) {
	credentialsFile := filepath.Join(t.TempDir(), "nats")
	secretURL := "nats://private-user:private-pass@host.containers.internal:4222"
	if err := os.WriteFile(credentialsFile, []byte(secretURL), 0o600); err != nil {
		t.Fatalf("write credentials file: %v", err)
	}

	config, err := loadAppNATSConnectionConfig("nats://host.containers.internal:4222", credentialsFile, true)
	if err != nil {
		t.Fatalf("loadAppNATSConnectionConfig() error = %v", err)
	}
	if config.url != secretURL {
		t.Fatalf("connection URL = %q, want mounted secret URL", redactURLForLog(config.url))
	}
	if config.authMode != appNATSAuthNone {
		t.Fatalf("auth mode = %d, want URL authentication", config.authMode)
	}
}

func TestLoadAppNATSConnectionConfigSupportsNATSCredsFile(t *testing.T) {
	credentialsFile := filepath.Join(t.TempDir(), "app.creds")
	contents := strings.Join([]string{
		"-----BEGIN NATS USER JWT-----",
		"fake-jwt-for-format-detection",
		"------END NATS USER JWT------",
		"-----BEGIN USER NKEY SEED-----",
		"fake-seed-for-format-detection",
		"------END USER NKEY SEED------",
	}, "\n")
	if err := os.WriteFile(credentialsFile, []byte(contents), 0o600); err != nil {
		t.Fatalf("write credentials file: %v", err)
	}

	config, err := loadAppNATSConnectionConfig("nats://legacy:secret@nats.internal:4222", credentialsFile, true)
	if err != nil {
		t.Fatalf("loadAppNATSConnectionConfig() error = %v", err)
	}
	if config.url != "nats://nats.internal:4222" {
		t.Fatalf("connection URL = %q, want endpoint without legacy userinfo", config.url)
	}
	if config.authMode != appNATSAuthUserCredentials || config.credentialsFile != credentialsFile {
		t.Fatalf("unexpected credentials config: mode=%d path=%q", config.authMode, config.credentialsFile)
	}
	options, err := config.natsOptions()
	if err != nil {
		t.Fatalf("natsOptions() error = %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("natsOptions() count = %d, want 1", len(options))
	}
}

func TestLoadAppNATSConnectionConfigPreservesLegacyURLWhenDefaultSecretMissing(t *testing.T) {
	legacyURL := "nats://legacy:secret@nats.internal:4222"
	config, err := loadAppNATSConnectionConfig(legacyURL, filepath.Join(t.TempDir(), "missing"), false)
	if err != nil {
		t.Fatalf("loadAppNATSConnectionConfig() error = %v", err)
	}
	if config.url != legacyURL {
		t.Fatalf("connection URL = %q, want legacy URL", redactURLForLog(config.url))
	}
}

func TestLoadAppNATSConnectionConfigFailsWhenExplicitSecretMissing(t *testing.T) {
	_, err := loadAppNATSConnectionConfig("nats://nats.internal:4222", filepath.Join(t.TempDir(), "missing"), true)
	if err == nil {
		t.Fatal("expected explicitly configured missing credentials file to fail")
	}
}

func TestStripNATSURLUserInfoHandlesServerList(t *testing.T) {
	raw := "nats://user:pass@nats-a:4222, tls://token@nats-b:4222"
	got := stripNATSURLUserInfo(raw)
	want := "nats://nats-a:4222,tls://nats-b:4222"
	if got != want {
		t.Fatalf("stripNATSURLUserInfo() = %q, want %q", got, want)
	}
}

func TestRedactURLForLogRedactsEveryServerInList(t *testing.T) {
	raw := "nats://user-a:password-a@nats-a:4222,nats://super-secret-token@nats-b:4222"
	got := redactURLForLog(raw)
	for _, forbidden := range []string{"user-a", "password-a", "super-secret-token"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redactURLForLog() leaked %q in %q", forbidden, got)
		}
	}
	if !strings.Contains(got, "nats-a:4222") || !strings.Contains(got, "nats-b:4222") {
		t.Fatalf("redactURLForLog() lost server endpoints: %q", got)
	}
}

func TestAppNATSServerCandidatesPreservePriorityAndAuthentication(t *testing.T) {
	raw := "nats://private-token@host.containers.internal:4222,nats://nats.example.com:4333"
	got := appNATSServerCandidates(raw)
	want := []string{
		"nats://private-token@host.containers.internal:4222",
		"nats://private-token@127.0.0.1:4222",
		"nats://private-token@host.docker.internal:4222",
		"nats://private-token@localhost:4222",
		"nats://nats.example.com:4333",
	}
	if len(got) != len(want) {
		t.Fatalf("appNATSServerCandidates() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadAppNATSConnectionConfigRejectsUnknownSecretFormat(t *testing.T) {
	credentialsFile := filepath.Join(t.TempDir(), "nats")
	if err := os.WriteFile(credentialsFile, []byte("definitely-not-a-nats-credential"), 0o600); err != nil {
		t.Fatalf("write credentials file: %v", err)
	}

	_, err := loadAppNATSConnectionConfig("nats://nats.internal:4222", credentialsFile, true)
	if err == nil {
		t.Fatal("expected unknown credential format to fail closed")
	}
}
