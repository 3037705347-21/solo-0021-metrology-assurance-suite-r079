package domain

import "errors"

var (
	ErrInvalidInput          = errors.New("invalid input")
	ErrNotFound              = errors.New("resource not found")
	ErrDuplicateAssetTag     = errors.New("duplicate asset tag")
	ErrDuplicateLabName      = errors.New("duplicate lab name")
	ErrActiveCaseExists      = errors.New("device already has an unfinished case")
	ErrStateConflict         = errors.New("resource state conflict")
	ErrSealAlreadyCreated    = errors.New("case already has an evidence seal")
	ErrCapabilityUnavailable = errors.New("external capability unavailable")
	ErrScopeMismatch         = errors.New("measurement outside accredited scope")
)

func IsKnownError(err error) bool {
	return errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrDuplicateAssetTag) ||
		errors.Is(err, ErrDuplicateLabName) ||
		errors.Is(err, ErrActiveCaseExists) ||
		errors.Is(err, ErrStateConflict) ||
		errors.Is(err, ErrSealAlreadyCreated) ||
		errors.Is(err, ErrCapabilityUnavailable) ||
		errors.Is(err, ErrScopeMismatch)
}
