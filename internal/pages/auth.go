package pages

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/suraciii/pages/internal/filesystem"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
var secretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var tokenUpdateMu sync.Mutex

// ErrTokenExists means IssueToken refused to replace an existing Token.
var ErrTokenExists = errors.New("token already exists")

// ValidName reports whether a path segment is safe for an identity or slug.
func ValidName(value string) bool {
	return namePattern.MatchString(value)
}

// ValidSecret reports whether a token secret is safe in the Token and HTTP
// Authorization header formats.
func ValidSecret(value string) bool {
	return secretPattern.MatchString(value)
}

// IdentityScope returns the public path segment for a Named Identity. The
// Default Identity has no segment.
func IdentityScope(identity string) string {
	if identity == "" {
		return ""
	}
	return "@" + identity
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

// TokenIdentity returns the Named Identity embedded in a Token. A pure-secret
// Default Identity Token returns an empty Identity.
func TokenIdentity(token string) (string, error) {
	identity, _, err := parseToken(token)
	return identity, err
}

func parseToken(token string) (identity, secret string, err error) {
	if token == "" {
		return "", "", fmt.Errorf("token must not be empty")
	}
	identity, secret, named := strings.Cut(token, ".")
	if !named {
		if !ValidSecret(token) {
			return "", "", fmt.Errorf("token must use secret or identity.secret format")
		}
		return "", token, nil
	}
	if !ValidName(identity) || !ValidSecret(secret) {
		return "", "", fmt.Errorf("token must use secret or identity.secret format")
	}
	return identity, secret, nil
}

// Tokens stores the Default Identity Token and optional Named Identity
// secrets.
type Tokens struct {
	Token      string            `json:"token,omitempty"`
	Identities map[string]string `json:"identities,omitempty"`
}

func (tokens Tokens) validate(requireToken bool) error {
	if tokens.Token != "" && !ValidSecret(tokens.Token) {
		return fmt.Errorf("default token secret must use base64url characters")
	}
	for identity, secret := range tokens.Identities {
		if !ValidName(identity) || !ValidSecret(secret) {
			return fmt.Errorf("invalid token entry for identity %q", identity)
		}
	}
	if requireToken && tokens.Token == "" && len(tokens.Identities) == 0 {
		return fmt.Errorf("tokens file must contain a default or named identity token")
	}
	return nil
}

// LoadTokens reads a non-empty tokens file through fileSystem.
func LoadTokens(fileSystem filesystem.FS, path string) (Tokens, error) {
	tokens, err := ReadTokensFile(fileSystem, path)
	if err != nil {
		return Tokens{}, err
	}
	if err := tokens.validate(true); err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

// ReadTokensFile reads structured Tokens through fileSystem. A missing file
// yields empty Tokens.
func ReadTokensFile(fileSystem filesystem.FS, path string) (Tokens, error) {
	file, err := fileSystem.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Tokens{}, nil
		}
		return Tokens{}, fmt.Errorf("read tokens file: %w", err)
	}
	defer file.Close()

	var tokens Tokens
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tokens); err != nil {
		return Tokens{}, fmt.Errorf("parse tokens file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Tokens{}, fmt.Errorf("parse tokens file: trailing data")
	}
	if err := tokens.validate(false); err != nil {
		return Tokens{}, err
	}
	return tokens, nil
}

// WriteTokensFile saves Tokens atomically through fileSystem.
func WriteTokensFile(fileSystem filesystem.FS, path string, tokens Tokens) error {
	if err := tokens.validate(true); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tokens file: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	temporary, err := fileSystem.CreateTemp(directory, ".tokens-*.json")
	if err != nil {
		return fmt.Errorf("create temporary tokens file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = fileSystem.Remove(temporaryName)
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
	if err := fileSystem.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace tokens file: %w", err)
	}
	return nil
}

// IssueToken performs the complete Token transaction through fileSystem.
func IssueToken(fileSystem filesystem.FS, path, identity string, replace bool) (string, error) {
	if identity != "" && !ValidName(identity) {
		return "", fmt.Errorf("invalid identity %q", identity)
	}
	if err := fileSystem.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create tokens directory: %w", err)
	}

	tokenUpdateMu.Lock()
	defer tokenUpdateMu.Unlock()
	unlock, err := fileSystem.Lock(path + ".lock")
	if err != nil {
		return "", err
	}
	defer unlock()

	tokens, err := ReadTokensFile(fileSystem, path)
	if err != nil {
		return "", err
	}
	exists := tokens.Token != ""
	if identity != "" {
		exists = tokens.Identities[identity] != ""
	}
	if exists && !replace {
		if identity == "" {
			return "", fmt.Errorf("%w for Default Identity; use --replace to rotate", ErrTokenExists)
		}
		return "", fmt.Errorf("%w for identity %q; use --replace to rotate", ErrTokenExists, identity)
	}

	secret, err := GenerateSecret()
	if err != nil {
		return "", err
	}
	if identity == "" {
		tokens.Token = secret
	} else {
		if tokens.Identities == nil {
			tokens.Identities = make(map[string]string)
		}
		tokens.Identities[identity] = secret
	}
	if err := WriteTokensFile(fileSystem, path, tokens); err != nil {
		return "", err
	}
	if identity == "" {
		return secret, nil
	}
	return identity + "." + secret, nil
}

// Authenticate validates an Authorization header and returns its verified
// Default or Named Identity.
func (tokens Tokens) Authenticate(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}

	identity, secret, err := parseToken(strings.TrimPrefix(header, prefix))
	if err != nil {
		return "", false
	}
	expected := tokens.Token
	if identity != "" {
		expected = tokens.Identities[identity]
	}
	if expected == "" {
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
