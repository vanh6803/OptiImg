package core

import "errors"

var (
	ErrUnsupportedFormat = errors.New("core: unsupported image format")
	ErrTooLarge          = errors.New("core: image exceeds size limits")
	ErrAnimated          = errors.New("core: animated PNG is not supported")
	ErrCorrupt           = errors.New("core: corrupt image data")
)
