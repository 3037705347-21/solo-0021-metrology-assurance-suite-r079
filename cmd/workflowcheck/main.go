package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"time"

	"example.com/solo-0021-metrology-assurance-suite/internal/bootstrap"
)

type apiResponse struct {
	Status int
	Body   map[string]any
	Raw    string
}

type harness struct {
	baseURL string
	client  *http.Client
	server  *httptest.Server
}

func main() {
	workflow := flag.String("workflow", "", "workflow id to verify")
	flag.Parse()

	checks := map[string]func() error{
		"register-device":                   checkRegisterDevice,
		"open-verification-case":            checkOpenVerificationCase,
		"record-measurement":                checkRecordMeasurement,
		"reopen-verification-case":          checkReopenVerificationCase,
		"seal-verification":                 checkSealVerification,
		"register-calibration-lab":          checkRegisterCalibrationLab,
		"maintain-capability-scope":         checkMaintainCapabilityScope,
		"record-external-measurement":       checkRecordExternalMeasurement,
		"reject-unavailable-external-basis": checkRejectUnavailableExternalBasis,
		"preserve-external-basis-history":   checkPreserveExternalBasisHistory,
	}
	run, ok := checks[*workflow]
	if !ok {
		fmt.Fprintf(flag.CommandLine.Output(), "unknown workflow %q\n", *workflow)
		flag.Usage()
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Printf("workflow %s: failed: %v\n", *workflow, err)
		os.Exit(1)
	}
	fmt.Printf("workflow %s: ok\n", *workflow)
}

func checkRegisterDevice() error {
	h := newHarness()
	defer h.close()

	response, err := h.request(http.MethodPost, "/devices", map[string]any{
		"asset_tag": " met-1001 ",
		"model":     "Bench Meter",
		"room":      "Room A",
		"steward":   "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return err
	}
	id, err := nestedString(response.Body, "device", "id")
	if err != nil {
		return err
	}
	tag, err := nestedString(response.Body, "device", "asset_tag")
	if err != nil {
		return err
	}
	if tag != "MET-1001" {
		return fmt.Errorf("asset tag was not normalized: %q", tag)
	}

	read, err := h.request(http.MethodGet, "/devices/"+id, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	readID, err := nestedString(read.Body, "device", "id")
	if err != nil {
		return err
	}
	if readID != id {
		return fmt.Errorf("read returned device %q, expected %q", readID, id)
	}

	duplicate, err := h.request(http.MethodPost, "/devices", map[string]any{
		"asset_tag": "MET-1001",
		"model":     "Other Meter",
		"room":      "Room B",
		"steward":   "Mira",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(duplicate, http.StatusConflict); err != nil {
		return err
	}
	return expectErrorCode(duplicate, "duplicate_asset_tag")
}

func checkOpenVerificationCase() error {
	h := newHarness()
	defer h.close()

	deviceID, err := h.createDevice("MET-1101")
	if err != nil {
		return err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return err
	}
	state, err := nestedString(opened.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "awaiting_measurement" {
		return fmt.Errorf("opened case state = %q", state)
	}

	read, err := h.request(http.MethodGet, "/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}

	second, err := h.request(http.MethodPost, "/cases", map[string]any{
		"device_id": deviceID,
		"nominal":   25.0,
		"tolerance": 0.5,
		"steward":   "Qiao",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(second, http.StatusConflict); err != nil {
		return err
	}
	return expectErrorCode(second, "active_case_exists")
}

func checkRecordMeasurement() error {
	h := newHarness()
	defer h.close()

	deviceID, err := h.createDevice("MET-1201")
	if err != nil {
		return err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return err
	}
	accepted, err := h.recordMeasurement(caseID, 25.2, "Rui")
	if err != nil {
		return err
	}
	if err := expectStatus(accepted, http.StatusCreated); err != nil {
		return err
	}
	state, err := nestedString(accepted.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "review_ready" {
		return fmt.Errorf("accepted measurement state = %q", state)
	}
	acceptedValue, err := nestedBool(accepted.Body, "measurements", 0, "accepted")
	if err != nil {
		return err
	}
	if !acceptedValue {
		return errors.New("in-tolerance measurement was not accepted")
	}

	second, err := h.recordMeasurement(caseID, 25.1, "Rui")
	if err != nil {
		return err
	}
	if err := expectStatus(second, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(second, "state_conflict"); err != nil {
		return err
	}

	rejectDeviceID, err := h.createDevice("MET-1202")
	if err != nil {
		return err
	}
	rejectCase, err := h.openCase(rejectDeviceID)
	if err != nil {
		return err
	}
	rejectCaseID, err := nestedString(rejectCase.Body, "case", "id")
	if err != nil {
		return err
	}
	rejected, err := h.recordMeasurement(rejectCaseID, 26.5, "Rui")
	if err != nil {
		return err
	}
	if err := expectStatus(rejected, http.StatusCreated); err != nil {
		return err
	}
	rejectedState, err := nestedString(rejected.Body, "case", "state")
	if err != nil {
		return err
	}
	if rejectedState != "recheck_required" {
		return fmt.Errorf("out-of-tolerance state = %q", rejectedState)
	}
	rejectedValue, err := nestedBool(rejected.Body, "measurements", 0, "accepted")
	if err != nil {
		return err
	}
	if rejectedValue {
		return errors.New("out-of-tolerance measurement was accepted")
	}
	return nil
}

func checkReopenVerificationCase() error {
	h := newHarness()
	defer h.close()

	caseID, err := h.rejectedCase("MET-1301")
	if err != nil {
		return err
	}
	reopened, err := h.request(http.MethodPost, "/cases/"+caseID+"/reopen", map[string]any{
		"actor":  "Lin",
		"reason": "measurement setup was replaced",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(reopened, http.StatusOK); err != nil {
		return err
	}
	state, err := nestedString(reopened.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "awaiting_measurement" {
		return fmt.Errorf("reopened case state = %q", state)
	}
	measurements, err := nestedSlice(reopened.Body, "measurements")
	if err != nil {
		return err
	}
	if len(measurements) != 1 {
		return fmt.Errorf("reopen changed measurement history: %d entries", len(measurements))
	}

	second, err := h.request(http.MethodPost, "/cases/"+caseID+"/reopen", map[string]any{
		"actor":  "Lin",
		"reason": "duplicate request",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(second, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(second, "state_conflict"); err != nil {
		return err
	}

	events, err := h.request(http.MethodGet, "/cases/"+caseID+"/events", nil)
	if err != nil {
		return err
	}
	if err := expectStatus(events, http.StatusOK); err != nil {
		return err
	}
	if !hasEvent(events.Body, "case_reopened") {
		return errors.New("reopen audit event is missing")
	}
	return nil
}

func checkSealVerification() error {
	h := newHarness()
	defer h.close()

	caseID, err := h.readyCase("MET-1401")
	if err != nil {
		return err
	}
	type outcome struct {
		response apiResponse
		err      error
	}
	results := make(chan outcome, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			response, err := h.request(http.MethodPost, "/cases/"+caseID+"/seal", map[string]any{
				"reviewer": fmt.Sprintf("Reviewer-%d", index+1),
				"summary":  "accepted measurement sealed for review",
			})
			results <- outcome{response: response, err: err}
		}(index)
	}
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for item := range results {
		if item.err != nil {
			return item.err
		}
		switch item.response.Status {
		case http.StatusCreated:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			return fmt.Errorf("unexpected concurrent seal status %d: %s", item.response.Status, item.response.Raw)
		}
	}
	if successes != 1 || conflicts != 1 {
		return fmt.Errorf("concurrent seal outcomes: %d success, %d conflict", successes, conflicts)
	}

	read, err := h.request(http.MethodGet, "/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	state, err := nestedString(read.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "sealed" {
		return fmt.Errorf("sealed case state = %q", state)
	}
	sealID, err := nestedString(read.Body, "seal", "id")
	if err != nil {
		return err
	}
	if sealID == "" {
		return errors.New("evidence seal is missing")
	}
	return nil
}

func checkRegisterCalibrationLab() error {
	h := newHarness()
	defer h.close()

	response, err := h.request(http.MethodPost, "/labs", map[string]any{
		"name":          "  Acme Metrology  ",
		"accreditation": "CNAS L0001",
		"steward":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return err
	}
	id, err := nestedString(response.Body, "lab", "id")
	if err != nil {
		return err
	}
	name, err := nestedString(response.Body, "lab", "name")
	if err != nil {
		return err
	}
	if name != "Acme Metrology" {
		return fmt.Errorf("lab name was not trimmed: %q", name)
	}
	status, err := nestedString(response.Body, "lab", "status")
	if err != nil {
		return err
	}
	if status != "active" {
		return fmt.Errorf("new lab status = %q", status)
	}

	read, err := h.request(http.MethodGet, "/labs/"+id, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	readID, err := nestedString(read.Body, "lab", "id")
	if err != nil {
		return err
	}
	if readID != id {
		return fmt.Errorf("read returned lab %q, expected %q", readID, id)
	}
	if !hasEvent(read.Body, "lab_registered") {
		return errors.New("lab_registered audit event is missing")
	}

	duplicate, err := h.request(http.MethodPost, "/labs", map[string]any{
		"name":          "ACME METROLOGY",
		"accreditation": "CNAS L9999",
		"steward":       "Mira",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(duplicate, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(duplicate, "duplicate_lab_name"); err != nil {
		return err
	}

	incomplete, err := h.request(http.MethodPost, "/labs", map[string]any{
		"name": "No Accreditation Lab",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(incomplete, http.StatusBadRequest); err != nil {
		return err
	}
	return expectErrorCode(incomplete, "invalid_input")
}

func checkMaintainCapabilityScope() error {
	h := newHarness()
	defer h.close()

	labID, err := h.createLab("Scope Lab")
	if err != nil {
		return err
	}
	scopeID, err := h.addScope(labID, "dc_voltage", 0, 100, "2020-01-01T00:00:00Z", "2030-01-01T00:00:00Z")
	if err != nil {
		return err
	}

	read, err := h.request(http.MethodGet, "/labs/"+labID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	scopes, err := nestedSlice(read.Body, "scopes")
	if err != nil {
		return err
	}
	if len(scopes) != 1 {
		return fmt.Errorf("lab has %d scopes, expected 1", len(scopes))
	}
	parameter, err := nestedStringPath(read.Body, "scopes", 0, "parameter")
	if err != nil {
		return err
	}
	if parameter != "dc_voltage" {
		return fmt.Errorf("scope parameter = %q", parameter)
	}

	badRange, err := h.request(http.MethodPost, "/labs/"+labID+"/scopes", map[string]any{
		"parameter":   "dc_voltage",
		"min_value":   50.0,
		"max_value":   10.0,
		"uncertainty": 0.01,
		"valid_from":  "2020-01-01T00:00:00Z",
		"valid_until": "2030-01-01T00:00:00Z",
		"actor":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(badRange, http.StatusBadRequest); err != nil {
		return err
	}
	if err := expectErrorCode(badRange, "invalid_input"); err != nil {
		return err
	}

	badWindow, err := h.request(http.MethodPost, "/labs/"+labID+"/scopes", map[string]any{
		"parameter":   "dc_voltage",
		"min_value":   0.0,
		"max_value":   100.0,
		"uncertainty": 0.01,
		"valid_from":  "2030-01-01T00:00:00Z",
		"valid_until": "2020-01-01T00:00:00Z",
		"actor":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(badWindow, http.StatusBadRequest); err != nil {
		return err
	}
	if err := expectErrorCode(badWindow, "invalid_input"); err != nil {
		return err
	}

	updated, err := h.request(http.MethodPut, "/labs/"+labID+"/scopes/"+scopeID, map[string]any{
		"parameter":   "dc_voltage",
		"min_value":   0.0,
		"max_value":   100.0,
		"uncertainty": 0.01,
		"valid_from":  "2020-01-01T00:00:00Z",
		"valid_until": "2031-06-30T00:00:00Z",
		"actor":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(updated, http.StatusOK); err != nil {
		return err
	}
	validUntil, err := nestedStringPath(updated.Body, "scopes", 0, "valid_until")
	if err != nil {
		return err
	}
	if validUntil != "2031-06-30T00:00:00Z" {
		return fmt.Errorf("updated scope valid_until = %q", validUntil)
	}
	if !hasEvent(updated.Body, "lab_scope_added") {
		return errors.New("lab_scope_added audit event is missing")
	}
	if !hasEvent(updated.Body, "lab_scope_updated") {
		return errors.New("lab_scope_updated audit event is missing")
	}

	otherLabID, err := h.createLab("Other Lab")
	if err != nil {
		return err
	}
	foreign, err := h.request(http.MethodPut, "/labs/"+otherLabID+"/scopes/"+scopeID, map[string]any{
		"parameter":   "dc_voltage",
		"min_value":   0.0,
		"max_value":   100.0,
		"uncertainty": 0.01,
		"valid_from":  "2020-01-01T00:00:00Z",
		"valid_until": "2031-06-30T00:00:00Z",
		"actor":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(foreign, http.StatusNotFound); err != nil {
		return err
	}
	return expectErrorCode(foreign, "not_found")
}

func checkRecordExternalMeasurement() error {
	h := newHarness()
	defer h.close()

	labID, err := h.createLab("Acme Metrology")
	if err != nil {
		return err
	}
	scopeID, err := h.addScope(labID, "dc_voltage", 0, 100, "2020-01-01T00:00:00Z", "2030-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	deviceID, err := h.createDevice("MET-1501")
	if err != nil {
		return err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return err
	}

	recorded, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", labID, scopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(recorded, http.StatusCreated); err != nil {
		return err
	}
	state, err := nestedString(recorded.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "review_ready" {
		return fmt.Errorf("external measurement state = %q", state)
	}
	labName, err := nestedStringPath(recorded.Body, "measurements", 0, "external_basis", "lab_name")
	if err != nil {
		return err
	}
	if labName != "Acme Metrology" {
		return fmt.Errorf("external basis lab name = %q", labName)
	}
	basisScope, err := nestedStringPath(recorded.Body, "measurements", 0, "external_basis", "scope_id")
	if err != nil {
		return err
	}
	if basisScope != scopeID {
		return fmt.Errorf("external basis scope = %q, expected %q", basisScope, scopeID)
	}
	accreditation, err := nestedStringPath(recorded.Body, "measurements", 0, "external_basis", "accreditation")
	if err != nil {
		return err
	}
	if accreditation != "CNAS L0001" {
		return fmt.Errorf("external basis accreditation = %q", accreditation)
	}
	referencedAt, err := nestedStringPath(recorded.Body, "measurements", 0, "external_basis", "referenced_at")
	if err != nil {
		return err
	}
	if referencedAt == "" {
		return errors.New("external basis reference time is missing")
	}
	if !eventDetailContains(recorded.Body, "measurement_accepted", scopeID) {
		return errors.New("measurement event does not explain the external basis source")
	}

	read, err := h.request(http.MethodGet, "/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	readScope, err := nestedStringPath(read.Body, "measurements", 0, "external_basis", "scope_id")
	if err != nil {
		return err
	}
	if readScope != scopeID {
		return fmt.Errorf("case detail external basis scope = %q, expected %q", readScope, scopeID)
	}
	if !eventDetailContains(read.Body, "measurement_accepted", "Acme Metrology") {
		return errors.New("case detail events do not explain the external basis source")
	}
	return nil
}

func checkRejectUnavailableExternalBasis() error {
	h := newHarness()
	defer h.close()

	deviceID, err := h.createDevice("MET-1601")
	if err != nil {
		return err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return err
	}

	expiredLabID, err := h.createLab("Expired Lab")
	if err != nil {
		return err
	}
	expiredScopeID, err := h.addScope(expiredLabID, "dc_voltage", 0, 100, "2020-01-01T00:00:00Z", "2021-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	expired, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", expiredLabID, expiredScopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(expired, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(expired, "capability_unavailable"); err != nil {
		return err
	}

	narrowLabID, err := h.createLab("Narrow Lab")
	if err != nil {
		return err
	}
	narrowScopeID, err := h.addScope(narrowLabID, "dc_voltage", 0, 10, "2020-01-01T00:00:00Z", "2030-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	mismatch, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", narrowLabID, narrowScopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(mismatch, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(mismatch, "scope_mismatch"); err != nil {
		return err
	}

	suspendedLabID, err := h.createLab("Suspended Lab")
	if err != nil {
		return err
	}
	suspendedScopeID, err := h.addScope(suspendedLabID, "dc_voltage", 0, 100, "2020-01-01T00:00:00Z", "2030-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	suspended, err := h.request(http.MethodPost, "/labs/"+suspendedLabID+"/suspend", map[string]any{
		"actor":  "Lin",
		"reason": "accreditation withdrawn by the accrediting body",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(suspended, http.StatusOK); err != nil {
		return err
	}
	suspendedAttempt, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", suspendedLabID, suspendedScopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(suspendedAttempt, http.StatusConflict); err != nil {
		return err
	}
	if err := expectErrorCode(suspendedAttempt, "capability_unavailable"); err != nil {
		return err
	}

	unknown, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", "lab-9999", "scope-9999")
	if err != nil {
		return err
	}
	if err := expectStatus(unknown, http.StatusNotFound); err != nil {
		return err
	}
	if err := expectErrorCode(unknown, "not_found"); err != nil {
		return err
	}

	read, err := h.request(http.MethodGet, "/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	state, err := nestedString(read.Body, "case", "state")
	if err != nil {
		return err
	}
	if state != "awaiting_measurement" {
		return fmt.Errorf("rejected references changed case state to %q", state)
	}
	measurementCount, err := nestedSliceLen(read.Body, "measurements")
	if err != nil {
		return err
	}
	if measurementCount != 0 {
		return fmt.Errorf("rejected references recorded %d measurements", measurementCount)
	}

	reinstated, err := h.request(http.MethodPost, "/labs/"+suspendedLabID+"/reinstate", map[string]any{
		"actor": "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(reinstated, http.StatusOK); err != nil {
		return err
	}
	accepted, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", suspendedLabID, suspendedScopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(accepted, http.StatusCreated); err != nil {
		return err
	}
	finalState, err := nestedString(accepted.Body, "case", "state")
	if err != nil {
		return err
	}
	if finalState != "review_ready" {
		return fmt.Errorf("reinstated lab measurement state = %q", finalState)
	}
	return nil
}

func checkPreserveExternalBasisHistory() error {
	h := newHarness()
	defer h.close()

	labID, err := h.createLab("History Lab")
	if err != nil {
		return err
	}
	scopeID, err := h.addScope(labID, "dc_voltage", 0, 100, "2020-01-01T00:00:00Z", "2030-01-01T00:00:00Z")
	if err != nil {
		return err
	}
	deviceID, err := h.createDevice("MET-1701")
	if err != nil {
		return err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return err
	}
	recorded, err := h.recordMeasurementWithBasis(caseID, 25.2, "Rui", labID, scopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(recorded, http.StatusCreated); err != nil {
		return err
	}
	sealed, err := h.request(http.MethodPost, "/cases/"+caseID+"/seal", map[string]any{
		"reviewer": "Wei",
		"summary":  "external calibration result sealed",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(sealed, http.StatusCreated); err != nil {
		return err
	}
	sealedScope, err := nestedStringPath(sealed.Body, "seal", "external_basis", "scope_id")
	if err != nil {
		return err
	}
	if sealedScope != scopeID {
		return fmt.Errorf("sealed basis scope = %q, expected %q", sealedScope, scopeID)
	}

	updated, err := h.request(http.MethodPut, "/labs/"+labID+"/scopes/"+scopeID, map[string]any{
		"parameter":   "dc_voltage",
		"min_value":   0.0,
		"max_value":   10.0,
		"uncertainty": 0.01,
		"valid_from":  "2020-01-01T00:00:00Z",
		"valid_until": "2021-01-01T00:00:00Z",
		"actor":       "Lin",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(updated, http.StatusOK); err != nil {
		return err
	}
	suspended, err := h.request(http.MethodPost, "/labs/"+labID+"/suspend", map[string]any{
		"actor":  "Lin",
		"reason": "accreditation lapsed",
	})
	if err != nil {
		return err
	}
	if err := expectStatus(suspended, http.StatusOK); err != nil {
		return err
	}

	read, err := h.request(http.MethodGet, "/cases/"+caseID, nil)
	if err != nil {
		return err
	}
	if err := expectStatus(read, http.StatusOK); err != nil {
		return err
	}
	maxValue, err := nestedNumber(read.Body, "measurements", 0, "external_basis", "max_value")
	if err != nil {
		return err
	}
	if maxValue != 100 {
		return fmt.Errorf("maintenance rewrote measurement basis max_value to %g", maxValue)
	}
	basisValidUntil, err := nestedStringPath(read.Body, "measurements", 0, "external_basis", "valid_until")
	if err != nil {
		return err
	}
	if basisValidUntil != "2030-01-01T00:00:00Z" {
		return fmt.Errorf("maintenance rewrote measurement basis valid_until to %q", basisValidUntil)
	}
	sealMaxValue, err := nestedNumber(read.Body, "seal", "external_basis", "max_value")
	if err != nil {
		return err
	}
	if sealMaxValue != 100 {
		return fmt.Errorf("maintenance rewrote sealed basis max_value to %g", sealMaxValue)
	}
	sealValidUntil, err := nestedStringPath(read.Body, "seal", "external_basis", "valid_until")
	if err != nil {
		return err
	}
	if sealValidUntil != "2030-01-01T00:00:00Z" {
		return fmt.Errorf("maintenance rewrote sealed basis valid_until to %q", sealValidUntil)
	}

	secondDeviceID, err := h.createDevice("MET-1702")
	if err != nil {
		return err
	}
	secondOpened, err := h.openCase(secondDeviceID)
	if err != nil {
		return err
	}
	secondCaseID, err := nestedString(secondOpened.Body, "case", "id")
	if err != nil {
		return err
	}
	attempt, err := h.recordMeasurementWithBasis(secondCaseID, 25.2, "Rui", labID, scopeID)
	if err != nil {
		return err
	}
	if err := expectStatus(attempt, http.StatusConflict); err != nil {
		return err
	}
	return expectErrorCode(attempt, "capability_unavailable")
}

func newHarness() *harness {
	server := httptest.NewServer(bootstrap.NewHandler())
	return &harness{
		baseURL: server.URL,
		client:  server.Client(),
		server:  server,
	}
}

func (h *harness) close() {
	h.server.Close()
}

func (h *harness) request(method, path string, body any) (apiResponse, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return apiResponse{}, err
		}
		reader = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, reader)
	if err != nil {
		return apiResponse{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := h.client.Do(request)
	if err != nil {
		return apiResponse{}, err
	}
	defer response.Body.Close()
	rawBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return apiResponse{}, err
	}
	result := apiResponse{Status: response.StatusCode, Raw: string(rawBytes)}
	if len(bytes.TrimSpace(rawBytes)) > 0 {
		if err := json.Unmarshal(rawBytes, &result.Body); err != nil {
			return apiResponse{}, fmt.Errorf("decode %s %s response: %w", method, path, err)
		}
	}
	return result, nil
}

func (h *harness) createDevice(tag string) (string, error) {
	response, err := h.request(http.MethodPost, "/devices", map[string]any{
		"asset_tag": tag,
		"model":     "Bench Meter",
		"room":      "Room A",
		"steward":   "Lin",
	})
	if err != nil {
		return "", err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return "", err
	}
	return nestedString(response.Body, "device", "id")
}

func (h *harness) openCase(deviceID string) (apiResponse, error) {
	response, err := h.request(http.MethodPost, "/cases", map[string]any{
		"device_id": deviceID,
		"nominal":   25.0,
		"tolerance": 0.5,
		"steward":   "Qiao",
	})
	if err != nil {
		return apiResponse{}, err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return apiResponse{}, err
	}
	return response, nil
}

func (h *harness) recordMeasurement(caseID string, observed float64, technician string) (apiResponse, error) {
	return h.request(http.MethodPost, "/cases/"+caseID+"/measurements", map[string]any{
		"observed":   observed,
		"technician": technician,
	})
}

func (h *harness) recordMeasurementWithBasis(caseID string, observed float64, technician, labID, scopeID string) (apiResponse, error) {
	return h.request(http.MethodPost, "/cases/"+caseID+"/measurements", map[string]any{
		"observed":   observed,
		"technician": technician,
		"external_basis": map[string]any{
			"lab_id":   labID,
			"scope_id": scopeID,
		},
	})
}

func (h *harness) createLab(name string) (string, error) {
	response, err := h.request(http.MethodPost, "/labs", map[string]any{
		"name":          name,
		"accreditation": "CNAS L0001",
		"steward":       "Lin",
	})
	if err != nil {
		return "", err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return "", err
	}
	return nestedString(response.Body, "lab", "id")
}

func (h *harness) addScope(labID, parameter string, minValue, maxValue float64, validFrom, validUntil string) (string, error) {
	response, err := h.request(http.MethodPost, "/labs/"+labID+"/scopes", map[string]any{
		"parameter":   parameter,
		"min_value":   minValue,
		"max_value":   maxValue,
		"uncertainty": 0.01,
		"valid_from":  validFrom,
		"valid_until": validUntil,
		"actor":       "Lin",
	})
	if err != nil {
		return "", err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return "", err
	}
	return nestedStringPath(response.Body, "scopes", 0, "id")
}

func (h *harness) rejectedCase(tag string) (string, error) {
	deviceID, err := h.createDevice(tag)
	if err != nil {
		return "", err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return "", err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return "", err
	}
	response, err := h.recordMeasurement(caseID, 26.5, "Rui")
	if err != nil {
		return "", err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return "", err
	}
	return caseID, nil
}

func (h *harness) readyCase(tag string) (string, error) {
	deviceID, err := h.createDevice(tag)
	if err != nil {
		return "", err
	}
	opened, err := h.openCase(deviceID)
	if err != nil {
		return "", err
	}
	caseID, err := nestedString(opened.Body, "case", "id")
	if err != nil {
		return "", err
	}
	response, err := h.recordMeasurement(caseID, 25.2, "Rui")
	if err != nil {
		return "", err
	}
	if err := expectStatus(response, http.StatusCreated); err != nil {
		return "", err
	}
	return caseID, nil
}

func expectStatus(response apiResponse, expected int) error {
	if response.Status != expected {
		return fmt.Errorf("status = %d, expected %d: %s", response.Status, expected, response.Raw)
	}
	return nil
}

func expectErrorCode(response apiResponse, expected string) error {
	code, err := nestedString(response.Body, "error", "code")
	if err != nil {
		return err
	}
	if code != expected {
		return fmt.Errorf("error code = %q, expected %q", code, expected)
	}
	return nil
}

func nestedString(root map[string]any, path ...string) (string, error) {
	value, err := nestedValue(root, path...)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s is not a string", strings.Join(path, "."))
	}
	return text, nil
}

func nestedStringPath(root map[string]any, path ...any) (string, error) {
	value, err := nestedAny(root, path...)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%v is not a string", path)
	}
	return text, nil
}

func nestedNumber(root map[string]any, path ...any) (float64, error) {
	value, err := nestedAny(root, path...)
	if err != nil {
		return 0, err
	}
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("%v is not a number", path)
	}
	return number, nil
}

func nestedBool(root map[string]any, path ...any) (bool, error) {
	value, err := nestedAny(root, path...)
	if err != nil {
		return false, err
	}
	flag, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%v is not a boolean", path)
	}
	return flag, nil
}

func nestedSlice(root map[string]any, key string) ([]any, error) {
	value, ok := root[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not an array", key)
	}
	return items, nil
}

// nestedSliceLen tolerates a null array, which the service emits for
// collections that have no entries yet.
func nestedSliceLen(root map[string]any, key string) (int, error) {
	value, ok := root[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	if value == nil {
		return 0, nil
	}
	items, ok := value.([]any)
	if !ok {
		return 0, fmt.Errorf("%s is not an array", key)
	}
	return len(items), nil
}

func hasEvent(root map[string]any, kind string) bool {
	items, err := nestedSlice(root, "events")
	if err != nil {
		return false
	}
	for _, item := range items {
		event, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if event["kind"] == kind {
			return true
		}
	}
	return false
}

func eventDetailContains(root map[string]any, kind, fragment string) bool {
	items, err := nestedSlice(root, "events")
	if err != nil {
		return false
	}
	for _, item := range items {
		event, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if event["kind"] != kind {
			continue
		}
		detail, ok := event["detail"].(string)
		if ok && strings.Contains(detail, fragment) {
			return true
		}
	}
	return false
}

func nestedValue(root map[string]any, path ...string) (any, error) {
	parts := make([]any, 0, len(path))
	for _, part := range path {
		parts = append(parts, part)
	}
	return nestedAny(root, parts...)
}

func nestedAny(root map[string]any, path ...any) (any, error) {
	var current any = root
	for _, part := range path {
		switch typed := part.(type) {
		case string:
			object, ok := current.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s is not an object", typed)
			}
			value, ok := object[typed]
			if !ok {
				return nil, fmt.Errorf("missing %s", typed)
			}
			current = value
		case int:
			items, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("index %d is not inside an array", typed)
			}
			if typed < 0 || typed >= len(items) {
				return nil, fmt.Errorf("index %d is out of range", typed)
			}
			current = items[typed]
		default:
			return nil, fmt.Errorf("unsupported path element %T", part)
		}
	}
	return current, nil
}
