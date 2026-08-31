package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	DeploymentGenerationSchemaVersion uint32 = 1
	deploymentGenerationKey                  = "active"
	deploymentGenerationMaxValueSize  int32  = 4 << 10
)

// DeploymentMode is the durable runtime compatibility barrier. Transitions
// are forward-only except that an unactivated drain may return to legacy.
type DeploymentMode string

const (
	DeploymentModeLegacy   DeploymentMode = "legacy"
	DeploymentModeDraining DeploymentMode = "draining"
	DeploymentModeSplit    DeploymentMode = "split"
)

var (
	ErrDeploymentGenerationNotProvisioned = errors.New("deployment generation infrastructure is not provisioned")
	ErrDeploymentModeConflict             = errors.New("deployment mode transition conflict")
	ErrDeploymentDraining                 = errors.New("deployment is draining legacy validation work")
)

// DeploymentGeneration records one namespace's active processing generation.
// Generation increments on every transition so operators can correlate a
// rollout with the exact durable barrier it observed.
type DeploymentGeneration struct {
	SchemaVersion uint32         `json:"schemaVersion"`
	Generation    uint64         `json:"generation"`
	Mode          DeploymentMode `json:"mode"`
	UpdatedAt     string         `json:"updatedAt"`
}

// VersionedDeploymentGeneration carries the KV revision used for CAS.
type VersionedDeploymentGeneration struct {
	State    DeploymentGeneration
	Revision uint64
}

func (state DeploymentGeneration) Validate() error {
	if state.SchemaVersion != DeploymentGenerationSchemaVersion {
		return fmt.Errorf("unsupported deployment generation schema version %d", state.SchemaVersion)
	}
	if state.Generation == 0 {
		return errors.New("deployment generation must be at least 1")
	}
	switch state.Mode {
	case DeploymentModeLegacy, DeploymentModeDraining, DeploymentModeSplit:
	default:
		return fmt.Errorf("unsupported deployment mode %q", state.Mode)
	}
	if _, err := time.Parse(time.RFC3339Nano, state.UpdatedAt); err != nil {
		return fmt.Errorf("invalid deployment generation updatedAt: %w", err)
	}
	return nil
}

// ProvisionDeploymentGenerationInfra creates the barrier bucket and its
// initial legacy marker. Repeated provisioning preserves the active marker.
func (c *Client) ProvisionDeploymentGenerationInfra(ctx context.Context, now time.Time) (VersionedDeploymentGeneration, error) {
	bucketName := c.Namespace.DeploymentGenerationBucketName()
	kv, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: bucketName, Description: "Durable legacy-to-split deployment generation barrier.",
		Storage: jetstream.FileStorage, MaxValueSize: deploymentGenerationMaxValueSize,
	})
	if err != nil {
		return VersionedDeploymentGeneration{}, fmt.Errorf("provision %s KV bucket: %w", bucketName, err)
	}
	c.handlesMu.Lock()
	c.cachedDeploymentGenerationKV = kv
	c.handlesMu.Unlock()
	initial := DeploymentGeneration{
		SchemaVersion: DeploymentGenerationSchemaVersion, Generation: 1,
		Mode: DeploymentModeLegacy, UpdatedAt: now.UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(initial)
	if err != nil {
		return VersionedDeploymentGeneration{}, err
	}
	revision, err := kv.Create(ctx, deploymentGenerationKey, payload)
	if err == nil {
		return VersionedDeploymentGeneration{State: initial, Revision: revision}, nil
	}
	if !errors.Is(err, jetstream.ErrKeyExists) {
		return VersionedDeploymentGeneration{}, fmt.Errorf("initialize deployment generation: %w", err)
	}
	return c.DeploymentGeneration(ctx)
}

// DeploymentGeneration loads the exact durable marker. Older installations
// without the new bucket are reported distinctly so legacy runtimes can stay
// backward compatible while split runtimes fail closed.
func (c *Client) DeploymentGeneration(ctx context.Context) (VersionedDeploymentGeneration, error) {
	kv, err := c.deploymentGenerationBucket(ctx)
	if err != nil {
		return VersionedDeploymentGeneration{}, err
	}
	entry, err := kv.Get(ctx, deploymentGenerationKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return VersionedDeploymentGeneration{}, ErrDeploymentGenerationNotProvisioned
		}
		return VersionedDeploymentGeneration{}, fmt.Errorf("read deployment generation: %w", err)
	}
	var state DeploymentGeneration
	if err := json.Unmarshal(entry.Value(), &state); err != nil {
		return VersionedDeploymentGeneration{}, fmt.Errorf("decode deployment generation: %w", err)
	}
	if err := state.Validate(); err != nil {
		return VersionedDeploymentGeneration{}, err
	}
	return VersionedDeploymentGeneration{State: state, Revision: entry.Revision()}, nil
}

// TransitionDeploymentMode applies one legal CAS-protected state change.
func (c *Client) TransitionDeploymentMode(ctx context.Context, current VersionedDeploymentGeneration, nextMode DeploymentMode, now time.Time) (VersionedDeploymentGeneration, error) {
	if err := current.State.Validate(); err != nil || current.Revision == 0 {
		return VersionedDeploymentGeneration{}, errors.New("invalid current deployment generation")
	}
	legal := current.State.Mode == DeploymentModeLegacy && nextMode == DeploymentModeDraining ||
		current.State.Mode == DeploymentModeDraining && (nextMode == DeploymentModeLegacy || nextMode == DeploymentModeSplit)
	if !legal {
		return VersionedDeploymentGeneration{}, fmt.Errorf("illegal deployment mode transition %q -> %q", current.State.Mode, nextMode)
	}
	next := DeploymentGeneration{
		SchemaVersion: DeploymentGenerationSchemaVersion, Generation: current.State.Generation + 1,
		Mode: nextMode, UpdatedAt: now.UTC().Format(time.RFC3339Nano),
	}
	if next.Generation == 0 {
		return VersionedDeploymentGeneration{}, errors.New("deployment generation overflow")
	}
	payload, err := json.Marshal(next)
	if err != nil {
		return VersionedDeploymentGeneration{}, err
	}
	kv, err := c.deploymentGenerationBucket(ctx)
	if err != nil {
		return VersionedDeploymentGeneration{}, err
	}
	revision, err := kv.Update(ctx, deploymentGenerationKey, payload, current.Revision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return VersionedDeploymentGeneration{}, fmt.Errorf("%w: %v", ErrDeploymentModeConflict, err)
		}
		return VersionedDeploymentGeneration{}, fmt.Errorf("update deployment generation: %w", err)
	}
	return VersionedDeploymentGeneration{State: next, Revision: revision}, nil
}

// RequireSplitDeployment prevents split runtimes from consuming before the
// migration command has completed reconciliation and activated the barrier.
func (c *Client) RequireSplitDeployment(ctx context.Context) error {
	current, err := c.DeploymentGeneration(ctx)
	if err != nil {
		return fmt.Errorf("split runtime requires an activated deployment generation: %w", err)
	}
	if current.State.Mode != DeploymentModeSplit {
		return fmt.Errorf("split runtime requires deployment mode %q, found %q", DeploymentModeSplit, current.State.Mode)
	}
	return nil
}

// RequireLegacyDeployment rejects a combined worker after split activation.
// A missing marker remains legacy-compatible for pre-migration installations.
func (c *Client) RequireLegacyDeployment(ctx context.Context) error {
	current, err := c.DeploymentGeneration(ctx)
	if errors.Is(err, ErrDeploymentGenerationNotProvisioned) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.State.Mode == DeploymentModeSplit {
		return errors.New("legacy combined worker is disabled after split deployment activation")
	}
	return nil
}

func (c *Client) deploymentGenerationBucket(ctx context.Context) (jetstream.KeyValue, error) {
	c.handlesMu.RLock()
	cached := c.cachedDeploymentGenerationKV
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	c.handlesMu.Lock()
	defer c.handlesMu.Unlock()
	if c.cachedDeploymentGenerationKV != nil {
		return c.cachedDeploymentGenerationKV, nil
	}
	bucketName := c.Namespace.DeploymentGenerationBucketName()
	kv, err := c.JS.KeyValue(ctx, bucketName)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			return nil, ErrDeploymentGenerationNotProvisioned
		}
		return nil, fmt.Errorf("open %s KV bucket: %w", bucketName, err)
	}
	c.cachedDeploymentGenerationKV = kv
	return kv, nil
}
