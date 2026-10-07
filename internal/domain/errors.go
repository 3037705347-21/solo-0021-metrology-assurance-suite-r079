package domain

import "errors"

var (
	ErrInvalidInput             = errors.New("invalid input")
	ErrNotFound                 = errors.New("resource not found")
	ErrDuplicateAssetTag        = errors.New("duplicate asset tag")
	ErrDuplicateProvider        = errors.New("duplicate external calibration provider")
	ErrActiveCaseExists         = errors.New("device already has an unfinished case")
	ErrStateConflict            = errors.New("resource state conflict")
	ErrSealAlreadyCreated       = errors.New("case already has an evidence seal")
	ErrExternalBasisUnavailable = errors.New("external calibration basis is unavailable")
)

func IsKnownError(err error) bool {
	return errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrDuplicateAssetTag) ||
		errors.Is(err, ErrDuplicateProvider) ||
		errors.Is(err, ErrActiveCaseExists) ||
		errors.Is(err, ErrStateConflict) ||
		errors.Is(err, ErrSealAlreadyCreated) ||
		errors.Is(err, ErrExternalBasisUnavailable)
}
