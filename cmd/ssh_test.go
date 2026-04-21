package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/skevetter/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func writeGitConfig(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(content), 0o600)
	assert.NoError(t, err)
}

func TestGpgSigningKey_GPGFormat(t *testing.T) {
	writeGitConfig(t, "[user]\n\tsigningKey = TESTKEY123\n")
	result := gpgSigningKey(log.Discard)
	assert.Equal(t, "TESTKEY123", result)
}

func TestGpgSigningKey_SSHFormat_Skipped(t *testing.T) {
	writeGitConfig(
		t,
		"[gpg]\n\tformat = ssh\n[user]\n\tsigningKey = /home/user/.ssh/id_ed25519.pub\n",
	)
	result := gpgSigningKey(log.Discard)
	assert.Empty(t, result)
}

func TestGpgSigningKey_NoKeyConfigured(t *testing.T) {
	writeGitConfig(t, "[user]\n\tname = Test\n")
	result := gpgSigningKey(log.Discard)
	assert.Empty(t, result)
}

func TestGpgSigningKey_X509Format_Returned(t *testing.T) {
	writeGitConfig(t, "[gpg]\n\tformat = x509\n[user]\n\tsigningKey = /path/to/cert\n")
	result := gpgSigningKey(log.Discard)
	assert.Equal(t, "/path/to/cert", result)
}

func TestGpgSigningKey_SSHKeyPath_Skipped(t *testing.T) {
	writeGitConfig(t, "[user]\n\tsigningKey = /home/user/.ssh/id_ed25519.pub\n")
	result := gpgSigningKey(log.Discard)
	assert.Empty(t, result)
}

func TestGpgSigningKey_TildeKeyPath_Skipped(t *testing.T) {
	writeGitConfig(t, "[user]\n\tsigningKey = ~/.ssh/id_ed25519.pub\n")
	result := gpgSigningKey(log.Discard)
	assert.Empty(t, result)
}

func TestRunPortForwards_ReturnsOnCleanExit(t *testing.T) {
	cmd := &SSHCmd{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := cmd.runPortForwards(ctx, nil, portForwardConfig{
		mappings:    []string{"8080:80"},
		logTemplate: "test %s/%s %s/%s",
		forwardFn: func(
			context.Context,
			*ssh.Client,
			string,
			string,
			string,
			string,
			time.Duration,
			log.Logger,
		) error {
			return nil
		},
	}, log.Discard)

	require.NoError(t, err)
}
