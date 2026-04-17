package cmd

import (
	"testing"

	"github.com/skevetter/devpod/pkg/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseForwardPortSpec_ServiceNameTarget(t *testing.T) {
	mapping, err := parseForwardPortSpec("8080:nginx:80")
	require.NoError(t, err)
	assert.Equal(t, "tcp", mapping.Host.Protocol)
	assert.Equal(t, "localhost:8080", mapping.Host.Address)
	assert.Equal(t, "tcp", mapping.Container.Protocol)
	assert.Equal(t, "nginx:80", mapping.Container.Address)
}

func TestParseForwardPortSpec_ServiceNameTargetWithLocalBindHost(t *testing.T) {
	mapping, err := parseForwardPortSpec("127.0.0.1:8080:nginx:80")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8080", mapping.Host.Address)
	assert.Equal(t, "nginx:80", mapping.Container.Address)
}

func TestParseForwardPortSpec_PreservesLocalBindDisambiguation(t *testing.T) {
	mapping, err := parseForwardPortSpec("localhost:8080:80")
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", mapping.Host.Address)
	assert.Equal(t, "localhost:80", mapping.Container.Address)
}

func TestParseForwardPortSpec_AllowsRemoteIPTargets(t *testing.T) {
	mapping, err := parseForwardPortSpec("8080:10.0.0.2:80")
	require.NoError(t, err)
	assert.Equal(t, "localhost:8080", mapping.Host.Address)
	assert.Equal(t, "10.0.0.2:80", mapping.Container.Address)
}

func TestParseForwardPortSpec_RejectsNonIPLocalBindHost(t *testing.T) {
	_, err := parseForwardPortSpec("app:8080:nginx:80")
	require.Error(t, err)
	assert.ErrorContains(t, err, "not an ip address app")
}

func TestParseForwardPortSpec_RejectsEmptyRemoteHost(t *testing.T) {
	_, err := parseForwardPortSpec("8080::80")
	require.Error(t, err)
	assert.ErrorContains(t, err, "remote host is empty")
}

func TestParseForwardPortSpec_DelegatesUnixSocketMappings(t *testing.T) {
	mapping, err := parseForwardPortSpec("/tmp/local.sock:/tmp/remote.sock")
	require.NoError(t, err)
	assert.Equal(t, port.Mapping{
		Host:      port.Address{Protocol: "unix", Address: "/tmp/local.sock"},
		Container: port.Address{Protocol: "unix", Address: "/tmp/remote.sock"},
	}, mapping)
}

func TestParseForwardPortSpec_DoesNotChangeSharedParser(t *testing.T) {
	_, err := port.ParsePortSpec("8080:nginx:80")
	require.Error(t, err)
}
