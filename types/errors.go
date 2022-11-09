package types

import "errors"

var (
	ErrEmptyCache        = errors.New("empty cache")
	ErrInvalidMaxEntries = errors.New("invalid max entries")
	ErrTooManyEntries    = errors.New("the number of blocks is more than maxEntries")
	ErrorUnsortedBlocks  = errors.New("blocks are not sorted by height")
)
