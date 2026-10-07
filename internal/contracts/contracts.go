package contracts

import (
	"encoding/json"
	"time"

	"example.com/solo-0021-metrology-assurance-suite/internal/domain"
)

type RegisterDeviceRequest struct {
	AssetTag string `json:"asset_tag"`
	Model    string `json:"model"`
	Room     string `json:"room"`
	Steward  string `json:"steward"`
}

type OpenCaseRequest struct {
	DeviceID  string  `json:"device_id"`
	Nominal   float64 `json:"nominal"`
	Tolerance float64 `json:"tolerance"`
	Scope     string  `json:"scope"`
	Unit      string  `json:"unit"`
	Steward   string  `json:"steward"`
}

type RecordMeasurementRequest struct {
	Observed             float64 `json:"observed"`
	Technician           string  `json:"technician"`
	ExternalCapabilityID string  `json:"external_capability_id"`
}

type ReopenCaseRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type SealCaseRequest struct {
	Reviewer string `json:"reviewer"`
	Summary  string `json:"summary"`
}

type RegisterExternalProviderRequest struct {
	Name                string `json:"name"`
	AccreditationNumber string `json:"accreditation_number"`
	AccreditationBody   string `json:"accreditation_body"`
	Maintainer          string `json:"maintainer"`
}

type UpdateExternalProviderRequest struct {
	Name                string `json:"name"`
	AccreditationNumber string `json:"accreditation_number"`
	AccreditationBody   string `json:"accreditation_body"`
	Maintainer          string `json:"maintainer"`
}

type RegisterExternalCapabilityPayload struct {
	CertificateNumber string    `json:"certificate_number"`
	Scope             string    `json:"scope"`
	Unit              string    `json:"unit"`
	MinValue          float64   `json:"min_value"`
	MaxValue          float64   `json:"max_value"`
	ValidFrom         time.Time `json:"valid_from"`
	ValidUntil        time.Time `json:"valid_until"`
	Maintainer        string    `json:"maintainer"`
}

type UpdateExternalCapabilityPayload struct {
	CertificateNumber string    `json:"certificate_number"`
	Scope             string    `json:"scope"`
	Unit              string    `json:"unit"`
	MinValue          float64   `json:"min_value"`
	MaxValue          float64   `json:"max_value"`
	ValidFrom         time.Time `json:"valid_from"`
	ValidUntil        time.Time `json:"valid_until"`
	Maintainer        string    `json:"maintainer"`
}

type dateTime time.Time
type endDateTime time.Time

func unmarshalFlexibleTime(data []byte, endOfDay bool) (time.Time, error) {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return time.Time{}, err
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return parsed, nil
}

func (d *dateTime) UnmarshalJSON(data []byte) error {
	parsed, err := unmarshalFlexibleTime(data, false)
	if err != nil {
		return err
	}
	*d = dateTime(parsed)
	return nil
}

func (d *endDateTime) UnmarshalJSON(data []byte) error {
	parsed, err := unmarshalFlexibleTime(data, true)
	if err != nil {
		return err
	}
	*d = endDateTime(parsed)
	return nil
}

type externalCapabilityRequest struct {
	CertificateNumber string      `json:"certificate_number"`
	Scope             string      `json:"scope"`
	Unit              string      `json:"unit"`
	MinValue          float64     `json:"min_value"`
	MaxValue          float64     `json:"max_value"`
	ValidFrom         dateTime    `json:"valid_from"`
	ValidUntil        endDateTime `json:"valid_until"`
	Maintainer        string      `json:"maintainer"`
}

func (r *RegisterExternalCapabilityPayload) UnmarshalJSON(data []byte) error {
	var alias externalCapabilityRequest
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = RegisterExternalCapabilityPayload{
		CertificateNumber: alias.CertificateNumber,
		Scope:             alias.Scope,
		Unit:              alias.Unit,
		MinValue:          alias.MinValue,
		MaxValue:          alias.MaxValue,
		ValidFrom:         time.Time(alias.ValidFrom),
		ValidUntil:        time.Time(alias.ValidUntil),
		Maintainer:        alias.Maintainer,
	}
	return nil
}

func (r *UpdateExternalCapabilityPayload) UnmarshalJSON(data []byte) error {
	var alias externalCapabilityRequest
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = UpdateExternalCapabilityPayload{
		CertificateNumber: alias.CertificateNumber,
		Scope:             alias.Scope,
		Unit:              alias.Unit,
		MinValue:          alias.MinValue,
		MaxValue:          alias.MaxValue,
		ValidFrom:         time.Time(alias.ValidFrom),
		ValidUntil:        time.Time(alias.ValidUntil),
		Maintainer:        alias.Maintainer,
	}
	return nil
}

type DeviceResponse struct {
	Device domain.Device `json:"device"`
}

type CaseResponse struct {
	Case         domain.VerificationCase `json:"case"`
	Measurements []domain.Measurement    `json:"measurements"`
	Seal         *domain.EvidenceSeal    `json:"seal,omitempty"`
	Events       []domain.AuditEvent     `json:"events"`
}

type EventsResponse struct {
	Events []domain.AuditEvent `json:"events"`
}

type ExternalProviderResponse struct {
	Provider domain.ExternalCalibrationProvider `json:"provider"`
}

type ExternalProviderDetailResponse struct {
	Provider     domain.ExternalCalibrationProvider     `json:"provider"`
	Capabilities []domain.ExternalCalibrationCapability `json:"capabilities"`
	Events       []domain.AuditEvent                    `json:"events"`
}

type ExternalCapabilityResponse struct {
	Capability domain.ExternalCalibrationCapability `json:"capability"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
