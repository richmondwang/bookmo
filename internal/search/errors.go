package search

import "errors"

var (
	ErrNotFound           = errors.New("search: not found")
	ErrInvalidCoordinates = errors.New("search: invalid coordinates")
)
