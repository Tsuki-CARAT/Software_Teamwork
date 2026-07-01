package mcppolicy

import (
	"errors"
	"path/filepath"
	"strings"
)

type StdioCommandSpec int

const (
	StdioCommandEchoTest StdioCommandSpec = iota + 1
)

func ValidateStdioCommand(command string, args []string) (StdioCommandSpec, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return 0, errors.New("MCP stdio command is required")
	}
	if strings.ContainsAny(command, ";|&$<>`\r\n\t ") || strings.ContainsRune(command, '\x00') {
		return 0, errors.New("MCP stdio command must be a shell-free executable name")
	}
	if filepath.Base(command) != command {
		return 0, errors.New("MCP stdio command must be an allowlisted executable name")
	}
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return 0, errors.New("MCP stdio arguments must not contain NUL or newlines")
		}
	}
	if command == "go" && len(args) == 2 && args[0] == "run" && args[1] == "./testserver/cmd/echo" {
		return StdioCommandEchoTest, nil
	}
	return 0, errors.New("MCP stdio command spec is not allowlisted")
}
