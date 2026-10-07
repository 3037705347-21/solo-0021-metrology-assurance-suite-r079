# Metrology Assurance Suite

## Project Goal

Metrology Assurance Suite is a local HTTP service that keeps a traceable record
of laboratory measurement assurance work. It registers devices, opens a
verification case for one device, records technician measurements, requests a
repeat measurement when tolerance is exceeded, and seals the final evidence
record after review. Measurements that rely on an external calibration result
can reference the accredited capability of a registered external calibration
lab, and the service pins an immutable snapshot of that reference into the
measurement and the evidence seal.

The service is intentionally self-contained. It uses an in-memory repository
with atomic copy-on-write mutations and does not require a database or an
external service.

## Users

- Laboratory steward: registers devices, opens verification cases, and
  maintains external calibration labs with their accredited scopes.
- Measurement technician: records observed values for an open case.
- Quality verifier: reviews accepted measurements and seals the case.

## Core Entities

- Device: a laboratory asset with a unique asset tag, model, room, steward, and
  lifecycle state.
- VerificationCase: one bounded assurance activity for a device, with a nominal
  value, tolerance, current state, version, and assigned steward.
- Measurement: an observed value, computed deviation, acceptance decision,
  technician, timestamp, and an optional external basis snapshot.
- EvidenceSeal: the immutable closing record for a reviewed case, including the
  selected accepted measurement and its external basis snapshot when present.
- CalibrationLab: an external calibration organization with a unique name,
  accreditation identifier, steward, and lifecycle state.
- CapabilityScope: one accredited measurement range of a lab, with parameter,
  value range, uncertainty, and a validity window.
- ExternalBasis: the immutable snapshot of the lab and scope a measurement
  referenced when it was recorded.
- AuditEvent: an append-only trace of meaningful case, device, and lab changes.

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

### 5. Seal Verification

The verifier sends `POST /cases/{id}/seal` after an accepted measurement. Only
`review_ready` cases may be sealed. The service selects the accepted
measurement with the smallest absolute deviation, creates exactly one evidence
seal, moves the case to `sealed`, and rejects later mutations. When the
selected measurement carries an external basis, the seal pins a copy of that
snapshot.

### 6. Register Calibration Lab

The steward sends `POST /labs` with a name, accreditation identifier, and
steward. The service trims the name, rejects duplicates case-insensitively,
and creates an active lab. `GET /labs/{id}` returns the lab with its scopes
and audit events.

### 7. Maintain Capability Scope

The steward sends `POST /labs/{id}/scopes` to add an accredited scope
(parameter, value range, uncertainty, validity window) and
`PUT /labs/{id}/scopes/{scopeID}` to replace it, for example to extend a
validity window. Updates only affect future references; snapshots already
attached to measurements and seals are never rewritten.

### 8. Suspend And Reinstate Calibration Lab

The steward sends `POST /labs/{id}/suspend` with an actor and reason when the
lab's capability lapses, and `POST /labs/{id}/reinstate` when it is restored.
A suspended lab's scopes cannot back new measurements.

### 9. Record Measurement With External Basis

A technician sends `POST /cases/{id}/measurements` with an `external_basis`
naming a lab and scope. The service proves the lab is active, the scope
belongs to the lab, the validity window covers the recording time, and the
observed value falls inside the accredited range. A valid reference is pinned
as an immutable snapshot on the measurement; an unavailable capability or an
out-of-range value rejects the measurement before any state changes.

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

CalibrationLab state:

- `active -> suspended`
- `suspended -> active`

Required invariants:

- Asset tags are unique after case-insensitive normalization.
- Lab names are unique after case-insensitive normalization.
- A device has at most one unfinished verification case.
- Measurements are accepted only while a case is awaiting measurement.
- A tolerance must be positive.
- A capability scope has a finite range with min below max, a positive
  uncertainty, and a validity window with valid_from before valid_until.
- A measurement may reference an external basis only while the lab is active,
  the scope validity window covers the recording time, and the observed value
  is inside the accredited range.
- External basis snapshots on measurements and seals are immutable; lab and
  scope maintenance never rewrites them.
- A case can have at most one evidence seal.
- A sealed case cannot accept measurements, reopen, or seal again.
- Every mutation that changes a case, device, or lab also appends an audit
  event in the same repository commit.

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
- `POST /cases`
- `GET /cases/{id}`
- `POST /cases/{id}/measurements`
- `POST /cases/{id}/reopen`
- `POST /cases/{id}/seal`
- `GET /cases/{id}/events`
- `POST /labs`
- `GET /labs/{id}`
- `POST /labs/{id}/scopes`
- `PUT /labs/{id}/scopes/{scopeID}`
- `POST /labs/{id}/suspend`
- `POST /labs/{id}/reinstate`

## Validation Plan

The baseline build command is `go build ./...`. Each declared workflow has a
bounded production smoke command under `cmd/workflowcheck`. The command starts
an HTTP test server in the same process, calls the public routes, and verifies
both success and failure behavior. The seal check also races two concurrent
seal requests to prove that the repository mutation lock permits exactly one
seal.

Tests are intentionally deferred for this initialization baseline. The later
engineering-task stage will add unit and integration tests, including red/green
verification for the workflows and their failure paths.
