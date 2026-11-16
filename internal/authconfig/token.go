package authconfig

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

const passwordRounds = 120000

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil { return "", err }
	sum := derivePassword(password, salt, passwordRounds)
	return "pbkdf2$"+base64.RawURLEncoding.EncodeToString(salt)+"$"+base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func VerifyPassword(password, encoded string) bool {
	if strings.HasPrefix(encoded, "plain$") {
		sum := sha256.Sum256([]byte("nodehub-admin-v1:" + password))
		return strings.EqualFold(encoded, "plain$"+hex.EncodeToString(sum[:]))
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "pbkdf2" { return false }
	salt, err1 := base64.RawURLEncoding.DecodeString(parts[1])
	want, err2 := base64.RawURLEncoding.DecodeString(parts[2])
	if err1 != nil || err2 != nil || len(want) != sha256.Size { return false }
	have := derivePassword(password, salt, passwordRounds)
	return strings.EqualFold(base64.RawURLEncoding.EncodeToString(have[:]), parts[2])
}

func derivePassword(password string, salt []byte, rounds int) [sha256.Size]byte {
	state := sha256.Sum256(append(append([]byte{}, salt...), []byte(password)...))
	for i := 1; i < rounds; i++ { state = sha256.Sum256(append(state[:], []byte(password)...)) }
	return state
}


// Reject distributable example values even when their length passes the guard.
func ValidAdminToken(token string) bool {
	if len(token) < 32 || strings.TrimSpace(token) != token {
		return false
	}
	lower := strings.ToLower(token)
	for _, marker := range []string{"generate-", "change-me", "changeme", "example", "replace-me", "your-admin"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return strings.Trim(token, string(token[0])) != ""
}
