package effect

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/vmihailenco/msgpack/v5"
)

// ServiceObservationContractV1 is the provider-neutral authenticated
// observation contract. A guest selects only an opaque host-configured
// definition. Endpoints, credentials, headers and transport requests are
// intentionally absent.
const ServiceObservationContractV1 = "service-observation.v1"

type ServiceObservationStatusV1 string

const (
	ServiceObservationOperationalV1 ServiceObservationStatusV1 = "operational"
	ServiceObservationDegradedV1    ServiceObservationStatusV1 = "degraded"
	ServiceObservationMajorV1       ServiceObservationStatusV1 = "major"
	ServiceObservationMaintenanceV1 ServiceObservationStatusV1 = "maintenance"
)

type ServiceObservationEvidenceCodeV1 string

const (
	ServiceObservationEvidenceAuthenticatedV1 ServiceObservationEvidenceCodeV1 = "authenticated"
	ServiceObservationEvidenceUnavailableV1   ServiceObservationEvidenceCodeV1 = "unavailable"
	ServiceObservationEvidenceUnauthorizedV1  ServiceObservationEvidenceCodeV1 = "unauthorized"
	ServiceObservationEvidenceRateLimitedV1   ServiceObservationEvidenceCodeV1 = "rate_limited"
	ServiceObservationEvidenceInvalidV1       ServiceObservationEvidenceCodeV1 = "invalid_response"
	ServiceObservationEvidenceTimeoutV1       ServiceObservationEvidenceCodeV1 = "timeout"
	ServiceObservationEvidenceMaintenanceV1   ServiceObservationEvidenceCodeV1 = "maintenance"
)

// ServiceObservationFenceV1 binds one host read to the exact durable lease
// granted by the state owner.
type ServiceObservationFenceV1 struct {
	LeaseID        string `msgpack:"lease_id"`
	Attempt        uint32 `msgpack:"attempt"`
	LeaseExpiresAt string `msgpack:"lease_expires_at"`
}

// ServiceObservationCommandV1 is the complete guest-controlled surface. The
// service definition is an opaque host registry key, not a provider, URL or
// credential. CommandID and IdempotencyKey must equal the enclosing Intent.
type ServiceObservationCommandV1 struct {
	Contract            string                    `msgpack:"contract"`
	ServiceDefinitionID string                    `msgpack:"service_definition_id"`
	CommandID           string                    `msgpack:"command_id"`
	IdempotencyKey      string                    `msgpack:"idempotency_key"`
	Fence               ServiceObservationFenceV1 `msgpack:"fence"`
	ObservedAt          string                    `msgpack:"observed_at"`
}

// ServiceObservationValueV1 is bounded provider-neutral evidence. Provider
// response bodies, request URLs, headers and diagnostics never cross this
// boundary.
type ServiceObservationValueV1 struct {
	Status     ServiceObservationStatusV1       `msgpack:"status"`
	Message    string                           `msgpack:"message,omitempty"`
	HTTPStatus uint16                           `msgpack:"http_status,omitempty"`
	Evidence   ServiceObservationEvidenceCodeV1 `msgpack:"evidence"`
	ObservedAt string                           `msgpack:"observed_at"`
}

// ServiceObservationResultV1 is the typed result stored inside a completed
// pulp.effect.v1 Receipt.
type ServiceObservationResultV1 struct {
	Contract            string                    `msgpack:"contract"`
	ServiceDefinitionID string                    `msgpack:"service_definition_id"`
	CommandID           string                    `msgpack:"command_id"`
	IdempotencyKey      string                    `msgpack:"idempotency_key"`
	Fence               ServiceObservationFenceV1 `msgpack:"fence"`
	Observation         ServiceObservationValueV1 `msgpack:"observation"`
}

func (value ServiceObservationFenceV1) Validate() error {
	if validateServiceObservationToken(value.LeaseID, 256) != nil || value.Attempt == 0 {
		return fmt.Errorf("service observation fence is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, value.LeaseExpiresAt); err != nil {
		return fmt.Errorf("service observation lease_expires_at must be RFC3339")
	}
	return nil
}

func (value ServiceObservationCommandV1) Validate() error {
	if value.Contract != ServiceObservationContractV1 ||
		validateServiceObservationToken(value.ServiceDefinitionID, 256) != nil ||
		validateServiceObservationToken(value.CommandID, 256) != nil ||
		validateServiceObservationToken(value.IdempotencyKey, 256) != nil {
		return fmt.Errorf("service observation command identity is invalid")
	}
	if err := value.Fence.Validate(); err != nil {
		return err
	}
	observedAt, err := time.Parse(time.RFC3339Nano, value.ObservedAt)
	if err != nil {
		return fmt.Errorf("service observation observed_at must be RFC3339")
	}
	expiresAt, _ := time.Parse(time.RFC3339Nano, value.Fence.LeaseExpiresAt)
	if !observedAt.Before(expiresAt) {
		return fmt.Errorf("service observation observed_at is outside its lease")
	}
	return nil
}

func (value ServiceObservationValueV1) Validate() error {
	switch value.Status {
	case ServiceObservationOperationalV1, ServiceObservationDegradedV1,
		ServiceObservationMajorV1, ServiceObservationMaintenanceV1:
	default:
		return fmt.Errorf("service observation status is not allowlisted")
	}
	switch value.Evidence {
	case ServiceObservationEvidenceAuthenticatedV1, ServiceObservationEvidenceUnavailableV1,
		ServiceObservationEvidenceUnauthorizedV1, ServiceObservationEvidenceRateLimitedV1,
		ServiceObservationEvidenceInvalidV1, ServiceObservationEvidenceTimeoutV1,
		ServiceObservationEvidenceMaintenanceV1:
	default:
		return fmt.Errorf("service observation evidence is not allowlisted")
	}
	if (value.Status == ServiceObservationOperationalV1) !=
		(value.Evidence == ServiceObservationEvidenceAuthenticatedV1) {
		return fmt.Errorf("service observation operational status requires authenticated evidence")
	}
	if (value.Status == ServiceObservationMaintenanceV1) !=
		(value.Evidence == ServiceObservationEvidenceMaintenanceV1) {
		return fmt.Errorf("service observation maintenance status requires maintenance evidence")
	}
	if value.Message != "" && validateServiceObservationText(value.Message, 512) != nil {
		return fmt.Errorf("service observation message is invalid")
	}
	if value.HTTPStatus != 0 && (value.HTTPStatus < 100 || value.HTTPStatus > 599) {
		return fmt.Errorf("service observation HTTP status is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
		return fmt.Errorf("service observation value observed_at must be RFC3339")
	}
	return nil
}

func (value ServiceObservationResultV1) Validate() error {
	command := ServiceObservationCommandV1{
		Contract: value.Contract, ServiceDefinitionID: value.ServiceDefinitionID,
		CommandID: value.CommandID, IdempotencyKey: value.IdempotencyKey,
		Fence: value.Fence, ObservedAt: value.Observation.ObservedAt,
	}
	if err := command.Validate(); err != nil {
		return err
	}
	return value.Observation.Validate()
}

func (value ServiceObservationResultV1) ValidateFor(command ServiceObservationCommandV1) error {
	if err := command.Validate(); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return err
	}
	if value.Contract != command.Contract ||
		value.ServiceDefinitionID != command.ServiceDefinitionID ||
		value.CommandID != command.CommandID ||
		value.IdempotencyKey != command.IdempotencyKey ||
		value.Fence != command.Fence ||
		value.Observation.ObservedAt != command.ObservedAt {
		return fmt.Errorf("service observation result does not match command")
	}
	return nil
}

func decodeServiceObservationCommandV1(raw msgpack.RawMessage) (ServiceObservationCommandV1, error) {
	var value ServiceObservationCommandV1
	if err := decodeServiceObservationStrict(raw, &value); err != nil {
		return value, fmt.Errorf("decode service observation command: %w", err)
	}
	return value, nil
}

func decodeServiceObservationResultV1(raw msgpack.RawMessage) (ServiceObservationResultV1, error) {
	var value ServiceObservationResultV1
	if err := decodeServiceObservationStrict(raw, &value); err != nil {
		return value, fmt.Errorf("decode service observation result: %w", err)
	}
	return value, nil
}

func decodeServiceObservationStrict(raw []byte, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty MessagePack value")
	}
	reader := bytes.NewReader(raw)
	decoder := msgpack.NewDecoder(reader)
	decoder.DisallowUnknownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if reader.Len() != 0 {
		return fmt.Errorf("trailing MessagePack data")
	}
	return nil
}

func validateServiceObservationToken(value string, limit int) error {
	if value == "" || len(value) > limit || strings.TrimSpace(value) != value ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("bounded token is required")
	}
	return nil
}

func validateServiceObservationText(value string, limit int) error {
	if value == "" || len(value) > limit || strings.TrimSpace(value) != value ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("bounded text is required")
	}
	return nil
}
