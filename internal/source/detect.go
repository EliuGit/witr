package source

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pranshuparmar/witr/pkg/model"
)

var suspiciousDirs = map[string]bool{"/": true, "/tmp": true, "/var/tmp": true}

var dangerousCapabilities = map[string]bool{
	"CAP_SYS_ADMIN":       true,
	"CAP_SYS_PTRACE":      true,
	"CAP_NET_RAW":         true,
	"CAP_DAC_OVERRIDE":    true,
	"CAP_DAC_READ_SEARCH": true,
	"CAP_FOWNER":          true,
	"CAP_SYS_MODULE":      true,
	"CAP_SYS_RAWIO":       true,
}

func isDangerousCapability(cap string) bool {
	return dangerousCapabilities[cap]
}

type envSuspiciousRule struct {
	pattern     string
	match       func(key, pattern string) bool
	warning     string
	includeKeys bool
}

var (
	envVarRules = []envSuspiciousRule{
		{
			pattern: "LD_PRELOAD",
			match:   func(key, pattern string) bool { return key == pattern },
			warning: "Process sets LD_PRELOAD (potential library injection)",
		},

		{
			pattern:     "DYLD_",
			match:       strings.HasPrefix,
			warning:     "Process sets DYLD_* variables (potential library injection)",
			includeKeys: true,
		},
	}
)

const (
	// orphanedDescription explains an unknown source whose chain was cut
	// where the process that started the target exited.
	orphanedDescription = "The process that started it has exited; what remains above it only adopted it"
	// adoptedByInitDescription qualifies an init source for such a chain.
	adoptedByInitDescription = "Adopted by init after the process that started it exited"
)

func Detect(ancestry []model.Process) model.Source {
	return forOrphan(detect(ancestry), ancestry)
}

// forOrphan adjusts src when the chain was cut where the process that started
// the target exited, so the source doesn't credit what merely adopted it.
func forOrphan(src model.Source, ancestry []model.Process) model.Source {
	cut := parentExitedAt(ancestry)
	switch {
	case cut < 0:
		return src
	case src.Type == model.SourceInit:
		// Without a richer service manager, daemons are routinely adopted by
		// init, so keep init as the source but say it only adopted the process.
		src.Description = adoptedByInitDescription
		return src
	case !explainsOrphan(src, ancestry[cut:]):
		return model.Source{Type: model.SourceUnknown, Description: orphanDescription(ancestry, cut)}
	}
	return src
}

// orphanDescription explains an unknown source whose chain was cut where the
// process that started the target exited. With nothing above the cut (Windows
// doesn't reparent orphans), nothing adopted it either.
func orphanDescription(ancestry []model.Process, cut int) string {
	switch {
	case cut > 0:
		return orphanedDescription
	case ancestry[0].PPID > 0:
		return fmt.Sprintf("The process that started it (pid %d) has exited", ancestry[0].PPID)
	}
	return "The process that started it has exited"
}

// parentExitedAt returns the index of the last process in the chain whose
// parent has exited, or -1 when the chain is intact.
func parentExitedAt(ancestry []model.Process) int {
	for i := len(ancestry) - 1; i >= 0; i-- {
		if ancestry[i].ParentExited {
			return i
		}
	}
	return -1
}

// Untraced reports whether a source of type st leaves what started the
// process unknown. Not on Windows: its ancestry routinely stops at a process
// whose parent exited (Windows leaves a stale PPID instead of reparenting to
// an init process), so an unknown source there is normal, not a finding.
func Untraced(st model.SourceType) bool {
	return st == model.SourceUnknown && runtime.GOOS != "windows"
}

// explainsOrphan reports whether src still names a real cause once the chain
// is cut at an exited parent, since everything above the cut merely adopted
// the process. Sources read from the process itself (its container, systemd
// service, launchd job or rc service) still hold; sources found from its
// ancestors only hold when found below the cut, the part of the chain that
// actually started it.
func explainsOrphan(src model.Source, below []model.Process) bool {
	switch src.Type {
	case model.SourceContainer:
		return true
	case model.SourceSystemd:
		// A service, login session or app launch scope says where the process
		// came from. init.scope (systemd itself, and every process under
		// WSL) and the user manager only say what adopted it.
		return src.Name != "" && src.Name != "init.scope" && !strings.HasPrefix(src.Name, "user@")
	case model.SourceLaunchd:
		return src.Name != "launchd"
	case model.SourceBsdRc:
		return src.Details["service"] != "" || src.UnitFile != ""
	case model.SourceShell:
		return detectShell(below) != nil
	case model.SourceSSH:
		return detectSSH(below) != nil
	case model.SourceSupervisor:
		return detectSupervisor(below) != nil
	case model.SourceCron:
		return detectCron(below) != nil
	case model.SourceWindowsService:
		return detectWindowsService(below) != nil
	}
	return false
}

func detect(ancestry []model.Process) model.Source {
	// Detection order prioritizes platform-specific init systems
	// over generic supervisor detection to avoid false positives
	if src := detectContainer(ancestry); src != nil {
		return *src
	}
	if src := detectSSH(ancestry); src != nil {
		return *src
	}
	if src := detectShell(ancestry); src != nil {
		return *src
	}
	if src := detectSystemd(ancestry); src != nil {
		return *src
	}
	if src := detectLaunchd(ancestry); src != nil {
		return *src
	}
	if src := detectBsdRc(ancestry); src != nil {
		return *src
	}
	if src := detectSupervisor(ancestry); src != nil {
		return *src
	}
	if src := detectCron(ancestry); src != nil {
		return *src
	}
	if src := detectWindowsService(ancestry); src != nil {
		return *src
	}
	if src := detectInit(ancestry); src != nil {
		return *src
	}

	return model.Source{
		Type: model.SourceUnknown,
	}
}

// env suspicious warnings returns warnings for known env based library injection patterns
func envSuspiciousWarnings(env []string) []string {
	matched := make([]bool, len(envVarRules))
	matchedKeys := make([]map[string]struct{}, len(envVarRules))

	// init per rule key capture only for rules that include keys
	for i, rule := range envVarRules {
		if rule.includeKeys {
			matchedKeys[i] = map[string]struct{}{}
		}
	}

	// scan env entries and record which rules match
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || value == "" {
			continue
		}

		// check this key against each configured rule
		for i, rule := range envVarRules {
			if !rule.match(key, rule.pattern) {
				continue
			}
			matched[i] = true
			if rule.includeKeys {
				matchedKeys[i][key] = struct{}{}
			}
		}
	}

	var warnings []string

	// emit warnings in the same order as envVarRules
	for i, rule := range envVarRules {
		if !matched[i] {
			continue
		}
		if !rule.includeKeys {
			warnings = append(warnings, rule.warning)
			continue
		}

		keys := make([]string, 0, len(matchedKeys[i]))
		// collect all matched keys for this rule
		for key := range matchedKeys[i] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		warnings = append(warnings, rule.warning+": "+strings.Join(keys, ", "))
	}

	return warnings
}

func Warnings(p []model.Process, restartCount int, srcType ...model.SourceType) []string {
	if len(p) == 0 {
		return nil
	}

	var w []string

	last := p[len(p)-1]

	// Warn on a service or container that has restarted many times.
	// restartCount is the real count from the service manager (e.g. systemd
	// NRestarts) or the container runtime, or 0 when unknown.
	if restartCount > 5 {
		what := "Service"
		if len(srcType) > 0 && srcType[0] == model.SourceContainer {
			what = "Container"
		}
		w = append(w, fmt.Sprintf("%s has restarted %d times", what, restartCount))
	}

	// Health warnings
	switch last.Health {
	case "zombie":
		w = append(w, "Process is a zombie (defunct)")
	case "stopped":
		w = append(w, "Process is stopped (T state)")
	case "high-cpu":
		w = append(w, "Process is using high CPU (>2h total)")
	case "high-mem":
		w = append(w, "Process is using high memory (>1GB RSS)")
	}

	if IsPublicBind(last.Sockets) {
		w = append(w, "Process is listening on a public interface")
	}

	if last.User == "root" {
		w = append(w, "Process is running as root")
	} else if len(last.Capabilities) > 0 {
		var dangerous []string
		for _, cap := range last.Capabilities {
			if isDangerousCapability(cap) {
				dangerous = append(dangerous, cap)
			}
		}
		if len(dangerous) > 0 {
			w = append(w, "Process has dangerous capabilities: "+strings.Join(dangerous, ", "))
		}
	}

	var st model.SourceType
	if len(srcType) > 0 {
		st = srcType[0]
	} else {
		st = Detect(p).Type
	}
	if Untraced(st) {
		if parentExitedAt(p) >= 0 {
			w = append(w, "Original parent process has exited, so what started this process can't be traced")
		} else {
			w = append(w, "No known supervisor or service manager detected")
		}
	}

	// Warn if process is very old (>90 days). A zero start time means we
	// couldn't read it (e.g. protected Windows processes), not that the process
	// is ancient — skip the warning rather than emit a false positive.
	if !last.StartedAt.IsZero() && time.Since(last.StartedAt).Hours() > 90*24 {
		w = append(w, "Process has been running for over 90 days")
	}

	// A container's working directory is a path inside the container's own
	// filesystem (usually "/"), so this host-path check doesn't apply.
	if suspiciousDirs[last.WorkingDir] && last.ContainerID == "" {
		w = append(w, "Process is running from a suspicious working directory: "+last.WorkingDir)
	}

	// Warn only when the runtime confirms no healthcheck is configured. Unknown
	// ("") — snap/flatpak, unprobed runtimes, non-Linux — does not warn.
	if last.ContainerHealthcheck == "absent" {
		w = append(w, "Container has no healthcheck configured")
	}

	// Warn if service name and process name are genuinely unrelated
	if last.Service != "" && last.Command != "" {
		svcCore := last.Service
		for _, suffix := range []string{".service", ".socket", ".timer", ".scope", ".slice", ".plist"} {
			svcCore = strings.TrimSuffix(svcCore, suffix)
		}
		// Compare against a systemd template's base name, not its instance
		// (getty@tty1 -> getty), so a template whose binary is named after the
		// template (agetty) doesn't read as a mismatch.
		if at := strings.IndexByte(svcCore, '@'); at >= 0 {
			svcCore = svcCore[:at]
		}
		svcCore = strings.ToLower(svcCore)
		cmdBase := strings.ToLower(last.Command)
		if !strings.Contains(svcCore, cmdBase) && !strings.Contains(cmdBase, svcCore) {
			w = append(w, "Service name and process name do not match")
		}
	}

	// Warn if binary is deleted
	if last.ExeDeleted {
		w = append(w, "Process is running from a deleted binary (potential library injection or pending update)")
	}

	// Include warnings based on suspicious env variables
	w = append(w, envSuspiciousWarnings(last.Env)...)

	return w
}

// EnrichSocketInfo provides human-readable explanations and workarounds for socket states
func EnrichSocketInfo(si *model.SocketInfo) {
	if si == nil {
		return
	}

	switch si.State {
	case "TIME_WAIT":
		si.Explanation = "The local OS is holding the port in a protocol-wait state to ensure all packets are received."
		si.Workaround = "Wait ~60s for the OS to release it, or enable SO_REUSEADDR in your code."
	case "CLOSE_WAIT":
		si.Explanation = "The remote end has closed the connection, but the local application hasn't responded."
		si.Workaround = "This usually indicates a resource leak in the application. Restart the process."
	case "FIN_WAIT_1", "FIN_WAIT_2":
		si.Explanation = "The connection is in the process of being closed."
	case "ESTABLISHED":
		si.Explanation = "The connection is active and data can be transferred."
	case "LISTEN":
		si.Explanation = "The process is actively waiting for incoming connections."
	}
}
