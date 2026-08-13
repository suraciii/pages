package pages

import (
	"errors"
	"unicode/utf8"
)

var errInvalidUTF8 = errors.New("body is not valid UTF-8")

// utf8Validator validates a stream without retaining the published body.
type utf8Validator struct {
	pending []byte
}

func (validator *utf8Validator) Write(chunk []byte) (int, error) {
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
			return len(chunk), errInvalidUTF8
		}
		data = data[size:]
	}
	return len(chunk), nil
}

func (validator *utf8Validator) Valid() bool {
	return len(validator.pending) == 0
}
