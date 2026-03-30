package profiles

import "errors"

var (
	ErrProfileNotFound   = errors.New("profiles: not found")
	ErrPhotoUploadFailed = errors.New("profiles: photo upload failed")
	ErrUnauthorized      = errors.New("profiles: unauthorized")
)
