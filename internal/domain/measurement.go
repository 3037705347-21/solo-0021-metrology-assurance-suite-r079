package domain

import (
	"fmt"
	"strings"
	"time"
)

type Measurement struct {
	ID            string         `json:"id"`
	CaseID        string         `json:"case_id"`
	Observed      float64        `json:"observed"`
	Nominal       float64        `json:"nominal"`
	Deviation     float64        `json:"deviation"`
	Tolerance     float64        `json:"tolerance"`
	Accepted      bool           `json:"accepted"`
	Technician    string         `json:"technician"`
	ExternalBasis *ExternalBasis `json:"external_basis,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

func NewMeasurement(id, caseID string, observed, nominal, tolerance float64, technician string, at time.Time) (Measurement, error) {
	item := Measurement{
		ID:         strings.TrimSpace(id),
		CaseID:     strings.TrimSpace(caseID),
		Observed:   observed,
		Nominal:    nominal,
		Deviation:  observed - nominal,
		Tolerance:  tolerance,
		Accepted:   abs(observed-nominal) <= tolerance,
		Technician: strings.TrimSpace(technician),
		CreatedAt:  at.UTC(),
	}
	if err := item.Validate(); err != nil {
		return Measurement{}, err
	}
	return item, nil
}

func (m Measurement) Validate() error {
	if m.ID == "" || m.CaseID == "" || m.Technician == "" {
		return fmt.Errorf("%w: measurement id, case id, and technician are required", ErrInvalidInput)
	}
	if !isFinite(m.Observed) || !isFinite(m.Nominal) || !isFinite(m.Tolerance) || m.Tolerance <= 0 {
		return fmt.Errorf("%w: observed and nominal must be finite and tolerance must be positive", ErrInvalidInput)
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("%w: measurement time is required", ErrInvalidInput)
	}
	if m.ExternalBasis != nil {
		if err := m.ExternalBasis.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// AttachExternalBasis pins an immutable snapshot of the external calibration
// capability that backs this measurement.
func (m *Measurement) AttachExternalBasis(basis ExternalBasis) error {
	if err := basis.Validate(); err != nil {
		return err
	}
	m.ExternalBasis = &basis
	return nil
}

func (m Measurement) AbsoluteDeviation() float64 {
	return abs(m.Deviation)
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
