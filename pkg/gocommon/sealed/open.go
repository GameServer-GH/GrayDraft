package sealed

import (
	"crypto/aes"
	"crypto/cipher"
)

func open(nonce, box []byte) string {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, nonce, box, nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

// DeepSeek returns the built-in DeepSeek API key.
func DeepSeek() string { return open(deepseekNonce, deepseekBox) }

// Intern returns the built-in Intern API key.
func Intern() string { return open(internNonce, internBox) }

// DashScope returns the built-in DashScope API key.
func DashScope() string { return open(dashscopeNonce, dashscopeBox) }

// TLSKey returns the built-in TLS private key PEM.
func TLSKey() string { return open(tlsNonce, tlsBox) }
