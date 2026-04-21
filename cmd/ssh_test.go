package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestForwardTimeout_UsesParsedDuration(t *testing.T) {
	cmd := &SSHCmd{ForwardPortsTimeout: "90s"}

	timeout, err := cmd.forwardTimeout(log.Discard)
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, timeout)
}

func TestRunPortForwards(t *testing.T) {
	tests := []struct {
		name      string
		mappings  []string
		forwardFn portForwardFunc
		wantErr   string
		wantCalls int32
	}{
		{
			name:     "clean exit",
			mappings: []string{"8080:80"},
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
			wantCalls: 1,
		},
		{
			name:     "eof exit",
			mappings: []string{"8080:80"},
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
				return io.EOF
			},
			wantCalls: 1,
		},
		{
			name:     "forward error",
			mappings: []string{"8080:80"},
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
				return errors.New("boom")
			},
			wantErr:   "error forwarding 8080:80: boom",
			wantCalls: 1,
		},
		{
			name:     "parse error",
			mappings: []string{""},
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
			wantErr:   "parse port mapping",
			wantCalls: 0,
		},
		{
			name:     "multiple mappings with error",
			mappings: []string{"8080:80", "8081:81"},
			forwardFn: func(
				_ context.Context,
				_ *ssh.Client,
				_ string,
				localAddr string,
				_ string,
				_ string,
				_ time.Duration,
				_ log.Logger,
			) error {
				if localAddr == "localhost:8081" {
					return errors.New("boom")
				}

				return nil
			},
			wantErr:   "error forwarding 8081:81: boom",
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &SSHCmd{}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			var calls atomic.Int32
			err := cmd.runPortForwards(ctx, nil, portForwardConfig{
				mappings:    tt.mappings,
				logTemplate: "test %s/%s %s/%s",
				forwardFn: func(
					ctx context.Context,
					client *ssh.Client,
					localNetwork string,
					localAddr string,
					remoteNetwork string,
					remoteAddr string,
					timeout time.Duration,
					logger log.Logger,
				) error {
					calls.Add(1)
					return tt.forwardFn(
						ctx,
						client,
						localNetwork,
						localAddr,
						remoteNetwork,
						remoteAddr,
						timeout,
						logger,
					)
				},
			}, log.Discard)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantErr)
			}

			assert.Equal(t, tt.wantCalls, calls.Load())
		})
	}
}
