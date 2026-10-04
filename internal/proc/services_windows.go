//go:build windows

package proc

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	scManagerEnumerateService uint32 = 0x0004
	scEnumProcessInfo         uint32 = 0
	serviceWin32              uint32 = 0x00000030
	serviceStateAll           uint32 = 0x00000003
)

var (
	modadvapi32               = syscall.NewLazyDLL("advapi32.dll")
	procOpenSCManagerW        = modadvapi32.NewProc("OpenSCManagerW")
	procCloseServiceHandle    = modadvapi32.NewProc("CloseServiceHandle")
	procEnumServicesStatusExW = modadvapi32.NewProc("EnumServicesStatusExW")
)

type serviceStatusProcess struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
	ProcessId               uint32
	ServiceFlags            uint32
}

type enumServiceStatusProcessW struct {
	ServiceName          *uint16
	DisplayName          *uint16
	ServiceStatusProcess serviceStatusProcess
}

// serviceTable is one scan of the installed Windows services.
type serviceTable struct {
	byPID   map[int]string    // running service PID → its serviceLabel
	display map[string]string // lower-cased service name → display name
}

var (
	serviceMapCache     *serviceTable
	serviceMapCacheTime time.Time
	serviceMapCacheMu   sync.Mutex
	serviceMapCacheTTL  = 2 * time.Second
)

// serviceMapForPIDs returns PID → the service each running service host
// stands for (see serviceLabel).
func serviceMapForPIDs() (map[int]string, error) {
	t, err := scanServices()
	if err != nil {
		return nil, err
	}
	return t.byPID, nil
}

// ServiceDisplayName returns the name Windows shows for a service, as in the
// Services console, or "" if no such service is installed.
func ServiceDisplayName(name string) string {
	t, err := scanServices()
	if err != nil {
		return ""
	}
	return t.display[strings.ToLower(name)]
}

// scanServices lists the installed Windows services. Cached so an ancestry
// walk pays one SCM scan, not N.
func scanServices() (*serviceTable, error) {
	serviceMapCacheMu.Lock()
	defer serviceMapCacheMu.Unlock()

	if serviceMapCache != nil && time.Since(serviceMapCacheTime) < serviceMapCacheTTL {
		return serviceMapCache, nil
	}

	scm, _, callErr := procOpenSCManagerW.Call(0, 0, uintptr(scManagerEnumerateService))
	if scm == 0 {
		return nil, fmt.Errorf("OpenSCManager: %w", callErr)
	}
	defer procCloseServiceHandle.Call(scm)

	// First call with a zero buffer probes for the required size.
	var bytesNeeded, count, resume uint32
	procEnumServicesStatusExW.Call(
		scm,
		uintptr(scEnumProcessInfo),
		uintptr(serviceWin32),
		uintptr(serviceStateAll),
		0, 0,
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&resume)),
		0,
	)
	if bytesNeeded == 0 {
		serviceMapCache = &serviceTable{byPID: map[int]string{}, display: map[string]string{}}
		serviceMapCacheTime = time.Now()
		return serviceMapCache, nil
	}

	buf := make([]byte, bytesNeeded)
	count = 0
	resume = 0
	ret, _, callErr := procEnumServicesStatusExW.Call(
		scm,
		uintptr(scEnumProcessInfo),
		uintptr(serviceWin32),
		uintptr(serviceStateAll),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(bytesNeeded),
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&resume)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("EnumServicesStatusEx: %w", callErr)
	}

	t := &serviceTable{byPID: make(map[int]string, count), display: make(map[string]string, count)}
	hosted := make(map[int][]string)
	entrySize := unsafe.Sizeof(enumServiceStatusProcessW{})
	base := unsafe.Pointer(&buf[0])
	for i := uintptr(0); i < uintptr(count); i++ {
		entry := (*enumServiceStatusProcessW)(unsafe.Pointer(uintptr(base) + i*entrySize))
		name := utf16PtrToString(entry.ServiceName)
		if name == "" {
			continue
		}
		t.display[strings.ToLower(name)] = utf16PtrToString(entry.DisplayName)
		pid := int(entry.ServiceStatusProcess.ProcessId)
		if pid == 0 {
			// Service registered but not currently running.
			continue
		}
		hosted[pid] = append(hosted[pid], name)
	}
	for pid, names := range hosted {
		t.byPID[pid] = serviceLabel(names)
	}

	serviceMapCache = t
	serviceMapCacheTime = time.Now()
	return t, nil
}

// serviceLabel names the service a host process stands for: its only
// service; DcomLaunch when a shared host runs it, since that service starts
// COM servers and packaged apps, the host's usual children; otherwise every
// service the host runs, as no single one can be credited.
func serviceLabel(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	if slices.Contains(names, "DcomLaunch") {
		return "DcomLaunch"
	}
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	return strings.Join(sorted, ", ")
}

// utf16PtrToString converts a null-terminated UTF-16 pointer to a Go string.
func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
}
