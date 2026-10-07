package domain

import (
	"fmt"
	"strings"
	"time"
)

type LabStatus string

const (
	LabActive    LabStatus = "active"
	LabSuspended LabStatus = "suspended"
)

// CalibrationLab is an external calibration organization whose accredited
// capability scopes may back case measurements.
type CalibrationLab struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Accreditation string    `json:"accreditation"`
	Steward       string    `json:"steward"`
	Status        LabStatus `json:"status"`
	RegisteredAt  time.Time `json:"registered_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewCalibrationLab(id, name, accreditation, steward string, at time.Time) (CalibrationLab, error) {
	item := CalibrationLab{
		ID:            strings.TrimSpace(id),
		Name:          strings.TrimSpace(name),
		Accreditation: strings.TrimSpace(accreditation),
		Steward:       strings.TrimSpace(steward),
		Status:        LabActive,
		RegisteredAt:  at.UTC(),
		UpdatedAt:     at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return CalibrationLab{}, err
	}
	return item, nil
}

// NormalizeLabName folds case so lab names stay unique without rewriting the
// steward-provided spelling stored on the lab.
func NormalizeLabName(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (l CalibrationLab) Validate() error {
	if l.ID == "" || l.Name == "" || l.Accreditation == "" || l.Steward == "" {
		return fmt.Errorf("%w: lab id, name, accreditation, and steward are required", ErrInvalidInput)
	}
	if l.RegisteredAt.IsZero() || l.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: lab timestamps are required", ErrInvalidInput)
	}
	switch l.Status {
	case LabActive, LabSuspended:
		return nil
	default:
		return fmt.Errorf("%w: unsupported lab state %q", ErrInvalidInput, l.Status)
	}
}

func (l CalibrationLab) IsActive() bool {
	return l.Status == LabActive
}

func (l *CalibrationLab) Suspend(at time.Time) error {
	if l.Status != LabActive {
		return fmt.Errorf("%w: only an active lab can be suspended", ErrStateConflict)
	}
	l.Status = LabSuspended
	l.UpdatedAt = at.UTC()
	return nil
}

func (l *CalibrationLab) Reinstate(at time.Time) error {
	if l.Status != LabSuspended {
		return fmt.Errorf("%w: only a suspended lab can be reinstated", ErrStateConflict)
	}
	l.Status = LabActive
	l.UpdatedAt = at.UTC()
	return nil
}

// CapabilityScope is one accredited measurement range of a lab, bounded by a
// validity window. Measurements may reference a scope only while it is valid.
type CapabilityScope struct {
	ID          string    `json:"id"`
	LabID       string    `json:"lab_id"`
	Parameter   string    `json:"parameter"`
	MinValue    float64   `json:"min_value"`
	MaxValue    float64   `json:"max_value"`
	Uncertainty float64   `json:"uncertainty"`
	ValidFrom   time.Time `json:"valid_from"`
	ValidUntil  time.Time `json:"valid_until"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewCapabilityScope(id, labID, parameter string, minValue, maxValue, uncertainty float64, validFrom, validUntil, at time.Time) (CapabilityScope, error) {
	item := CapabilityScope{
		ID:          strings.TrimSpace(id),
		LabID:       strings.TrimSpace(labID),
		Parameter:   strings.TrimSpace(parameter),
		MinValue:    minValue,
		MaxValue:    maxValue,
		Uncertainty: uncertainty,
		ValidFrom:   validFrom.UTC(),
		ValidUntil:  validUntil.UTC(),
		UpdatedAt:   at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return CapabilityScope{}, err
	}
	return item, nil
}

func (s CapabilityScope) Validate() error {
	if s.ID == "" || s.LabID == "" || s.Parameter == "" {
		return fmt.Errorf("%w: scope id, lab id, and parameter are required", ErrInvalidInput)
	}
	if !isFinite(s.MinValue) || !isFinite(s.MaxValue) || s.MinValue >= s.MaxValue {
		return fmt.Errorf("%w: scope range must be finite and min_value must be below max_value", ErrInvalidInput)
	}
	if !isFinite(s.Uncertainty) || s.Uncertainty <= 0 {
		return fmt.Errorf("%w: scope uncertainty must be positive and finite", ErrInvalidInput)
	}
	if s.ValidFrom.IsZero() || s.ValidUntil.IsZero() || !s.ValidFrom.Before(s.ValidUntil) {
		return fmt.Errorf("%w: scope validity window must be set and valid_from must precede valid_until", ErrInvalidInput)
	}
	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: scope update time is required", ErrInvalidInput)
	}
	return nil
}

// ValidAt reports whether the scope's validity window covers the given time.
func (s CapabilityScope) ValidAt(at time.Time) bool {
	stamp := at.UTC()
	return !stamp.Before(s.ValidFrom) && !stamp.After(s.ValidUntil)
}

// Covers reports whether an observed value falls inside the accredited range.
func (s CapabilityScope) Covers(value float64) bool {
	return value >= s.MinValue && value <= s.MaxValue
}

// ExternalBasis is the immutable snapshot of the lab capability a measurement
// referenced when it was recorded. Later lab or scope maintenance never
// rewrites snapshots already attached to measurements or evidence seals.
type ExternalBasis struct {
	LabID         string    `json:"lab_id"`
	LabName       string    `json:"lab_name"`
	Accreditation string    `json:"accreditation"`
	ScopeID       string    `json:"scope_id"`
	Parameter     string    `json:"parameter"`
	MinValue      float64   `json:"min_value"`
	MaxValue      float64   `json:"max_value"`
	Uncertainty   float64   `json:"uncertainty"`
	ValidFrom     time.Time `json:"valid_from"`
	ValidUntil    time.Time `json:"valid_until"`
	ReferencedAt  time.Time `json:"referenced_at"`
}

func NewExternalBasis(lab CalibrationLab, scope CapabilityScope, at time.Time) (ExternalBasis, error) {
	item := ExternalBasis{
		LabID:         lab.ID,
		LabName:       lab.Name,
		Accreditation: lab.Accreditation,
		ScopeID:       scope.ID,
		Parameter:     scope.Parameter,
		MinValue:      scope.MinValue,
		MaxValue:      scope.MaxValue,
		Uncertainty:   scope.Uncertainty,
		ValidFrom:     scope.ValidFrom,
		ValidUntil:    scope.ValidUntil,
		ReferencedAt:  at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return ExternalBasis{}, err
	}
	return item, nil
}

func (b ExternalBasis) Validate() error {
	if b.LabID == "" || b.LabName == "" || b.Accreditation == "" || b.ScopeID == "" || b.Parameter == "" {
		return fmt.Errorf("%w: external basis lab, accreditation, scope, and parameter are required", ErrInvalidInput)
	}
	if !isFinite(b.MinValue) || !isFinite(b.MaxValue) || b.MinValue >= b.MaxValue {
		return fmt.Errorf("%w: external basis range must be finite and min_value must be below max_value", ErrInvalidInput)
	}
	if !isFinite(b.Uncertainty) || b.Uncertainty <= 0 {
		return fmt.Errorf("%w: external basis uncertainty must be positive and finite", ErrInvalidInput)
	}
	if b.ValidFrom.IsZero() || b.ValidUntil.IsZero() || b.ReferencedAt.IsZero() {
		return fmt.Errorf("%w: external basis validity and reference times are required", ErrInvalidInput)
	}
	return nil
}
