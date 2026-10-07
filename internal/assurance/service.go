package assurance

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/solo-0021-metrology-assurance-suite/internal/contracts"
	"example.com/solo-0021-metrology-assurance-suite/internal/domain"
	"example.com/solo-0021-metrology-assurance-suite/internal/repository"
)

type Service struct {
	repository *repository.Repository
	now        func() time.Time
}

func NewService(repo *repository.Repository) *Service {
	return &Service{
		repository: repo,
		now:        time.Now,
	}
}

func (s *Service) RegisterDevice(ctx context.Context, input contracts.RegisterDeviceRequest) (domain.Device, error) {
	var created domain.Device
	tag := domain.NormalizeAssetTag(input.AssetTag)
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		if _, exists := m.DeviceByTag(tag); exists {
			return fmt.Errorf("%w: %s", domain.ErrDuplicateAssetTag, tag)
		}
		item, err := domain.NewDevice(
			m.NextID("device"),
			input.AssetTag,
			input.Model,
			input.Room,
			input.Steward,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutDevice(item); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"device",
			item.ID,
			"",
			"device_enrolled",
			item.Steward,
			"device enrolled for assurance work",
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		created = item
		return nil
	})
	return created, err
}

func (s *Service) Device(ctx context.Context, id string) (domain.Device, error) {
	var found domain.Device
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		item, exists := snapshot.Device(strings.TrimSpace(id))
		if !exists {
			return fmt.Errorf("%w: device %s", domain.ErrNotFound, id)
		}
		found = item
		return nil
	})
	return found, err
}

func (s *Service) RegisterExternalProvider(ctx context.Context, input contracts.RegisterExternalProviderRequest) (domain.ExternalCalibrationProvider, error) {
	var created domain.ExternalCalibrationProvider
	name := domain.NormalizeProviderName(input.Name)
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		if _, exists := m.ExternalProviderByName(name); exists {
			return fmt.Errorf("%w: %s", domain.ErrDuplicateProvider, name)
		}
		item, err := domain.NewExternalCalibrationProvider(
			m.NextID("external_provider"),
			input.Name,
			input.AccreditationNumber,
			input.AccreditationBody,
			input.Maintainer,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutExternalProvider(item); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"external_calibration_provider",
			item.ID,
			"",
			"external_provider_registered",
			item.Maintainer,
			fmt.Sprintf("external calibration provider %s (%s) registered", item.Name, item.AccreditationNumber),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		created = item
		return nil
	})
	return created, err
}

func (s *Service) ExternalProviders(ctx context.Context) ([]domain.ExternalCalibrationProvider, error) {
	var result []domain.ExternalCalibrationProvider
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		result = snapshot.ExternalProviders()
		sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
		return nil
	})
	return result, err
}

func (s *Service) ExternalProvider(ctx context.Context, id string) (contracts.ExternalProviderDetailResponse, error) {
	var result contracts.ExternalProviderDetailResponse
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		item, exists := snapshot.ExternalProvider(strings.TrimSpace(id))
		if !exists {
			return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, id)
		}
		result = providerDetail(snapshot, item)
		return nil
	})
	return result, err
}

func (s *Service) UpdateExternalProvider(ctx context.Context, id string, input contracts.UpdateExternalProviderRequest) (contracts.ExternalProviderDetailResponse, error) {
	var result contracts.ExternalProviderDetailResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		trimmedID := strings.TrimSpace(id)
		item, exists := m.ExternalProvider(trimmedID)
		if !exists {
			return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, id)
		}
		name := domain.NormalizeProviderName(input.Name)
		if other, exists := m.ExternalProviderByName(name); exists && other.ID != item.ID {
			return fmt.Errorf("%w: %s", domain.ErrDuplicateProvider, name)
		}
		if err := item.Update(input.Name, input.AccreditationNumber, input.AccreditationBody, input.Maintainer, s.now()); err != nil {
			return err
		}
		if err := m.UpdateExternalProvider(item); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"external_calibration_provider",
			item.ID,
			"",
			"external_provider_updated",
			item.Maintainer,
			fmt.Sprintf("external calibration provider %s record updated to version %d", item.Name, item.Version),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = providerDetail(m, item)
		return nil
	})
	return result, err
}

func (s *Service) RegisterExternalCapability(ctx context.Context, providerID string, input contracts.RegisterExternalCapabilityPayload) (domain.ExternalCalibrationCapability, error) {
	var created domain.ExternalCalibrationCapability
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		provider, exists := m.ExternalProvider(strings.TrimSpace(providerID))
		if !exists {
			return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, providerID)
		}
		item, err := domain.NewExternalCalibrationCapability(
			m.NextID("external_capability"),
			provider.ID,
			input.CertificateNumber,
			input.Scope,
			input.Unit,
			input.MinValue,
			input.MaxValue,
			input.ValidFrom,
			input.ValidUntil,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutExternalCapability(item); err != nil {
			return err
		}
		maintainer := strings.TrimSpace(input.Maintainer)
		if maintainer == "" {
			maintainer = provider.Maintainer
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"external_calibration_provider",
			provider.ID,
			"",
			"external_capability_registered",
			maintainer,
			fmt.Sprintf("external calibration capability %s for scope %s %s valid through %s", item.ID, item.Scope, item.Unit, item.ValidUntil.Format("2006-01-02")),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		created = item
		return nil
	})
	return created, err
}

func (s *Service) ExternalCapability(ctx context.Context, providerID, capabilityID string) (domain.ExternalCalibrationCapability, error) {
	var found domain.ExternalCalibrationCapability
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		item, exists := snapshot.ExternalCapability(strings.TrimSpace(providerID), strings.TrimSpace(capabilityID))
		if !exists {
			return fmt.Errorf("%w: external calibration capability %s for provider %s", domain.ErrNotFound, capabilityID, providerID)
		}
		found = item
		return nil
	})
	return found, err
}

func (s *Service) UpdateExternalCapability(ctx context.Context, providerID, capabilityID string, input contracts.UpdateExternalCapabilityPayload) (domain.ExternalCalibrationCapability, error) {
	var updated domain.ExternalCalibrationCapability
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		trimmedProviderID := strings.TrimSpace(providerID)
		provider, exists := m.ExternalProvider(trimmedProviderID)
		if !exists {
			return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, providerID)
		}
		item, exists := m.ExternalCapability(provider.ID, strings.TrimSpace(capabilityID))
		if !exists {
			return fmt.Errorf("%w: external calibration capability %s for provider %s", domain.ErrNotFound, capabilityID, providerID)
		}
		if err := item.Update(input.CertificateNumber, input.Scope, input.Unit, input.MinValue, input.MaxValue, input.ValidFrom, input.ValidUntil, s.now()); err != nil {
			return err
		}
		if err := m.UpdateExternalCapability(item); err != nil {
			return err
		}
		maintainer := strings.TrimSpace(input.Maintainer)
		if maintainer == "" {
			maintainer = provider.Maintainer
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"external_calibration_provider",
			provider.ID,
			"",
			"external_capability_updated",
			maintainer,
			fmt.Sprintf("external calibration capability %s updated to version %d; existing references remain frozen", item.ID, item.Version),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		updated = item
		return nil
	})
	return updated, err
}

func (s *Service) OpenCase(ctx context.Context, input contracts.OpenCaseRequest) (contracts.CaseResponse, error) {
	var result contracts.CaseResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		deviceID := strings.TrimSpace(input.DeviceID)
		device, exists := m.Device(deviceID)
		if !exists {
			return fmt.Errorf("%w: device %s", domain.ErrNotFound, deviceID)
		}
		if !device.IsActive() {
			return fmt.Errorf("%w: device %s is %s", domain.ErrStateConflict, device.ID, device.Status)
		}
		if active, exists := m.ActiveCaseForDevice(device.ID); exists {
			return fmt.Errorf("%w: case %s is still %s", domain.ErrActiveCaseExists, active.ID, active.State)
		}
		item, err := domain.NewVerificationCase(
			m.NextID("case"),
			device.ID,
			input.Nominal,
			input.Tolerance,
			input.Scope,
			input.Unit,
			input.Steward,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutCase(item); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			"case_opened",
			item.Steward,
			"verification case opened",
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = caseResponse(m, item)
		return nil
	})
	return result, err
}

func (s *Service) Case(ctx context.Context, id string) (contracts.CaseResponse, error) {
	var result contracts.CaseResponse
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		item, exists := snapshot.Case(strings.TrimSpace(id))
		if !exists {
			return fmt.Errorf("%w: case %s", domain.ErrNotFound, id)
		}
		result = caseResponseFromSnapshot(snapshot, item)
		return nil
	})
	return result, err
}

func (s *Service) RecordMeasurement(ctx context.Context, caseID string, input contracts.RecordMeasurementRequest) (contracts.CaseResponse, error) {
	var result contracts.CaseResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		id := strings.TrimSpace(caseID)
		item, exists := m.Case(id)
		if !exists {
			return fmt.Errorf("%w: case %s", domain.ErrNotFound, id)
		}
		at := s.now()
		var basis *domain.ExternalCalibrationBasis
		if basisID := strings.TrimSpace(input.ExternalCapabilityID); basisID != "" {
			capability, exists := m.ExternalCapabilityByID(basisID)
			if !exists {
				return fmt.Errorf("%w: external calibration capability %s", domain.ErrExternalBasisUnavailable, basisID)
			}
			if item.Scope == "" || item.Unit == "" {
				return fmt.Errorf("%w: case %s must declare the same scope and unit before referencing an external basis", domain.ErrExternalBasisUnavailable, item.ID)
			}
			provider, exists := m.ExternalProvider(capability.ProviderID)
			if !exists {
				return fmt.Errorf("%w: external calibration provider for capability %s", domain.ErrExternalBasisUnavailable, capability.ID)
			}
			if err := capability.Covers(item.Scope, item.Unit, item.Nominal, input.Observed, at); err != nil {
				return err
			}
			captured, err := domain.NewExternalCalibrationBasis(capability, provider, at)
			if err != nil {
				return err
			}
			basis = &captured
		}
		measurement, err := domain.NewMeasurement(
			m.NextID("measurement"),
			item.ID,
			input.Observed,
			item.Nominal,
			item.Tolerance,
			input.Technician,
			basis,
			at,
		)
		if err != nil {
			return err
		}
		if err := item.RecordMeasurement(measurement.Accepted, at); err != nil {
			return err
		}
		if err := m.AppendMeasurement(measurement); err != nil {
			return err
		}
		if err := m.UpdateCase(item); err != nil {
			return err
		}
		kind := "measurement_rejected"
		detail := "measurement exceeded tolerance and requires another measurement"
		if measurement.Accepted {
			kind = "measurement_accepted"
			detail = "measurement is within tolerance and ready for review"
		}
		if basis != nil {
			detail = fmt.Sprintf("%s; external basis %s from %s, capability %s, scope %s %s, valid %s to %s, referenced at %s",
				detail,
				basis.CertificateNumber,
				basis.ProviderName,
				basis.CapabilityID,
				basis.Scope,
				basis.Unit,
				basis.ValidFrom.Format("2006-01-02"),
				basis.ValidUntil.Format("2006-01-02"),
				basis.ReferencedAt.UTC().Format(time.RFC3339),
			)
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			kind,
			measurement.Technician,
			detail,
			at,
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = caseResponse(m, item)
		return nil
	})
	return result, err
}

func (s *Service) ReopenCase(ctx context.Context, caseID string, input contracts.ReopenCaseRequest) (contracts.CaseResponse, error) {
	var result contracts.CaseResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		id := strings.TrimSpace(caseID)
		item, exists := m.Case(id)
		if !exists {
			return fmt.Errorf("%w: case %s", domain.ErrNotFound, id)
		}
		if err := item.Reopen(s.now()); err != nil {
			return err
		}
		if err := m.UpdateCase(item); err != nil {
			return err
		}
		actor := strings.TrimSpace(input.Actor)
		reason := strings.TrimSpace(input.Reason)
		if actor == "" || reason == "" {
			return fmt.Errorf("%w: actor and reason are required", domain.ErrInvalidInput)
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			"case_reopened",
			actor,
			reason,
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = caseResponse(m, item)
		return nil
	})
	return result, err
}

func (s *Service) SealCase(ctx context.Context, caseID string, input contracts.SealCaseRequest) (contracts.CaseResponse, error) {
	var result contracts.CaseResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		id := strings.TrimSpace(caseID)
		item, exists := m.Case(id)
		if !exists {
			return fmt.Errorf("%w: case %s", domain.ErrNotFound, id)
		}
		if _, exists := m.Seal(item.ID); exists {
			return fmt.Errorf("%w: case %s", domain.ErrSealAlreadyCreated, item.ID)
		}
		measurement, ok := bestAcceptedMeasurement(m.Measurements(item.ID))
		if !ok {
			return fmt.Errorf("%w: an accepted measurement is required before sealing", domain.ErrStateConflict)
		}
		if err := item.Seal(s.now()); err != nil {
			return err
		}
		seal, err := domain.NewEvidenceSeal(
			m.NextID("seal"),
			item.ID,
			item.DeviceID,
			measurement.ID,
			input.Reviewer,
			input.Summary,
			measurement.ExternalBasis,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutSeal(seal); err != nil {
			return err
		}
		if err := m.UpdateCase(item); err != nil {
			return err
		}
		sealDetail := seal.Summary
		if seal.ExternalBasis != nil {
			sealDetail = fmt.Sprintf("%s; sealed evidence retains external basis %s from %s capability %s referenced at %s",
				sealDetail,
				seal.ExternalBasis.CertificateNumber,
				seal.ExternalBasis.ProviderName,
				seal.ExternalBasis.CapabilityID,
				seal.ExternalBasis.ReferencedAt.UTC().Format(time.RFC3339),
			)
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			"case_sealed",
			seal.Reviewer,
			sealDetail,
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = caseResponse(m, item)
		return nil
	})
	return result, err
}

func bestAcceptedMeasurement(items []domain.Measurement) (domain.Measurement, bool) {
	var selected domain.Measurement
	found := false
	for _, item := range items {
		if !item.Accepted {
			continue
		}
		if !found || item.AbsoluteDeviation() < selected.AbsoluteDeviation() {
			selected = item
			found = true
		}
	}
	return selected, found
}

type externalProviderReader interface {
	ExternalCapabilities(providerID string) []domain.ExternalCalibrationCapability
	ProviderEvents(providerID string) []domain.AuditEvent
}

func providerDetail(reader externalProviderReader, item domain.ExternalCalibrationProvider) contracts.ExternalProviderDetailResponse {
	capabilities := reader.ExternalCapabilities(item.ID)
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].ID < capabilities[j].ID })
	events := reader.ProviderEvents(item.ID)
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})
	return contracts.ExternalProviderDetailResponse{
		Provider:     item,
		Capabilities: capabilities,
		Events:       events,
	}
}

func caseResponse(m *repository.Mutation, item domain.VerificationCase) contracts.CaseResponse {
	result := contracts.CaseResponse{
		Case:         item,
		Measurements: m.Measurements(item.ID),
		Events:       m.Events(item.ID),
	}
	if seal, exists := m.Seal(item.ID); exists {
		result.Seal = &seal
	}
	return result
}

func caseResponseFromSnapshot(snapshot repository.Snapshot, item domain.VerificationCase) contracts.CaseResponse {
	result := contracts.CaseResponse{
		Case:         item,
		Measurements: snapshot.Measurements(item.ID),
		Events:       snapshot.Events(item.ID),
	}
	if seal, exists := snapshot.Seal(item.ID); exists {
		result.Seal = &seal
	}
	return result
}
