//go:build windows

package source

import (
	"fmt"
	"strings"

	procpkg "github.com/pranshuparmar/witr/internal/proc"
	"github.com/pranshuparmar/witr/pkg/model"
)

// serviceDescription returns a service's display name, or "" when it only
// repeats the service name (MySQL80 is displayed as "MySQL80").
func serviceDescription(name string) string {
	if d := procpkg.ServiceDisplayName(name); !strings.EqualFold(d, name) {
		return d
	}
	return ""
}

func detectWindowsService(ancestry []model.Process) *model.Source {
	// 1. Check for explicit service name in process metadata (prioritize target)
	for i := len(ancestry) - 1; i >= 0; i-- {
		p := ancestry[i]
		if p.Service != "" {
			src := &model.Source{
				Type: model.SourceWindowsService,
				Name: p.Service,
				Details: map[string]string{
					"manager": "services.exe",
					"service": p.Service,
				},
			}
			if strings.Contains(p.Service, ", ") {
				// A shared host runs several services; no one of them alone
				// started what runs under it.
				src.Description = fmt.Sprintf("Shared service host (%s, pid %d)", p.Command, p.PID)
			} else {
				src.Description = serviceDescription(p.Service)
				src.UnitFile = `HKLM\SYSTEM\CurrentControlSet\Services\` + p.Service
			}
			return src
		}
	}

	// 2. Fallback: Check if services.exe is in ancestry without explicit service name
	for _, p := range ancestry {
		if strings.ToLower(p.Command) == "services.exe" {
			return &model.Source{
				Type: model.SourceWindowsService,
				Name: "Service Control Manager",
				Details: map[string]string{
					"manager": "services.exe",
				},
			}
		}
	}

	// 3. Check for children of services.exe where valid service name wasn't found
	if len(ancestry) >= 2 {
		parent := ancestry[len(ancestry)-2]
		target := ancestry[len(ancestry)-1]
		if strings.ToLower(parent.Command) == "services.exe" {
			name := strings.TrimSuffix(target.Command, ".exe")

			registryKey := `HKLM\SYSTEM\CurrentControlSet\Services\` + name
			description := serviceDescription(name)

			return &model.Source{
				Type:        model.SourceWindowsService,
				Name:        name,
				Description: description,
				UnitFile:    registryKey,
				Details: map[string]string{
					"manager": "services.exe",
				},
			}
		}
	}

	return nil
}
