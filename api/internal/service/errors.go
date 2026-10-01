package service

import "errors"

// Sentinel errors let handlers map service failures to HTTP status codes.
// Wrap them with fmt.Errorf("%w: ...") to add detail.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid request")
	ErrConflict = errors.New("conflict")
)
