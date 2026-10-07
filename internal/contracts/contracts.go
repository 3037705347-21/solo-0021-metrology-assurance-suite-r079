package contracts

import (
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
	Steward   string  `json:"steward"`
}

type RecordMeasurementRequest struct {
	Observed      float64                   `json:"observed"`
	Technician    string                    `json:"technician"`
	ExternalBasis *ExternalReferenceRequest `json:"external_basis,omitempty"`
}

// ExternalReferenceRequest points a measurement at the lab capability scope
// that backs it. The service resolves it to an immutable snapshot at record
// time.
type ExternalReferenceRequest struct {
	LabID   string `json:"lab_id"`
	ScopeID string `json:"scope_id"`
}

type RegisterLabRequest struct {
	Name          string `json:"name"`
	Accreditation string `json:"accreditation"`
	Steward       string `json:"steward"`
}

type UpsertScopeRequest struct {
	Parameter   string    `json:"parameter"`
	MinValue    float64   `json:"min_value"`
	MaxValue    float64   `json:"max_value"`
	Uncertainty float64   `json:"uncertainty"`
	ValidFrom   time.Time `json:"valid_from"`
	ValidUntil  time.Time `json:"valid_until"`
	Actor       string    `json:"actor"`
}

type SuspendLabRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type ReinstateLabRequest struct {
	Actor string `json:"actor"`
}

type ReopenCaseRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type SealCaseRequest struct {
	Reviewer string `json:"reviewer"`
	Summary  string `json:"summary"`
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

type LabResponse struct {
	Lab    domain.CalibrationLab    `json:"lab"`
	Scopes []domain.CapabilityScope `json:"scopes"`
	Events []domain.AuditEvent      `json:"events"`
}

type EventsResponse struct {
	Events []domain.AuditEvent `json:"events"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
