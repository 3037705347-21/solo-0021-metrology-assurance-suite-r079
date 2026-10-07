package assurance

import (
	"context"
	"fmt"
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

func (s *Service) RegisterLab(ctx context.Context, input contracts.RegisterLabRequest) (domain.CalibrationLab, error) {
	var created domain.CalibrationLab
	name := domain.NormalizeLabName(input.Name)
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		if _, exists := m.LabByName(name); exists {
			return fmt.Errorf("%w: %s", domain.ErrDuplicateLabName, strings.TrimSpace(input.Name))
		}
		item, err := domain.NewCalibrationLab(
			m.NextID("lab"),
			input.Name,
			input.Accreditation,
			input.Steward,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutLab(item); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"lab",
			item.ID,
			"",
			"lab_registered",
			item.Steward,
			fmt.Sprintf("external calibration lab registered with accreditation %s", item.Accreditation),
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

func (s *Service) Lab(ctx context.Context, id string) (contracts.LabResponse, error) {
	var result contracts.LabResponse
	err := s.repository.Read(ctx, func(snapshot repository.Snapshot) error {
		item, exists := snapshot.Lab(strings.TrimSpace(id))
		if !exists {
			return fmt.Errorf("%w: lab %s", domain.ErrNotFound, id)
		}
		result = contracts.LabResponse{
			Lab:    item,
			Scopes: snapshot.ScopesForLab(item.ID),
			Events: snapshot.EventsForSubject("lab", item.ID),
		}
		return nil
	})
	return result, err
}

func (s *Service) AddScope(ctx context.Context, labID string, input contracts.UpsertScopeRequest) (contracts.LabResponse, error) {
	var result contracts.LabResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		lab, exists := m.Lab(strings.TrimSpace(labID))
		if !exists {
			return fmt.Errorf("%w: lab %s", domain.ErrNotFound, labID)
		}
		actor, err := requireActor(input.Actor)
		if err != nil {
			return err
		}
		scope, err := domain.NewCapabilityScope(
			m.NextID("scope"),
			lab.ID,
			input.Parameter,
			input.MinValue,
			input.MaxValue,
			input.Uncertainty,
			input.ValidFrom,
			input.ValidUntil,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.PutScope(scope); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"lab",
			lab.ID,
			"",
			"lab_scope_added",
			actor,
			scopeDetail("capability scope added", scope),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = labResponse(m, lab)
		return nil
	})
	return result, err
}

func (s *Service) UpdateScope(ctx context.Context, labID, scopeID string, input contracts.UpsertScopeRequest) (contracts.LabResponse, error) {
	var result contracts.LabResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		lab, exists := m.Lab(strings.TrimSpace(labID))
		if !exists {
			return fmt.Errorf("%w: lab %s", domain.ErrNotFound, labID)
		}
		scope, exists := m.Scope(strings.TrimSpace(scopeID))
		if !exists || scope.LabID != lab.ID {
			return fmt.Errorf("%w: scope %s", domain.ErrNotFound, scopeID)
		}
		actor, err := requireActor(input.Actor)
		if err != nil {
			return err
		}
		updated, err := domain.NewCapabilityScope(
			scope.ID,
			lab.ID,
			input.Parameter,
			input.MinValue,
			input.MaxValue,
			input.Uncertainty,
			input.ValidFrom,
			input.ValidUntil,
			s.now(),
		)
		if err != nil {
			return err
		}
		if err := m.UpdateScope(updated); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"lab",
			lab.ID,
			"",
			"lab_scope_updated",
			actor,
			scopeDetail("capability scope updated; previously recorded measurements and seals keep their original snapshot", updated),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = labResponse(m, lab)
		return nil
	})
	return result, err
}

func (s *Service) SuspendLab(ctx context.Context, labID string, input contracts.SuspendLabRequest) (contracts.LabResponse, error) {
	var result contracts.LabResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		lab, exists := m.Lab(strings.TrimSpace(labID))
		if !exists {
			return fmt.Errorf("%w: lab %s", domain.ErrNotFound, labID)
		}
		actor, err := requireActor(input.Actor)
		if err != nil {
			return err
		}
		reason := strings.TrimSpace(input.Reason)
		if reason == "" {
			return fmt.Errorf("%w: reason is required", domain.ErrInvalidInput)
		}
		if err := lab.Suspend(s.now()); err != nil {
			return err
		}
		if err := m.UpdateLab(lab); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"lab",
			lab.ID,
			"",
			"lab_suspended",
			actor,
			fmt.Sprintf("lab suspended; new measurements cannot reference its scopes: %s", reason),
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = labResponse(m, lab)
		return nil
	})
	return result, err
}

func (s *Service) ReinstateLab(ctx context.Context, labID string, input contracts.ReinstateLabRequest) (contracts.LabResponse, error) {
	var result contracts.LabResponse
	err := s.repository.Mutate(ctx, func(m *repository.Mutation) error {
		lab, exists := m.Lab(strings.TrimSpace(labID))
		if !exists {
			return fmt.Errorf("%w: lab %s", domain.ErrNotFound, labID)
		}
		actor, err := requireActor(input.Actor)
		if err != nil {
			return err
		}
		if err := lab.Reinstate(s.now()); err != nil {
			return err
		}
		if err := m.UpdateLab(lab); err != nil {
			return err
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"lab",
			lab.ID,
			"",
			"lab_reinstated",
			actor,
			"lab reinstated; its valid scopes can back new measurements again",
			s.now(),
		)
		if err != nil {
			return err
		}
		m.AppendEvent(event)
		result = labResponse(m, lab)
		return nil
	})
	return result, err
}

func requireActor(actor string) (string, error) {
	trimmed := strings.TrimSpace(actor)
	if trimmed == "" {
		return "", fmt.Errorf("%w: actor is required", domain.ErrInvalidInput)
	}
	return trimmed, nil
}

func scopeDetail(prefix string, scope domain.CapabilityScope) string {
	return fmt.Sprintf("%s: %s parameter %s range [%g, %g] uncertainty %g valid %s to %s",
		prefix,
		scope.ID,
		scope.Parameter,
		scope.MinValue,
		scope.MaxValue,
		scope.Uncertainty,
		scope.ValidFrom.Format("2006-01-02"),
		scope.ValidUntil.Format("2006-01-02"),
	)
}

func basisNote(basis domain.ExternalBasis) string {
	return fmt.Sprintf("external basis %s (%s, accreditation %s) scope %s parameter %s range [%g, %g] valid until %s",
		basis.LabName,
		basis.LabID,
		basis.Accreditation,
		basis.ScopeID,
		basis.Parameter,
		basis.MinValue,
		basis.MaxValue,
		basis.ValidUntil.Format("2006-01-02"),
	)
}

func labResponse(m *repository.Mutation, lab domain.CalibrationLab) contracts.LabResponse {
	return contracts.LabResponse{
		Lab:    lab,
		Scopes: m.ScopesForLab(lab.ID),
		Events: m.EventsForSubject("lab", lab.ID),
	}
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
		measurement, err := domain.NewMeasurement(
			m.NextID("measurement"),
			item.ID,
			input.Observed,
			item.Nominal,
			item.Tolerance,
			input.Technician,
			s.now(),
		)
		if err != nil {
			return err
		}
		if input.ExternalBasis != nil {
			basis, err := resolveExternalBasis(m, *input.ExternalBasis, measurement.Observed, s.now())
			if err != nil {
				return err
			}
			if err := measurement.AttachExternalBasis(basis); err != nil {
				return err
			}
		}
		if err := item.RecordMeasurement(measurement.Accepted, s.now()); err != nil {
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
		if measurement.ExternalBasis != nil {
			detail += "; references " + basisNote(*measurement.ExternalBasis)
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			kind,
			measurement.Technician,
			detail,
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

// resolveExternalBasis proves the referenced lab capability was usable at the
// moment of recording and snapshots it, so later lab maintenance cannot
// rewrite the basis a measurement relied on.
func resolveExternalBasis(m *repository.Mutation, ref contracts.ExternalReferenceRequest, observed float64, at time.Time) (domain.ExternalBasis, error) {
	labID := strings.TrimSpace(ref.LabID)
	scopeID := strings.TrimSpace(ref.ScopeID)
	if labID == "" || scopeID == "" {
		return domain.ExternalBasis{}, fmt.Errorf("%w: external basis lab id and scope id are required", domain.ErrInvalidInput)
	}
	lab, exists := m.Lab(labID)
	if !exists {
		return domain.ExternalBasis{}, fmt.Errorf("%w: lab %s", domain.ErrNotFound, labID)
	}
	if !lab.IsActive() {
		return domain.ExternalBasis{}, fmt.Errorf("%w: lab %s is %s", domain.ErrCapabilityUnavailable, lab.ID, lab.Status)
	}
	scope, exists := m.Scope(scopeID)
	if !exists {
		return domain.ExternalBasis{}, fmt.Errorf("%w: scope %s", domain.ErrNotFound, scopeID)
	}
	if scope.LabID != lab.ID {
		return domain.ExternalBasis{}, fmt.Errorf("%w: scope %s does not belong to lab %s", domain.ErrInvalidInput, scope.ID, lab.ID)
	}
	if !scope.ValidAt(at) {
		return domain.ExternalBasis{}, fmt.Errorf("%w: scope %s validity %s to %s does not cover %s",
			domain.ErrCapabilityUnavailable,
			scope.ID,
			scope.ValidFrom.Format("2006-01-02"),
			scope.ValidUntil.Format("2006-01-02"),
			at.UTC().Format("2006-01-02"),
		)
	}
	if !scope.Covers(observed) {
		return domain.ExternalBasis{}, fmt.Errorf("%w: observed value %g is outside scope %s range [%g, %g]",
			domain.ErrScopeMismatch,
			observed,
			scope.ID,
			scope.MinValue,
			scope.MaxValue,
		)
	}
	return domain.NewExternalBasis(lab, scope, at)
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
		detail := seal.Summary
		if seal.ExternalBasis != nil {
			detail += "; sealed with " + basisNote(*seal.ExternalBasis)
		}
		event, err := domain.NewAuditEvent(
			m.NextID("event"),
			"case",
			item.ID,
			item.ID,
			"case_sealed",
			seal.Reviewer,
			detail,
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
