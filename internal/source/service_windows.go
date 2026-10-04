//go:build windows

package source

import (
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
			registryKey := `HKLM\SYSTEM\CurrentControlSet\Services\` + p.Service
			description := serviceDescription(p.Service)

			return &model.Source{
				Type:        model.SourceWindowsService,
				Name:        p.Service,
				Description: description,
				UnitFile:    registryKey,
				Details: map[string]string{
					"manager": "services.exe",
					"service": p.Service,
				},
			}
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
