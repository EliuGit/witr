//go:build darwin

package proc

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pranshuparmar/witr/pkg/model"
)

func init() { registerRuntime(appleContainerRuntime{}) }

type appleContainerRuntime struct{}

func (appleContainerRuntime) Name() string    { return "container" }
func (appleContainerRuntime) Available() bool { return binAvailable("container") }

func (appleContainerRuntime) List() []*model.ContainerMatch {
	return appleContainerList()
}

// HostPID returns 0 because Apple container runs containers as lightweight VMs
// using the macOS Virtualization framework; there is no direct host PID mapping
// for container processes.
func (appleContainerRuntime) HostPID(id string) int { return 0 }

// ---------------------------------------------------------------------------
// JSON structures matching `container ls --format json --all` output
// ---------------------------------------------------------------------------

// appleContainerEntry mirrors one entry of `container ls --format json`.
// Current releases nest the runtime state under status; older ones write
// status as a bare state string and keep networks and the start date at the
// top level.
type appleContainerEntry struct {
	Configuration appleContainerConfig `json:"configuration"`
	Status        appleContainerStatus `json:"status"`
	Networks      []appleNetwork       `json:"networks"`
	StartedDate   string               `json:"startedDate"`
}

type appleContainerConfig struct {
	ID             string               `json:"id"`
	Image          appleImageDesc       `json:"image"`
	Platform       applePlatform        `json:"platform"`
	InitProcess    appleProcessConfig   `json:"initProcess"`
	Resources      appleResources       `json:"resources"`
	Mounts         []appleMount         `json:"mounts"`
	PublishedPorts []applePublishedPort `json:"publishedPorts"`
	Networks       []appleNetworkConfig `json:"networks"`
	CreationDate   string               `json:"creationDate"`
}

type appleImageDesc struct {
	Reference string `json:"reference"`
}

type applePlatform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type appleProcessConfig struct {
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}

type appleResources struct {
	CPUs          int    `json:"cpus"`
	MemoryInBytes uint64 `json:"memoryInBytes"`
}

type appleMount struct {
	HostPath      string `json:"source"`
	ContainerPath string `json:"destination"`
}

// applePublishedPort publishes Count consecutive ports (1 when unset).
type applePublishedPort struct {
	HostAddress   json.RawMessage `json:"hostAddress"`
	HostPort      uint16          `json:"hostPort"`
	ContainerPort uint16          `json:"containerPort"`
	Protocol      string          `json:"proto"`
	Count         uint16          `json:"count"`
}

type appleNetworkConfig struct {
	Network string `json:"network"`
}

type appleContainerStatus struct {
	State       string         `json:"state"`
	Networks    []appleNetwork `json:"networks"`
	StartedDate string         `json:"startedDate"`
}

// UnmarshalJSON accepts both status shapes: the object current releases write
// and the bare state string older releases write.
func (s *appleContainerStatus) UnmarshalJSON(data []byte) error {
	var state string
	if err := json.Unmarshal(data, &state); err == nil {
		*s = appleContainerStatus{State: state}
		return nil
	}
	type status appleContainerStatus
	return json.Unmarshal(data, (*status)(s))
}

type appleNetwork struct {
	Network     string `json:"network"`
	Hostname    string `json:"hostname"`
	IPv4Address string `json:"ipv4Address"`
	IPv4Gateway string `json:"ipv4Gateway"`
}

// ---------------------------------------------------------------------------
// List implementation
// ---------------------------------------------------------------------------

func appleContainerList() []*model.ContainerMatch {
	ctx, cancel := context.WithTimeout(context.Background(), runtimeQueryTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "container", "ls", "--format", "json", "--all").Output()
	if err != nil {
		return nil
	}

	entries, err := parseAppleContainerEntries(out)
	if err != nil {
		return nil
	}

	matches := make([]*model.ContainerMatch, 0, len(entries))
	for _, e := range entries {
		cfg := e.Configuration
		status := e.Status
		networks := status.Networks
		if len(networks) == 0 {
			networks = e.Networks
		}
		started := status.StartedDate
		if started == "" {
			started = e.StartedDate
		}

		m := &model.ContainerMatch{
			Runtime:   "container",
			ID:        cfg.ID,
			Name:      cfg.ID, // Apple container uses ID as name
			Image:     cfg.Image.Reference,
			Command:   buildCommand(cfg.InitProcess),
			State:     status.State,
			Status:    status.State,
			CreatedAt: parseRFC3339(cfg.CreationDate),
			StartedAt: parseRFC3339(started),
			Networks:  buildNetworks(networks),
			Mounts:    buildMounts(cfg.Mounts),
			Ports:     buildPorts(cfg.PublishedPorts),
		}
		matches = append(matches, m)
	}
	return matches
}

// parseAppleContainerEntries decodes `container ls` output, skipping entries
// it can't read rather than dropping the whole list.
func parseAppleContainerEntries(data []byte) ([]appleContainerEntry, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entries := make([]appleContainerEntry, 0, len(raw))
	for _, r := range raw {
		var e appleContainerEntry
		if err := json.Unmarshal(r, &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildCommand(p appleProcessConfig) string {
	if p.Executable == "" {
		return ""
	}
	if len(p.Arguments) == 0 {
		return p.Executable
	}
	return p.Executable + " " + strings.Join(p.Arguments, " ")
}

// buildNetworks lists network:address pairs, dropping the /prefix the runtime
// reports addresses with.
func buildNetworks(nets []appleNetwork) string {
	parts := make([]string, 0, len(nets))
	for _, n := range nets {
		if addr, _, _ := strings.Cut(n.IPv4Address, "/"); addr != "" {
			parts = append(parts, n.Network+":"+addr)
		}
	}
	return strings.Join(parts, ", ")
}

func buildMounts(mounts []appleMount) string {
	parts := make([]string, 0, len(mounts))
	for _, m := range mounts {
		parts = append(parts, m.HostPath+":"+m.ContainerPath)
	}
	return strings.Join(parts, ", ")
}

func buildPorts(ports []applePublishedPort) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, formatPort(p))
	}
	return strings.Join(parts, ", ")
}

// formatPort renders a published port the way docker does, e.g.
// 0.0.0.0:8000-8002->80-82/tcp for a range of three.
func formatPort(p applePublishedPort) string {
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	host := portRange(p.HostPort, p.Count)
	if addr := jsonString(p.HostAddress); addr != "" {
		if strings.Contains(addr, ":") {
			addr = "[" + addr + "]"
		}
		host = addr + ":" + host
	}
	return host + "->" + portRange(p.ContainerPort, p.Count) + "/" + proto
}

// portRange renders count consecutive ports starting at first.
func portRange(first, count uint16) string {
	if count <= 1 {
		return itoaU16(first)
	}
	return itoaU16(first) + "-" + itoaU16(first+count-1)
}

// jsonString returns raw when it is a JSON string, or "".
func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func parseRFC3339(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func itoaU16(n uint16) string {
	return strconv.FormatUint(uint64(n), 10)
}
