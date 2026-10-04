package version

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The agent skill's plugin follows witr's version, so Claude Code users get
// the guidance for the release they run. Bump them together.
func TestPluginVersionMatchesRelease(t *testing.T) {
	data, err := os.ReadFile("../../plugins/witr/.claude-plugin/plugin.json")
	if os.IsNotExist(err) {
		t.Skip("the plugin isn't in this source tree (packaged builds ship only the Go sources)")
	}
	if err != nil {
		t.Fatal(err)
	}
	var plugin struct{ Version string }
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(embedded); plugin.Version != want {
		t.Errorf("plugins/witr/.claude-plugin/plugin.json version = %q, want %q (internal/version/VERSION)", plugin.Version, want)
	}
}

// Packagers read VERSION from the repository root, but go:embed can only read
// the copy in this package, so the two files are kept equal.
func TestRootVersionMatchesRelease(t *testing.T) {
	data, err := os.ReadFile("../../VERSION")
	if os.IsNotExist(err) {
		t.Skip("the root VERSION isn't in this source tree (packaged builds ship only the Go sources)")
	}
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(data)), strings.TrimSpace(embedded); got != want {
		t.Errorf("VERSION = %q, want %q (internal/version/VERSION)", got, want)
	}
}
