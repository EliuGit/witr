//go:build linux

package proc

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLabelFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// witr's own process shares its user namespace.
func TestInOtherUserNamespaceSelf(t *testing.T) {
	if inOtherUserNamespace(os.Getpid()) {
		t.Error("witr's own process reported in another user namespace")
	}
}

// The label comes from AppArmor's own file, or from the shared file only when
// the module is known to be active: on WSL the shared file reads "kernel".
func TestReadSecurityLabel(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantModule string
		wantLabel  string
	}{
		{"apparmor file", map[string]string{"proc/42/attr/apparmor/current": "/usr/sbin/cupsd (enforce)\n"}, "AppArmor", "/usr/sbin/cupsd (enforce)"},
		{"selinux", map[string]string{"proc/42/attr/current": "system_u:system_r:httpd_t:s0\x00", "sys/kernel/security/lsm": "lockdown,capability,yama,selinux"}, "SELinux", "system_u:system_r:httpd_t:s0"},
		{"selinux without securityfs", map[string]string{"proc/42/attr/current": "system_u:system_r:sshd_t:s0-s0:c0.c1023", "sys/fs/selinux/enforce": "1"}, "SELinux", "system_u:system_r:sshd_t:s0-s0:c0.c1023"},
		{"older apparmor kernel", map[string]string{"proc/42/attr/current": "unconfined\n", "sys/module/apparmor/parameters/enabled": "Y\n"}, "AppArmor", "unconfined"},
		{"no active module (WSL)", map[string]string{"proc/42/attr/current": "kernel", "sys/module/apparmor/parameters/enabled": "N\n"}, "", ""},
	}
	for _, tt := range tests {
		root := t.TempDir()
		for path, content := range tt.files {
			writeLabelFile(t, filepath.Join(root, path), content)
		}
		module, label := readSecurityLabel(root, 42)
		if module != tt.wantModule || label != tt.wantLabel {
			t.Errorf("%s: got %q %q, want %q %q", tt.name, module, label, tt.wantModule, tt.wantLabel)
		}
	}
}
