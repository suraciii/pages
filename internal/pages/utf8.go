package pages

import (
	"errors"
	"unicode/utf8"
)

// ErrInvalidUTF8 marks a Page body that is not valid UTF-8. Both the
// server Upload path and local Publish report it as a rejected Page.
var ErrInvalidUTF8 = errors.New("body is not valid UTF-8")

// UTF8Validator validates a stream without retaining the published body.
// Callers write the Page bytes through it while they copy the Page into
// place, then read Valid once the copy ends.
type UTF8Validator struct {
	pending []byte
}

// NewUTF8Validator returns a validator with no buffered bytes.
func NewUTF8Validator() *UTF8Validator {
	return &UTF8Validator{}
}

func (validator *UTF8Validator) Write(chunk []byte) (int, error) {
	data := make([]byte, len(validator.pending)+len(chunk))
	copy(data, validator.pending)
	copy(data[len(validator.pending):], chunk)
	validator.pending = validator.pending[:0]

	for len(data) > 0 {
		if !utf8.FullRune(data) {
			validator.pending = append(validator.pending, data...)
			return len(chunk), nil
		}
		_, size := utf8.DecodeRune(data)
		if size == 1 && data[0] >= utf8.RuneSelf {
			return len(chunk), ErrInvalidUTF8
		}
		data = data[size:]
	}
	return len(chunk), nil
}

// Valid reports whether every byte written so far formed complete UTF-8.
func (validator *UTF8Validator) Valid() bool {
	return len(validator.pending) == 0
}
