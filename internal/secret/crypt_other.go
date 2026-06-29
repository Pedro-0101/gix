//go:build !windows

package secret

// Fora do Windows ainda não há cofre nativo ligado: o segredo é gravado em texto
// (modo 0600). Linux (secret-service/libsecret) e macOS (Keychain) entram numa
// fase futura sem mexer no contrato de Store.
func protect(data []byte) ([]byte, error)   { return data, nil }
func unprotect(data []byte) ([]byte, error) { return data, nil }
