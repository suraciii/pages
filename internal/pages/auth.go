package pages

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ValidName reports whether a path segment is safe for an identity or slug.
func ValidName(value string) bool {
	return namePattern.MatchString(value)
}

// GenerateSecret returns a new 32-byte token secret in unpadded base64url.
// The encoding never contains ".".
func GenerateSecret() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

// TokenIdentity returns the identity embedded in an identity.secret token.
func TokenIdentity(token string) (string, error) {
	identity, secret, found := strings.Cut(token, ".")
	if !found || secret == "" || strings.Contains(secret, ".") || !ValidName(identity) {
		return "", fmt.Errorf("token must use identity.random-secret format")
	}
	return identity, nil
}

// Tokens maps an identity to its private token secret.
type Tokens map[string]string

// LoadTokens reads a JSON object mapping safe identities to non-empty secrets.
func LoadTokens(path string) (Tokens, error) {
	tokens, err := ReadTokensFile(path)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("tokens file must contain at least one identity")
	}
	return tokens, nil
}

// ReadTokensFile reads a JSON token map. A missing file yields an empty
// map.
func ReadTokensFile(path string) (Tokens, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Tokens{}, nil
		}
		return nil, fmt.Errorf("read tokens file: %w", err)
	}

	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("parse tokens file: %w", err)
	}
	for identity, secret := range tokens {
		if !ValidName(identity) || secret == "" || strings.Contains(secret, ".") {
			return nil, fmt.Errorf("invalid token entry for identity %q", identity)
		}
	}
	return tokens, nil
}

// WriteTokensFile saves the token map atomically with mode 0600, keeping
// every other entry of the target file.
func WriteTokensFile(path string, tokens Tokens) error {
	if len(tokens) == 0 {
		return fmt.Errorf("tokens file must contain at least one identity")
	}
	for identity, secret := range tokens {
		if !ValidName(identity) || secret == "" || strings.Contains(secret, ".") {
			return fmt.Errorf("invalid token entry for identity %q", identity)
		}
	}
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tokens file: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".tokens-*.json")
	if err != nil {
		return fmt.Errorf("create temporary tokens file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set tokens file mode: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write tokens file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close tokens file: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace tokens file: %w", err)
	}
	return nil
}

// Authenticate validates an Authorization header and returns its verified identity.
func (tokens Tokens) Authenticate(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	token := strings.TrimPrefix(header, prefix)
	identity, err := TokenIdentity(token)
	if err != nil {
		return "", false
	}
	_, secret, _ := strings.Cut(token, ".")
	expected, found := tokens[identity]
	if !found {
		return "", false
	}

	// Hashing makes both operands a fixed length before the constant-time compare.
	expectedHash := sha256.Sum256([]byte(expected))
	providedHash := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) != 1 {
		return "", false
	}
	return identity, true
}
