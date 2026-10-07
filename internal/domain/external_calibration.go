package domain

import (
	"fmt"
	"strings"
	"time"
)

// ExternalCalibrationProvider is the organization that performed an external
// calibration. Its current record is maintainable, while references made by
// measurements retain their own snapshot.
type ExternalCalibrationProvider struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	AccreditationNumber string    `json:"accreditation_number"`
	AccreditationBody   string    `json:"accreditation_body,omitempty"`
	Maintainer          string    `json:"maintainer"`
	Version             int       `json:"version"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ExternalCalibrationCapability is one accredited scope and validity period
// offered by an external calibration provider.
type ExternalCalibrationCapability struct {
	ID                string    `json:"id"`
	ProviderID        string    `json:"provider_id"`
	CertificateNumber string    `json:"certificate_number"`
	Scope             string    `json:"scope"`
	Unit              string    `json:"unit"`
	MinValue          float64   `json:"min_value"`
	MaxValue          float64   `json:"max_value"`
	ValidFrom         time.Time `json:"valid_from"`
	ValidUntil        time.Time `json:"valid_until"`
	Version           int       `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ExternalCalibrationBasis is the immutable source captured when a measurement
// references an external calibration capability.
type ExternalCalibrationBasis struct {
	CapabilityID        string    `json:"capability_id"`
	ProviderID          string    `json:"provider_id"`
	ProviderName        string    `json:"provider_name"`
	AccreditationNumber string    `json:"accreditation_number"`
	CertificateNumber   string    `json:"certificate_number"`
	Scope               string    `json:"scope"`
	Unit                string    `json:"unit"`
	MinValue            float64   `json:"min_value"`
	MaxValue            float64   `json:"max_value"`
	ValidFrom           time.Time `json:"valid_from"`
	ValidUntil          time.Time `json:"valid_until"`
	ReferencedAt        time.Time `json:"referenced_at"`
}

func NewExternalCalibrationProvider(id, name, accreditationNumber, accreditationBody, maintainer string, at time.Time) (ExternalCalibrationProvider, error) {
	item := ExternalCalibrationProvider{
		ID:                  strings.TrimSpace(id),
		Name:                normalizeProviderName(name),
		AccreditationNumber: strings.TrimSpace(accreditationNumber),
		AccreditationBody:   strings.TrimSpace(accreditationBody),
		Maintainer:          strings.TrimSpace(maintainer),
		Version:             1,
		CreatedAt:           at.UTC(),
		UpdatedAt:           at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return ExternalCalibrationProvider{}, err
	}
	return item, nil
}

func (p ExternalCalibrationProvider) Validate() error {
	if p.ID == "" || p.Name == "" || p.AccreditationNumber == "" || p.Maintainer == "" {
		return fmt.Errorf("%w: provider id, name, accreditation number, and maintainer are required", ErrInvalidInput)
	}
	if len(p.Name) > 128 || len(p.AccreditationNumber) > 64 || len(p.AccreditationBody) > 128 || len(p.Maintainer) > 128 {
		return fmt.Errorf("%w: external calibration provider field is too long", ErrInvalidInput)
	}
	if p.Version < 1 || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: provider version and timestamps are required", ErrInvalidInput)
	}
	return nil
}

func (p *ExternalCalibrationProvider) Update(name, accreditationNumber, accreditationBody, maintainer string, at time.Time) error {
	p.Name = normalizeProviderName(name)
	p.AccreditationNumber = strings.TrimSpace(accreditationNumber)
	p.AccreditationBody = strings.TrimSpace(accreditationBody)
	if trimmed := strings.TrimSpace(maintainer); trimmed != "" {
		p.Maintainer = trimmed
	}
	p.Version++
	p.UpdatedAt = at.UTC()
	return p.Validate()
}

func NormalizeProviderName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func normalizeProviderName(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func NewExternalCalibrationCapability(id, providerID, certificateNumber, scope, unit string, minValue, maxValue float64, validFrom, validUntil time.Time, at time.Time) (ExternalCalibrationCapability, error) {
	item := ExternalCalibrationCapability{
		ID:                strings.TrimSpace(id),
		ProviderID:        strings.TrimSpace(providerID),
		CertificateNumber: strings.TrimSpace(certificateNumber),
		Scope:             NormalizeScope(scope),
		Unit:              normalizeUnit(unit),
		MinValue:          minValue,
		MaxValue:          maxValue,
		ValidFrom:         validFrom.UTC(),
		ValidUntil:        validUntil.UTC(),
		Version:           1,
		CreatedAt:         at.UTC(),
		UpdatedAt:         at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return ExternalCalibrationCapability{}, err
	}
	return item, nil
}

func (c ExternalCalibrationCapability) Validate() error {
	if c.ID == "" || c.ProviderID == "" || c.CertificateNumber == "" || c.Scope == "" || c.Unit == "" {
		return fmt.Errorf("%w: capability id, provider, certificate number, scope, and unit are required", ErrInvalidInput)
	}
	if len(c.CertificateNumber) > 128 || len(c.Scope) > 64 || len(c.Unit) > 32 {
		return fmt.Errorf("%w: external calibration capability field is too long", ErrInvalidInput)
	}
	if !isFinite(c.MinValue) || !isFinite(c.MaxValue) || c.MinValue > c.MaxValue {
		return fmt.Errorf("%w: capability range must have finite minimum and maximum values", ErrInvalidInput)
	}
	if c.ValidFrom.IsZero() || c.ValidUntil.IsZero() || c.ValidFrom.After(c.ValidUntil) {
		return fmt.Errorf("%w: capability validity period must include a start and an end", ErrInvalidInput)
	}
	if c.Version < 1 || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: capability version and timestamps are required", ErrInvalidInput)
	}
	return nil
}

func (c *ExternalCalibrationCapability) Update(certificateNumber, scope, unit string, minValue, maxValue float64, validFrom, validUntil time.Time, at time.Time) error {
	c.CertificateNumber = strings.TrimSpace(certificateNumber)
	c.Scope = NormalizeScope(scope)
	c.Unit = normalizeUnit(unit)
	c.MinValue = minValue
	c.MaxValue = maxValue
	c.ValidFrom = validFrom.UTC()
	c.ValidUntil = validUntil.UTC()
	c.Version++
	c.UpdatedAt = at.UTC()
	return c.Validate()
}

// Covers verifies that the capability was valid at the reference time, that its
// scope and unit match the case, and that the measured value is inside range.
func (c ExternalCalibrationCapability) Covers(scope, unit string, nominal, observed float64, at time.Time) error {
	stamp := at.UTC()
	if stamp.Before(c.ValidFrom) || stamp.After(c.ValidUntil) {
		return fmt.Errorf("%w: capability %s is not valid between %s and %s at %s", ErrExternalBasisUnavailable, c.ID, c.ValidFrom, c.ValidUntil, stamp)
	}
	if NormalizeScope(scope) != c.Scope || normalizeUnit(unit) != c.Unit {
		return fmt.Errorf("%w: capability %s scope %s/%s does not cover case scope %s/%s", ErrExternalBasisUnavailable, c.ID, c.Scope, c.Unit, NormalizeScope(scope), normalizeUnit(unit))
	}
	if !c.coversValue(nominal) || !c.coversValue(observed) {
		return fmt.Errorf("%w: nominal %v and observed %v must both be inside capability %s range [%v, %v]", ErrExternalBasisUnavailable, nominal, observed, c.ID, c.MinValue, c.MaxValue)
	}
	return nil
}

func (c ExternalCalibrationCapability) coversValue(value float64) bool {
	return isFinite(value) && value >= c.MinValue && value <= c.MaxValue
}

func NewExternalCalibrationBasis(capability ExternalCalibrationCapability, provider ExternalCalibrationProvider, at time.Time) (ExternalCalibrationBasis, error) {
	item := ExternalCalibrationBasis{
		CapabilityID:        capability.ID,
		ProviderID:          provider.ID,
		ProviderName:        provider.Name,
		AccreditationNumber: provider.AccreditationNumber,
		CertificateNumber:   capability.CertificateNumber,
		Scope:               capability.Scope,
		Unit:                capability.Unit,
		MinValue:            capability.MinValue,
		MaxValue:            capability.MaxValue,
		ValidFrom:           capability.ValidFrom,
		ValidUntil:          capability.ValidUntil,
		ReferencedAt:        at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return ExternalCalibrationBasis{}, err
	}
	return item, nil
}

func (b ExternalCalibrationBasis) Validate() error {
	if b.CapabilityID == "" || b.ProviderID == "" || b.ProviderName == "" || b.AccreditationNumber == "" || b.CertificateNumber == "" || b.Scope == "" {
		return fmt.Errorf("%w: external calibration basis source fields are required", ErrInvalidInput)
	}
	if !isFinite(b.MinValue) || !isFinite(b.MaxValue) || b.MinValue > b.MaxValue {
		return fmt.Errorf("%w: external calibration basis range is invalid", ErrInvalidInput)
	}
	if b.ValidFrom.IsZero() || b.ValidUntil.IsZero() || b.ReferencedAt.IsZero() {
		return fmt.Errorf("%w: external calibration basis validity and reference times are required", ErrInvalidInput)
	}
	return nil
}

func NormalizeScope(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeUnit(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
