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
