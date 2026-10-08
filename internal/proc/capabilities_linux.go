//go:build linux

package proc

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// readSecurityLabel returns the security module confining pid and its label:
// "AppArmor" with a profile such as "/usr/sbin/cupsd (enforce)" or
// "unconfined", or "SELinux" with a context. Both are "" when neither module
// is active. root prefixes the /proc and /sys paths, for tests.
func readSecurityLabel(root string, pid int) (module, label string) {
	attr := fmt.Sprintf("%s/proc/%d/attr/", root, pid)
	// Kernels that can stack modules give AppArmor its own file; reading it
	// fails when AppArmor isn't active.
	if l := readAttr(attr + "apparmor/current"); l != "" {
		return "AppArmor", l
	}
	// The shared file belongs to whichever module the kernel set up first
	// (on WSL it reads "kernel"), so trust it only for an active module.
	for _, m := range []struct{ name, module string }{{"selinux", "SELinux"}, {"apparmor", "AppArmor"}} {
		if lsmActive(root, m.name) {
			if l := readAttr(attr + "current"); l != "" {
				return m.module, l
			}
		}
	}
	return "", ""
}

// lsmActive reports whether the kernel runs the named security module.
func lsmActive(root, name string) bool {
	if b, err := os.ReadFile(root + "/sys/kernel/security/lsm"); err == nil {
		return slices.Contains(strings.Split(strings.TrimSpace(string(b)), ","), name)
	}
	switch name {
	case "selinux":
		_, err := os.Stat(root + "/sys/fs/selinux/enforce")
		return err == nil
	case "apparmor":
		b, _ := os.ReadFile(root + "/sys/module/apparmor/parameters/enabled")
		return strings.TrimSpace(string(b)) == "Y"
	}
	return false
}

// readAttr reads a /proc/<pid>/attr file, or "" when it can't be read.
func readAttr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(b), "\x00\n ")
}

// Capability bit positions from include/uapi/linux/capability.h
var capNames = map[int]string{
	0:  "CAP_CHOWN",
	1:  "CAP_DAC_OVERRIDE",
	2:  "CAP_DAC_READ_SEARCH",
	3:  "CAP_FOWNER",
	4:  "CAP_FSETID",
	5:  "CAP_KILL",
	6:  "CAP_SETGID",
	7:  "CAP_SETUID",
	8:  "CAP_SETPCAP",
	9:  "CAP_LINUX_IMMUTABLE",
	10: "CAP_NET_BIND_SERVICE",
	11: "CAP_NET_BROADCAST",
	12: "CAP_NET_ADMIN",
	13: "CAP_NET_RAW",
	14: "CAP_IPC_LOCK",
	15: "CAP_IPC_OWNER",
	16: "CAP_SYS_MODULE",
	17: "CAP_SYS_RAWIO",
	18: "CAP_SYS_CHROOT",
	19: "CAP_SYS_PTRACE",
	20: "CAP_SYS_PACCT",
	21: "CAP_SYS_ADMIN",
	22: "CAP_SYS_BOOT",
	23: "CAP_SYS_NICE",
	24: "CAP_SYS_RESOURCE",
	25: "CAP_SYS_TIME",
	26: "CAP_SYS_TTY_CONFIG",
	27: "CAP_MKNOD",
	28: "CAP_LEASE",
	29: "CAP_AUDIT_WRITE",
	30: "CAP_AUDIT_CONTROL",
	31: "CAP_SETFCAP",
	32: "CAP_MAC_OVERRIDE",
	33: "CAP_MAC_ADMIN",
	34: "CAP_SYSLOG",
	35: "CAP_WAKE_ALARM",
	36: "CAP_BLOCK_SUSPEND",
	37: "CAP_AUDIT_READ",
	38: "CAP_PERFMON",
	39: "CAP_BPF",
	40: "CAP_CHECKPOINT_RESTORE",
}

// ReadCapabilities reads the effective capabilities of a process from
// /proc/<pid>/status. A process in another user namespace, such as a rootless
// container, holds them only over that namespace's resources, not the host's,
// so none are reported for it.
func ReadCapabilities(pid int) []string {
	if inOtherUserNamespace(pid) {
		return nil
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "CapEff:\t") {
			hex := strings.TrimSpace(strings.TrimPrefix(line, "CapEff:"))
			return decodeCapabilities(hex)
		}
	}
	return nil
}

// inOtherUserNamespace reports whether pid runs in a different user namespace
// than witr. Another user's namespace link can't be read without privileges;
// then it reports false and the capabilities stand.
func inOtherUserNamespace(pid int) bool {
	own, err := os.Readlink("/proc/self/ns/user")
	if err != nil {
		return false
	}
	theirs, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/user", pid))
	return err == nil && theirs != own
}

// decodeCapabilities converts a hex capability bitmask into named capabilities.
func decodeCapabilities(hex string) []string {
	val, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return nil
	}
	if val == 0 {
		return nil
	}

	var caps []string
	for bit := 0; bit < 64; bit++ {
		if val&(1<<uint(bit)) != 0 {
			if name, ok := capNames[bit]; ok {
				caps = append(caps, name)
			}
		}
	}
	return caps
}
