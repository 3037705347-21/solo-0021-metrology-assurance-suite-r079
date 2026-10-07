package repository

import (
	"context"
	"fmt"
	"sync"

	"example.com/solo-0021-metrology-assurance-suite/internal/domain"
)

type Repository struct {
	mu                   sync.RWMutex
	devices              map[string]domain.Device
	cases                map[string]domain.VerificationCase
	measurements         map[string][]domain.Measurement
	seals                map[string]domain.EvidenceSeal
	externalProviders    map[string]domain.ExternalCalibrationProvider
	externalCapabilities map[string]map[string]domain.ExternalCalibrationCapability
	events               []domain.AuditEvent
	counters             map[string]int
}

func New() *Repository {
	return &Repository{
		devices:              make(map[string]domain.Device),
		cases:                make(map[string]domain.VerificationCase),
		measurements:         make(map[string][]domain.Measurement),
		seals:                make(map[string]domain.EvidenceSeal),
		externalProviders:    make(map[string]domain.ExternalCalibrationProvider),
		externalCapabilities: make(map[string]map[string]domain.ExternalCalibrationCapability),
		counters:             make(map[string]int),
	}
}

type Snapshot struct {
	devices              map[string]domain.Device
	cases                map[string]domain.VerificationCase
	measurements         map[string][]domain.Measurement
	seals                map[string]domain.EvidenceSeal
	externalProviders    map[string]domain.ExternalCalibrationProvider
	externalCapabilities map[string]map[string]domain.ExternalCalibrationCapability
	events               []domain.AuditEvent
}

func (s Snapshot) Device(id string) (domain.Device, bool) {
	item, ok := s.devices[id]
	return item, ok
}

func (s Snapshot) DeviceByTag(tag string) (domain.Device, bool) {
	for _, item := range s.devices {
		if item.AssetTag == tag {
			return item, true
		}
	}
	return domain.Device{}, false
}

func (s Snapshot) Case(id string) (domain.VerificationCase, bool) {
	item, ok := s.cases[id]
	return item, ok
}

func (s Snapshot) Measurements(caseID string) []domain.Measurement {
	return append([]domain.Measurement(nil), s.measurements[caseID]...)
}

func (s Snapshot) Seal(caseID string) (domain.EvidenceSeal, bool) {
	item, ok := s.seals[caseID]
	return item, ok
}

func (s Snapshot) Events(caseID string) []domain.AuditEvent {
	var result []domain.AuditEvent
	for _, item := range s.events {
		if item.CaseID == caseID {
			result = append(result, item)
		}
	}
	return result
}

func (s Snapshot) AllDevices() []domain.Device {
	result := make([]domain.Device, 0, len(s.devices))
	for _, item := range s.devices {
		result = append(result, item)
	}
	return result
}

func (s Snapshot) AllCases() []domain.VerificationCase {
	result := make([]domain.VerificationCase, 0, len(s.cases))
	for _, item := range s.cases {
		result = append(result, item)
	}
	return result
}

func (s Snapshot) ExternalProvider(id string) (domain.ExternalCalibrationProvider, bool) {
	item, ok := s.externalProviders[id]
	return item, ok
}

func (s Snapshot) ExternalProviderByName(name string) (domain.ExternalCalibrationProvider, bool) {
	normalized := domain.NormalizeProviderName(name)
	for _, item := range s.externalProviders {
		if domain.NormalizeProviderName(item.Name) == normalized {
			return item, true
		}
	}
	return domain.ExternalCalibrationProvider{}, false
}

func (s Snapshot) ExternalProviders() []domain.ExternalCalibrationProvider {
	result := make([]domain.ExternalCalibrationProvider, 0, len(s.externalProviders))
	for _, item := range s.externalProviders {
		result = append(result, item)
	}
	return result
}

func (s Snapshot) ExternalCapability(providerID, capabilityID string) (domain.ExternalCalibrationCapability, bool) {
	capabilities, ok := s.externalCapabilities[providerID]
	if !ok {
		return domain.ExternalCalibrationCapability{}, false
	}
	item, ok := capabilities[capabilityID]
	return item, ok
}

func (s Snapshot) ExternalCapabilityByID(capabilityID string) (domain.ExternalCalibrationCapability, bool) {
	for _, capabilities := range s.externalCapabilities {
		if item, ok := capabilities[capabilityID]; ok {
			return item, true
		}
	}
	return domain.ExternalCalibrationCapability{}, false
}

func (s Snapshot) ExternalCapabilities(providerID string) []domain.ExternalCalibrationCapability {
	values := s.externalCapabilities[providerID]
	result := make([]domain.ExternalCalibrationCapability, 0, len(values))
	for _, item := range values {
		result = append(result, item)
	}
	return result
}

func (s Snapshot) ProviderEvents(providerID string) []domain.AuditEvent {
	var result []domain.AuditEvent
	for _, item := range s.events {
		if item.SubjectType == "external_calibration_provider" && item.SubjectID == providerID {
			result = append(result, item)
		}
	}
	return result
}

type Mutation struct {
	devices              map[string]domain.Device
	cases                map[string]domain.VerificationCase
	measurements         map[string][]domain.Measurement
	seals                map[string]domain.EvidenceSeal
	externalProviders    map[string]domain.ExternalCalibrationProvider
	externalCapabilities map[string]map[string]domain.ExternalCalibrationCapability
	events               []domain.AuditEvent
	counters             map[string]int
}

func (m *Mutation) NextID(prefix string) string {
	m.counters[prefix]++
	return fmt.Sprintf("%s-%04d", prefix, m.counters[prefix])
}

func (m *Mutation) ExternalProvider(id string) (domain.ExternalCalibrationProvider, bool) {
	item, ok := m.externalProviders[id]
	return item, ok
}

func (m *Mutation) ExternalProviderByName(name string) (domain.ExternalCalibrationProvider, bool) {
	for _, item := range m.externalProviders {
		if domain.NormalizeProviderName(item.Name) == domain.NormalizeProviderName(name) {
			return item, true
		}
	}
	return domain.ExternalCalibrationProvider{}, false
}

func (m *Mutation) PutExternalProvider(item domain.ExternalCalibrationProvider) error {
	if _, exists := m.externalProviders[item.ID]; exists {
		return fmt.Errorf("%w: external calibration provider %s", domain.ErrStateConflict, item.ID)
	}
	m.externalProviders[item.ID] = item
	if _, exists := m.externalCapabilities[item.ID]; !exists {
		m.externalCapabilities[item.ID] = make(map[string]domain.ExternalCalibrationCapability)
	}
	return nil
}

func (m *Mutation) UpdateExternalProvider(item domain.ExternalCalibrationProvider) error {
	if _, exists := m.externalProviders[item.ID]; !exists {
		return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, item.ID)
	}
	m.externalProviders[item.ID] = item
	return nil
}

func (m *Mutation) ExternalCapability(providerID, capabilityID string) (domain.ExternalCalibrationCapability, bool) {
	capabilities, ok := m.externalCapabilities[providerID]
	if !ok {
		return domain.ExternalCalibrationCapability{}, false
	}
	item, ok := capabilities[capabilityID]
	return item, ok
}

func (m *Mutation) ExternalCapabilityByID(capabilityID string) (domain.ExternalCalibrationCapability, bool) {
	for _, capabilities := range m.externalCapabilities {
		if item, ok := capabilities[capabilityID]; ok {
			return item, true
		}
	}
	return domain.ExternalCalibrationCapability{}, false
}

func (m *Mutation) ExternalCapabilities(providerID string) []domain.ExternalCalibrationCapability {
	values := m.externalCapabilities[providerID]
	result := make([]domain.ExternalCalibrationCapability, 0, len(values))
	for _, item := range values {
		result = append(result, item)
	}
	return result
}

func (m *Mutation) PutExternalCapability(item domain.ExternalCalibrationCapability) error {
	if _, exists := m.externalProviders[item.ProviderID]; !exists {
		return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, item.ProviderID)
	}
	capabilities := m.externalCapabilities[item.ProviderID]
	if _, exists := capabilities[item.ID]; exists {
		return fmt.Errorf("%w: external calibration capability %s", domain.ErrStateConflict, item.ID)
	}
	capabilities[item.ID] = item
	return nil
}

func (m *Mutation) UpdateExternalCapability(item domain.ExternalCalibrationCapability) error {
	capabilities, exists := m.externalCapabilities[item.ProviderID]
	if !exists {
		return fmt.Errorf("%w: external calibration provider %s", domain.ErrNotFound, item.ProviderID)
	}
	if _, exists := capabilities[item.ID]; !exists {
		return fmt.Errorf("%w: external calibration capability %s", domain.ErrNotFound, item.ID)
	}
	capabilities[item.ID] = item
	return nil
}

func (m *Mutation) ProviderEvents(providerID string) []domain.AuditEvent {
	var result []domain.AuditEvent
	for _, item := range m.events {
		if item.SubjectType == "external_calibration_provider" && item.SubjectID == providerID {
			result = append(result, item)
		}
	}
	return result
}

func (m *Mutation) Device(id string) (domain.Device, bool) {
	item, ok := m.devices[id]
	return item, ok
}

func (m *Mutation) DeviceByTag(tag string) (domain.Device, bool) {
	for _, item := range m.devices {
		if item.AssetTag == tag {
			return item, true
		}
	}
	return domain.Device{}, false
}

func (m *Mutation) PutDevice(item domain.Device) error {
	if _, exists := m.devices[item.ID]; exists {
		return fmt.Errorf("%w: device %s", domain.ErrStateConflict, item.ID)
	}
	m.devices[item.ID] = item
	return nil
}

func (m *Mutation) UpdateDevice(item domain.Device) error {
	if _, exists := m.devices[item.ID]; !exists {
		return fmt.Errorf("%w: device %s", domain.ErrNotFound, item.ID)
	}
	m.devices[item.ID] = item
	return nil
}

func (m *Mutation) Case(id string) (domain.VerificationCase, bool) {
	item, ok := m.cases[id]
	return item, ok
}

func (m *Mutation) PutCase(item domain.VerificationCase) error {
	if _, exists := m.cases[item.ID]; exists {
		return fmt.Errorf("%w: case %s", domain.ErrStateConflict, item.ID)
	}
	m.cases[item.ID] = item
	return nil
}

func (m *Mutation) UpdateCase(item domain.VerificationCase) error {
	if _, exists := m.cases[item.ID]; !exists {
		return fmt.Errorf("%w: case %s", domain.ErrNotFound, item.ID)
	}
	m.cases[item.ID] = item
	return nil
}

func (m *Mutation) ActiveCaseForDevice(deviceID string) (domain.VerificationCase, bool) {
	for _, item := range m.cases {
		if item.DeviceID == deviceID && item.IsUnfinished() {
			return item, true
		}
	}
	return domain.VerificationCase{}, false
}

func (m *Mutation) AppendMeasurement(item domain.Measurement) error {
	if _, exists := m.cases[item.CaseID]; !exists {
		return fmt.Errorf("%w: case %s", domain.ErrNotFound, item.CaseID)
	}
	m.measurements[item.CaseID] = append(m.measurements[item.CaseID], item)
	return nil
}

func (m *Mutation) Measurements(caseID string) []domain.Measurement {
	return append([]domain.Measurement(nil), m.measurements[caseID]...)
}

func (m *Mutation) Seal(caseID string) (domain.EvidenceSeal, bool) {
	item, ok := m.seals[caseID]
	return item, ok
}

func (m *Mutation) PutSeal(item domain.EvidenceSeal) error {
	if _, exists := m.seals[item.CaseID]; exists {
		return fmt.Errorf("%w: case %s", domain.ErrSealAlreadyCreated, item.CaseID)
	}
	m.seals[item.CaseID] = item
	return nil
}

func (m *Mutation) AppendEvent(item domain.AuditEvent) {
	m.events = append(m.events, item)
}

func (m *Mutation) Events(caseID string) []domain.AuditEvent {
	var result []domain.AuditEvent
	for _, item := range m.events {
		if item.CaseID == caseID {
			result = append(result, item)
		}
	}
	return result
}

func (r *Repository) Read(ctx context.Context, fn func(Snapshot) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return fn(r.snapshot())
}

func (r *Repository) Mutate(ctx context.Context, fn func(*Mutation) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	next := &Mutation{
		devices:              cloneDevices(r.devices),
		cases:                cloneCases(r.cases),
		measurements:         cloneMeasurements(r.measurements),
		seals:                cloneSeals(r.seals),
		externalProviders:    cloneExternalProviders(r.externalProviders),
		externalCapabilities: cloneExternalCapabilities(r.externalCapabilities),
		events:               append([]domain.AuditEvent(nil), r.events...),
		counters:             cloneCounters(r.counters),
	}
	if err := fn(next); err != nil {
		return err
	}
	r.devices = next.devices
	r.cases = next.cases
	r.measurements = next.measurements
	r.seals = next.seals
	r.externalProviders = next.externalProviders
	r.externalCapabilities = next.externalCapabilities
	r.events = next.events
	r.counters = next.counters
	return nil
}

func (r *Repository) snapshot() Snapshot {
	return Snapshot{
		devices:              cloneDevices(r.devices),
		cases:                cloneCases(r.cases),
		measurements:         cloneMeasurements(r.measurements),
		seals:                cloneSeals(r.seals),
		externalProviders:    cloneExternalProviders(r.externalProviders),
		externalCapabilities: cloneExternalCapabilities(r.externalCapabilities),
		events:               append([]domain.AuditEvent(nil), r.events...),
	}
}

func cloneDevices(values map[string]domain.Device) map[string]domain.Device {
	result := make(map[string]domain.Device, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneCases(values map[string]domain.VerificationCase) map[string]domain.VerificationCase {
	result := make(map[string]domain.VerificationCase, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneMeasurements(values map[string][]domain.Measurement) map[string][]domain.Measurement {
	result := make(map[string][]domain.Measurement, len(values))
	for key, value := range values {
		cloned := make([]domain.Measurement, 0, len(value))
		for _, measurement := range value {
			if measurement.ExternalBasis != nil {
				basis := *measurement.ExternalBasis
				measurement.ExternalBasis = &basis
			}
			cloned = append(cloned, measurement)
		}
		result[key] = cloned
	}
	return result
}

func cloneSeals(values map[string]domain.EvidenceSeal) map[string]domain.EvidenceSeal {
	result := make(map[string]domain.EvidenceSeal, len(values))
	for key, value := range values {
		if value.ExternalBasis != nil {
			basis := *value.ExternalBasis
			value.ExternalBasis = &basis
		}
		result[key] = value
	}
	return result
}

func cloneExternalProviders(values map[string]domain.ExternalCalibrationProvider) map[string]domain.ExternalCalibrationProvider {
	result := make(map[string]domain.ExternalCalibrationProvider, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneExternalCapabilities(values map[string]map[string]domain.ExternalCalibrationCapability) map[string]map[string]domain.ExternalCalibrationCapability {
	result := make(map[string]map[string]domain.ExternalCalibrationCapability, len(values))
	for providerID, capabilities := range values {
		cloned := make(map[string]domain.ExternalCalibrationCapability, len(capabilities))
		for capabilityID, value := range capabilities {
			cloned[capabilityID] = value
		}
		result[providerID] = cloned
	}
	return result
}

func cloneCounters(values map[string]int) map[string]int {
	result := make(map[string]int, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
