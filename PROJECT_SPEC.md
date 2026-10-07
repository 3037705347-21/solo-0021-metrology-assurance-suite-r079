# Metrology Assurance Suite

## Project Goal

Metrology Assurance Suite is a local HTTP service that keeps a traceable record
of laboratory measurement assurance work. It registers devices, opens a
verification case for one device, records technician measurements, requests a
repeat measurement when tolerance is exceeded, and seals the final evidence
record after review.

The service is intentionally self-contained. It uses an in-memory repository
with atomic copy-on-write mutations and does not require a database or an
external service.

## Users

- Laboratory steward: registers devices and opens verification cases.
- Measurement technician: records observed values for an open case.
- Quality verifier: reviews accepted measurements and seals the case.

## Core Entities

- Device: a laboratory asset with a unique asset tag, model, room, steward, and
  lifecycle state.
- VerificationCase: one bounded assurance activity for a device, with a nominal
  value, tolerance, current state, version, and assigned steward.
- Measurement: an observed value, computed deviation, acceptance decision,
  technician, and timestamp.
- EvidenceSeal: the immutable closing record for a reviewed case, including the
  selected accepted measurement and a frozen copy of any external basis.
- ExternalCalibrationProvider: an external calibration organization with
  accreditation identity and the person responsible for maintaining it.
- ExternalCalibrationCapability: one provider's accredited scope, unit,
  measurable range, certificate number, and validity dates.
- ExternalCalibrationBasis: an immutable source snapshot attached to a
  measurement, recording who performed the calibration and what capability was
  available at that moment.
- AuditEvent: an append-only trace of meaningful case, device, and external
  calibration provider changes.

## Workflows

### 1. Register Device

The steward sends `POST /devices` with an asset tag, model, room, and steward.
The service validates the request, normalizes the asset tag, rejects duplicate
tags, and creates an active device.

### 2. Open Verification Case

The steward sends `POST /cases` with a device ID, nominal value, tolerance, and
owner. The service verifies that the device exists, is active, and has no
unfinished case. A valid request creates a case in `awaiting_measurement`.

### 3. Record Measurement

A technician sends `POST /cases/{id}/measurements`. The case must be
`awaiting_measurement`. The service computes the signed deviation from the
nominal value. An in-tolerance measurement moves the case to
`review_ready`; an out-of-tolerance measurement moves it to
`recheck_required`.

### 4. Reopen Verification Case

The steward sends `POST /cases/{id}/reopen` after an out-of-tolerance result.
Only `recheck_required` cases may move back to `awaiting_measurement`. The
service records the actor and reason in the audit trail.

### 5. Maintain External Calibration Provider And Capability

A responsible maintainer registers an external calibration provider and one or
more capabilities with an accredited scope, unit, measurable range, certificate
number, and validity period. Provider and capability records may later be
corrected; corrections increment their version and append an event, but do not
delete or rewrite snapshots already attached to measurements.

### 6. Reference External Basis In A Measurement

When recording a measurement, the technician may reference an external
calibration capability. The case must declare the same scope and unit. The
service rejects the new measurement if the capability does not exist, is
outside its validity period at measurement time, has a scope/unit mismatch, or
does not cover the observed value. An accepted reference captures the provider,
accreditation number, certificate, scope, range, validity, and reference time on
the measurement.

### 7. Seal Verification

The verifier sends `POST /cases/{id}/seal` after an accepted measurement. Only
`review_ready` cases may be sealed. The service selects the accepted
measurement with the smallest absolute deviation, creates exactly one evidence
seal, moves the case to `sealed`, and rejects later mutations.

## State And Rules

Device state:

- `active -> retired`
- `retired` is terminal.

VerificationCase state:

- `awaiting_measurement -> review_ready`
- `awaiting_measurement -> recheck_required`
- `recheck_required -> awaiting_measurement`
- `review_ready -> sealed`
- `sealed` is terminal.

Required invariants:

- Asset tags are unique after case-insensitive normalization.
- A device has at most one unfinished verification case.
- Measurements are accepted only while a case is awaiting measurement.
- A tolerance must be positive.
- A case can have at most one evidence seal.
- A sealed case cannot accept measurements, reopen, or seal again.
- An external calibration capability can be referenced only when it exists, is
  valid at measurement time, matches the case scope and unit, and covers the
  observed value.
- Provider and capability maintenance changes only future references. It never
  rewrites a measurement basis or the basis retained by an evidence seal.
- Each measurement event and seal event records enough detail to explain the
  referenced external certificate, provider, capability, scope, validity, and
  reference time.
- Every mutation that changes a case also appends an audit event in the same
  repository commit.

## Modules

- `cmd/metrology-service`: process entrypoint and graceful HTTP shutdown.
- `cmd/workflowcheck`: bounded smoke checks that call the public HTTP API.
- `internal/httpapi`: routing, request decoding, and stable error mapping.
- `internal/assurance`: workflow orchestration and atomic decisions.
- `internal/domain`: entities, validation, state transitions, and errors.
- `internal/repository`: mutex-protected in-memory persistence with copy-on-write
  commits.
- `internal/contracts`: request and response contracts shared by the HTTP layer.

## Public Interfaces

- `GET /healthz`
- `POST /devices`
- `GET /devices/{id}`
- `GET /external-calibration-providers`
- `POST /external-calibration-providers`
- `GET /external-calibration-providers/{id}`
- `PUT /external-calibration-providers/{id}`
- `GET /external-calibration-providers/{id}/events`
- `POST /external-calibration-providers/{id}/capabilities`
- `GET /external-calibration-providers/{id}/capabilities/{capabilityId}`
- `PUT /external-calibration-providers/{id}/capabilities/{capabilityId}`
- `POST /cases`
- `GET /cases/{id}`
- `POST /cases/{id}/measurements`
- `POST /cases/{id}/reopen`
- `POST /cases/{id}/seal`
- `GET /cases/{id}/events`

## Validation Plan

The baseline build command is `go build ./...`. Each declared workflow has a
bounded production smoke command under `cmd/workflowcheck`. The command starts
an HTTP test server in the same process, calls the public routes, and verifies
both success and failure behavior. The external calibration check verifies a
valid reference, an out-of-range rejection, a post-maintenance frozen basis,
and an expired/mismatched rejection for a new measurement. The seal check also
races two concurrent
seal requests to prove that the repository mutation lock permits exactly one
seal.

Tests are intentionally deferred for this initialization baseline. The later
engineering-task stage will add unit and integration tests, including red/green
verification for the workflows and their failure paths.
