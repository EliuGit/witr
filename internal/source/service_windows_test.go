//go:build windows

package source

import (
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

// A process under a shared host isn't credited to one of its services.
func TestDetectWindowsServiceSharedHost(t *testing.T) {
	ancestry := []model.Process{
		{PID: 1372, Command: "services.exe"},
		{PID: 1736, PPID: 1372, Command: "svchost.exe", Service: "RpcEptMapper, RpcSs"},
		{PID: 4000, PPID: 1736, Command: "worker.exe"},
	}
	src := detectWindowsService(ancestry)
	if src == nil || src.Name != "RpcEptMapper, RpcSs" || src.UnitFile != "" || !strings.Contains(src.Description, "Shared service host (svchost.exe, pid 1736)") {
		t.Errorf("detectWindowsService = %+v", src)
	}
}
