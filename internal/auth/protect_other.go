//go:build !windows

package auth

// The tool only ships for windows/amd64 (T3); this build stays for local
// development and tests on other platforms, where DPAPI does not exist, so
// the credential is kept as written rather than protected at rest.

func Protect(plain []byte) ([]byte, error) {
	return plain, nil
}

func Unprotect(protected []byte) ([]byte, error) {
	return protected, nil
}
