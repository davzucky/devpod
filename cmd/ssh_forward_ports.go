package cmd

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/skevetter/devpod/pkg/port"
)

func parseForwardPortSpec(raw string) (port.Mapping, error) {
	parts := strings.Split(raw, ":")

	switch len(parts) {
	case 1, 2:
		return port.ParsePortSpec(raw)
	case 3:
		if !isPortNumber(parts[0]) {
			return port.ParsePortSpec(raw)
		}

		return newForwardPortMapping("", parts[0], parts[1], parts[2])
	case 4:
		if parts[0] == "" {
			return port.Mapping{}, fmt.Errorf("local host is empty")
		}

		return newForwardPortMapping(parts[0], parts[1], parts[2], parts[3])
	default:
		return port.Mapping{}, fmt.Errorf("unexpected port format: %s", raw)
	}
}

func newForwardPortMapping(
	localHost, localPort, remoteHost, remotePort string,
) (port.Mapping, error) {
	hostAddress, err := parseForwardLocalAddress(localHost, localPort)
	if err != nil {
		return port.Mapping{}, fmt.Errorf("parse host address: %w", err)
	}

	containerAddress, err := parseForwardRemoteAddress(remoteHost, remotePort)
	if err != nil {
		return port.Mapping{}, fmt.Errorf("parse container address: %w", err)
	}

	return port.Mapping{
		Host:      hostAddress,
		Container: containerAddress,
	}, nil
}

func parseForwardLocalAddress(host, rawPort string) (port.Address, error) {
	return parseForwardTCPAddress(host, rawPort, false)
}

func parseForwardRemoteAddress(host, rawPort string) (port.Address, error) {
	if host == "" {
		return port.Address{}, fmt.Errorf("remote host is empty")
	}

	return parseForwardTCPAddress(host, rawPort, true)
}

func parseForwardTCPAddress(host, rawPort string, allowHostnames bool) (port.Address, error) {
	if !isPortNumber(rawPort) {
		return port.Address{}, fmt.Errorf("invalid port %s", rawPort)
	}

	if host == "" {
		host = "localhost"
	}

	if !allowHostnames && host != "localhost" && net.ParseIP(host) == nil {
		return port.Address{}, fmt.Errorf("not an ip address %s", host)
	}

	return port.Address{
		Protocol: "tcp",
		Address:  net.JoinHostPort(host, rawPort),
	}, nil
}

func isPortNumber(raw string) bool {
	_, err := strconv.Atoi(raw)
	return err == nil
}
