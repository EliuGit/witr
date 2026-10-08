//go:build linux

package source

import (
	"os"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestDetectContainerNerdctl(t *testing.T) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Skipf("cannot read own cgroup: %v", err)
	}
	for _, marker := range []string{"docker", "podman", "libpod", "kubepods", "colima", "containerd", "lxc.payload"} {
		if strings.Contains(string(data), marker) {
			t.Skipf("test process already runs in a %s cgroup", marker)
		}
	}

	// A nerdctl container's cgroup never names containerd, so the source
	// relies on the runtime the process reader recognised.
	src := detectContainer([]model.Process{{PID: os.Getpid(), ContainerRuntime: "nerdctl"}})
	if src == nil || src.Type != model.SourceContainer || src.Name != "containerd" {
		t.Errorf("detectContainer = %+v, want a containerd container source", src)
	}
}
