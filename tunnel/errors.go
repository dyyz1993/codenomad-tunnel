package tunnel

import "errors"

var (
	ErrNotFound  = errors.New("tunnel not found")
	ErrDuplicate = errors.New("subdomain already exists")
	ErrNoRelay   = errors.New("no relay client connected")
)
