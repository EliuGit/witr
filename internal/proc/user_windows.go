//go:build windows

package proc

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// readUser returns the account a process runs as, or "unknown".
func readUser(pid int) string {
	user, _ := readTokenInfo(pid)
	return user
}

// readTokenInfo reads a process's token: the account it runs as ("unknown"
// when the token can't be read) and its integrity level ("" then).
func readTokenInfo(pid int) (user, integrity string) {
	// PROCESS_QUERY_LIMITED_INFORMATION is enough to open the token and is
	// granted for more processes than PROCESS_QUERY_INFORMATION.
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if h, err = windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(pid)); err != nil {
			return "unknown", ""
		}
	}
	defer func() { _ = windows.CloseHandle(h) }()

	var token windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token); err != nil {
		return "unknown", ""
	}
	defer func() { _ = token.Close() }()

	user = "unknown"
	if tu, err := token.GetTokenUser(); err == nil {
		if name, domain, _, err := tu.User.Sid.LookupAccount(""); err == nil {
			user = name
			if domain != "" {
				user = domain + `\` + name
			}
		}
	}
	return user, tokenIntegrity(token)
}

// tokenIntegrity returns a token's integrity level, or "" when it can't be
// read.
func tokenIntegrity(token windows.Token) string {
	var n uint32
	_ = windows.GetTokenInformation(token, windows.TokenIntegrityLevel, nil, 0, &n)
	if n == 0 {
		return ""
	}
	buf := make([]byte, n)
	if windows.GetTokenInformation(token, windows.TokenIntegrityLevel, &buf[0], n, &n) != nil {
		return ""
	}
	sid := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buf[0])).Label.Sid
	count := sid.SubAuthorityCount()
	if count == 0 {
		return ""
	}
	return integrityName(sid.SubAuthority(uint32(count) - 1))
}

// integrityName maps a mandatory label's RID to its integrity level.
func integrityName(rid uint32) string {
	switch {
	case rid < 0x1000:
		return "Untrusted"
	case rid < 0x2000:
		return "Low"
	case rid < 0x3000: // includes Medium Plus
		return "Medium"
	case rid < 0x4000:
		return "High"
	case rid < 0x5000:
		return "System"
	}
	return "Protected"
}
