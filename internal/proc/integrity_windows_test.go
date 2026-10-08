//go:build windows

package proc

import (
	"os"
	"slices"
	"testing"
)

func TestIntegrityName(t *testing.T) {
	for rid, want := range map[uint32]string{0x0: "Untrusted", 0x1000: "Low", 0x2000: "Medium", 0x2100: "Medium", 0x3000: "High", 0x4000: "System", 0x5000: "Protected"} {
		if got := integrityName(rid); got != want {
			t.Errorf("integrityName(%#x) = %q, want %q", rid, got, want)
		}
	}
}

// The test process's own token is always readable.
func TestReadTokenInfoSelf(t *testing.T) {
	user, integrity := readTokenInfo(os.Getpid())
	if user == "unknown" || !slices.Contains([]string{"Low", "Medium", "High", "System"}, integrity) {
		t.Errorf("readTokenInfo(self) = %q, %q", user, integrity)
	}
}
