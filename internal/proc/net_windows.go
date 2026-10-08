package proc

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/pranshuparmar/witr/pkg/model"
)

// Sockets come from the IP Helper tables rather than `netstat -ano`: netstat
// localizes state names and prints them in the console's OEM code page, while
// the tables carry numeric states and binary addresses in every locale.
var (
	modiphlpapi             = syscall.NewLazyDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")
)

const (
	afInet              = 2
	afInet6             = 23
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1
)

// WinSocket is one row of the system TCP or UDP table.
type WinSocket struct {
	Protocol   string // "TCP" or "UDP" for both address families, as netstat names them
	LocalIP    string
	LocalPort  int
	RemoteIP   string
	RemotePort int
	State      string
	PID        int
}

// tcpStates maps MIB_TCP_STATE values to the names used on every platform.
var tcpStates = map[uint32]string{
	1: "CLOSED", 2: "LISTEN", 3: "SYN_SENT", 4: "SYN_RECEIVED", 5: "ESTABLISHED",
	6: "FIN_WAIT_1", 7: "FIN_WAIT_2", 8: "CLOSE_WAIT", 9: "CLOSING", 10: "LAST_ACK",
	11: "TIME_WAIT", 12: "DELETE_TCB",
}

// ListSockets returns every TCP and UDP socket on the system with its owning
// PID. A missing IPv6 stack is not an error.
func ListSockets() ([]WinSocket, error) {
	tables := []struct {
		proc  *syscall.LazyProc
		af    uint32
		class uint32
		parse func([]byte) []WinSocket
	}{
		{procGetExtendedTcpTable, afInet, tcpTableOwnerPIDAll, parseTCP4Table},
		{procGetExtendedTcpTable, afInet6, tcpTableOwnerPIDAll, parseTCP6Table},
		{procGetExtendedUdpTable, afInet, udpTableOwnerPID, parseUDP4Table},
		{procGetExtendedUdpTable, afInet6, udpTableOwnerPID, parseUDP6Table},
	}

	var socks []WinSocket
	for _, t := range tables {
		buf, err := extendedTable(t.proc, t.af, t.class)
		if err != nil {
			if t.af == afInet6 {
				continue
			}
			return nil, err
		}
		socks = append(socks, t.parse(buf)...)
	}
	return socks, nil
}

// extendedTable fetches one IP Helper table, growing the buffer until the
// table fits (it can grow between the size query and the read).
func extendedTable(proc *syscall.LazyProc, af, class uint32) ([]byte, error) {
	var size uint32
	for range 5 {
		var buf []byte
		var p unsafe.Pointer
		if size > 0 {
			buf = make([]byte, size)
			p = unsafe.Pointer(&buf[0])
		}
		r, _, _ := proc.Call(uintptr(p), uintptr(unsafe.Pointer(&size)), 0, uintptr(af), uintptr(class), 0)
		switch errno := syscall.Errno(r); errno {
		case 0:
			return buf, nil
		case syscall.ERROR_INSUFFICIENT_BUFFER:
			continue
		default:
			return nil, fmt.Errorf("%s: %w", proc.Name, errno)
		}
	}
	return nil, fmt.Errorf("%s: table kept growing", proc.Name)
}

// tableRows calls fn for each fixed-size row of a table laid out as a uint32
// row count followed by the rows.
func tableRows(buf []byte, rowSize int, fn func(row []byte)) {
	if len(buf) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(buf))
	for i := range n {
		off := 4 + i*rowSize
		if off+rowSize > len(buf) {
			return
		}
		fn(buf[off : off+rowSize])
	}
}

// MIB_TCPROW_OWNER_PID: state, local addr, local port, remote addr, remote port, pid.
func parseTCP4Table(buf []byte) (socks []WinSocket) {
	tableRows(buf, 24, func(r []byte) {
		socks = append(socks, WinSocket{
			Protocol:   "TCP",
			State:      tcpState(le32(r[0:])),
			LocalIP:    ip4(r[4:8]),
			LocalPort:  tablePort(r[8:]),
			RemoteIP:   ip4(r[12:16]),
			RemotePort: tablePort(r[16:]),
			PID:        int(le32(r[20:])),
		})
	})
	return socks
}

// MIB_TCP6ROW_OWNER_PID: local addr, scope, port, remote addr, scope, port, state, pid.
func parseTCP6Table(buf []byte) (socks []WinSocket) {
	tableRows(buf, 56, func(r []byte) {
		socks = append(socks, WinSocket{
			Protocol:   "TCP",
			LocalIP:    ip6(r[0:16], le32(r[16:])),
			LocalPort:  tablePort(r[20:]),
			RemoteIP:   ip6(r[24:40], le32(r[40:])),
			RemotePort: tablePort(r[44:]),
			State:      tcpState(le32(r[48:])),
			PID:        int(le32(r[52:])),
		})
	})
	return socks
}

// MIB_UDPROW_OWNER_PID: local addr, local port, pid.
func parseUDP4Table(buf []byte) (socks []WinSocket) {
	tableRows(buf, 12, func(r []byte) {
		socks = append(socks, WinSocket{
			Protocol:  "UDP",
			State:     "OPEN",
			LocalIP:   ip4(r[0:4]),
			LocalPort: tablePort(r[4:]),
			PID:       int(le32(r[8:])),
		})
	})
	return socks
}

// MIB_UDP6ROW_OWNER_PID: local addr, scope, port, pid.
func parseUDP6Table(buf []byte) (socks []WinSocket) {
	tableRows(buf, 28, func(r []byte) {
		socks = append(socks, WinSocket{
			Protocol:  "UDP",
			State:     "OPEN",
			LocalIP:   ip6(r[0:16], le32(r[16:])),
			LocalPort: tablePort(r[20:]),
			PID:       int(le32(r[24:])),
		})
	})
	return socks
}

func tcpState(v uint32) string {
	if s, ok := tcpStates[v]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN (%d)", v)
}

func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

// tablePort reads a port, which the tables store in network byte order in
// the low 16 bits of a DWORD.
func tablePort(b []byte) int { return int(binary.BigEndian.Uint16(b)) }

func ip4(b []byte) string { return net.IPv4(b[0], b[1], b[2], b[3]).String() }

func ip6(b []byte, scope uint32) string {
	s := net.IP(append([]byte(nil), b...)).String()
	if scope != 0 {
		s += "%" + strconv.FormatUint(uint64(scope), 10)
	}
	return s
}

func ListOpenPorts() ([]model.OpenPort, error) {
	socks, err := ListSockets()
	if err != nil {
		return nil, err
	}

	var ports []model.OpenPort
	seen := make(map[string]bool)
	for _, s := range socks {
		key := fmt.Sprintf("%d|%s|%s|%d|%s", s.PID, s.Protocol, s.LocalIP, s.LocalPort, s.State)
		if seen[key] {
			continue
		}
		seen[key] = true
		ports = append(ports, model.OpenPort{
			PID:      s.PID,
			Port:     s.LocalPort,
			Address:  s.LocalIP,
			Protocol: s.Protocol,
			State:    s.State,
		})
	}
	return ports, nil
}

// GetSocketsForPID returns every IP socket owned by a PID, including
// non-listening sockets.
func GetSocketsForPID(pid int) []model.Socket {
	socks, err := ListSockets()
	if err != nil {
		return nil
	}

	var sockets []model.Socket
	seen := make(map[string]bool)
	for _, s := range socks {
		if s.PID != pid {
			continue
		}
		key := s.Protocol + "|" + s.LocalIP + "|" + strconv.Itoa(s.LocalPort) + "|" + s.State + "|" + s.RemoteIP + "|" + strconv.Itoa(s.RemotePort)
		if seen[key] {
			continue
		}
		seen[key] = true
		sock := model.Socket{
			Port:     s.LocalPort,
			Address:  s.LocalIP,
			Protocol: s.Protocol,
			State:    s.State,
		}
		if s.RemotePort > 0 {
			sock.RemoteAddress, sock.RemotePort = s.RemoteIP, s.RemotePort
		}
		sockets = append(sockets, sock)
	}
	return sockets
}
