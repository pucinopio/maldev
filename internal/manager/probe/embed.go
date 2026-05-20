package probe

import (
	"embed"
	"fmt"
	"sort"
)

//go:embed agents/linux-amd64 agents/linux-arm64 agents/darwin-amd64 agents/darwin-arm64 agents/windows-amd64.exe
var agentFS embed.FS

var targetFile = map[string]string{
	"linux-amd64":   "agents/linux-amd64",
	"linux-arm64":   "agents/linux-arm64",
	"darwin-amd64":  "agents/darwin-amd64",
	"darwin-arm64":  "agents/darwin-arm64",
	"windows-amd64": "agents/windows-amd64.exe",
}

// ServeAgent returns the raw bytes of the probe binary for target (formatted
// as "os-arch", e.g. "linux-amd64"). The Windows binary is served when
// "windows-amd64" is requested even though the embedded filename has the
// .exe suffix.
func ServeAgent(target string) ([]byte, error) {
	path, ok := targetFile[target]
	if !ok {
		return nil, fmt.Errorf("probe: unknown target %q (available: %v)", target, AvailableTargets())
	}
	return agentFS.ReadFile(path)
}

// AvailableTargets returns the list of supported OS-arch identifiers.
func AvailableTargets() []string {
	out := make([]string, 0, len(targetFile))
	for k := range targetFile {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
