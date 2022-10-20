package types

import "errors"

var (
	ErrEmptyCache        = errors.New("empty cache")
	ErrInvalidMaxEntries = errors.New("invalid max entries")
)
