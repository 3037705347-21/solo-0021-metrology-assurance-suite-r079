# Metrology Assurance Suite

Metrology Assurance Suite is a Go HTTP service for bounded laboratory
measurement assurance work. It registers devices, opens verification cases,
records measurements, handles failed tolerance decisions, and seals traceable
evidence records. Stewards can also register external calibration labs with
their accredited capability scopes and validity windows, so measurements can
reference the external basis that backs them; the reference is validated at
record time and pinned as an immutable snapshot on the measurement and the
evidence seal.

## Run

```bash
go run ./cmd/metrology-service --addr 127.0.0.1:18121
```

The service listens on `127.0.0.1:18121` by default.

## Build And Verify

```bash
go build ./...
go run ./cmd/workflowcheck --workflow register-device
go run ./cmd/workflowcheck --workflow open-verification-case
go run ./cmd/workflowcheck --workflow record-measurement
go run ./cmd/workflowcheck --workflow reopen-verification-case
go run ./cmd/workflowcheck --workflow seal-verification
go run ./cmd/workflowcheck --workflow register-calibration-lab
go run ./cmd/workflowcheck --workflow maintain-capability-scope
go run ./cmd/workflowcheck --workflow record-external-measurement
go run ./cmd/workflowcheck --workflow reject-unavailable-external-basis
go run ./cmd/workflowcheck --workflow preserve-external-basis-history
```

The workflow runner starts a fresh in-process HTTP service and exercises the
public API. It is a production smoke command, not a unit test.

Tests are intentionally deferred for this initialization baseline. The later
engineering-task stage will add the unit and integration test suite and replace
or extend these smoke checks with red/green verification tests.

## API

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

A measurement can carry an `external_basis` object with `lab_id` and
`scope_id`. The service records it only while the lab is active, the scope's
validity window covers the recording time, and the observed value is inside
the accredited range; otherwise the request fails with
`capability_unavailable` or `scope_mismatch` and the case is unchanged.
Accepted references appear as immutable snapshots on the measurement, the
evidence seal, and the audit event details.

## Layout

- `cmd/metrology-service`: executable HTTP server.
- `cmd/workflowcheck`: bounded workflow verification commands.
- `internal/httpapi`: route and JSON boundary.
- `internal/assurance`: workflow orchestration.
- `internal/domain`: state rules and invariants.
- `internal/repository`: copy-on-write in-memory persistence.
- `internal/contracts`: API request and response models.
- `internal/bootstrap`: concrete dependency wiring.

## Inputs And Outputs

Inputs are JSON HTTP requests. Outputs are JSON response envelopes with either a
resource view or a stable error object:

```json
{
  "error": {
    "code": "state_conflict",
    "message": "case is not awaiting measurement"
  }
}
```

The service has no environment variables and no external dependencies.
