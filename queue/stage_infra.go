package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// Stage requests contain only the compact versioned descriptor from
	// stage_contracts.go. This broker limit is defense in depth against raw XML
	// or another unbounded payload being published to a request queue.
	stageRequestMaxMessageSize int32 = 16 << 10
	// JSON can expand one valid UTF-8 source byte to six ASCII bytes (for
	// example NUL becomes \u0000). The conservative calculation includes all
	// bounded issue/diagnostic/version text, a maximum layer per issue, and
	// 64 KiB of structural/identity overhead.
	maxStageJSONByteExpansion            = 6
	stageRequestWorstCaseSerializedBytes = maxStageJSONByteExpansion*(2*MaxStageVersionBytes) + MaxStagePlanIDBytes + MaxStageIDBytes + 128 + 4<<10
	stageResultWorstCaseSerializedBytes  = maxStageJSONByteExpansion*(MaxStageIssueTextBytes+MaxStageDiagnosticBytes+2*MaxStageVersionBytes) +
		MaxStageIssues*(MaxStageIDBytes+128) + 64<<10
	stageResultMaxMessageSize     int32 = 2 << 20
	stageDeadLetterMaxMessageSize int32 = 32 << 10
	stageConsumerMaxDeliver             = -1
)

// Compile-time proof that each configured stream ceiling covers its
// conservative V1 serialized-message bound.
var (
	_ [int(stageRequestMaxMessageSize) - stageRequestWorstCaseSerializedBytes]byte
	_ [int(stageResultMaxMessageSize) - stageResultWorstCaseSerializedBytes]byte
)

// StageDeliveryConfig controls one durable pull consumer independently from
// every other stage and direction.
type StageDeliveryConfig struct {
	AckWait       time.Duration
	MaxAckPending int
}

// StageQueueConfig defines retention and delivery limits for one stage's
// request, result, and operational dead-letter queues. A zero MaxAge retains
// work without a time limit; positive values are explicit storage policies.
type StageQueueConfig struct {
	Stage            ValidationLayer
	RequestMaxAge    time.Duration
	ResultMaxAge     time.Duration
	DeadLetterMaxAge time.Duration
	RequestDelivery  StageDeliveryConfig
	ResultDelivery   StageDeliveryConfig
}

// StageQueueInfraConfig is the deployer-owned desired topology. Stages are a
// slice rather than a shared global map so callers can review and version the
// exact resources being provisioned.
type StageQueueInfraConfig struct {
	Stages []StageQueueConfig
}

// DefaultStageQueueInfraConfig reserves independent XSD, EN 16931 and future
// Peppol queues. The future queue reservation does not activate Peppol in any
// validation plan. All queues retain unacknowledged work indefinitely by
// default; capacity tuning remains independently configurable.
func DefaultStageQueueInfraConfig() StageQueueInfraConfig {
	const (
		defaultAckWait       = 60 * time.Second
		defaultMaxAckPending = 64
	)
	stages := []ValidationLayer{
		ValidationLayerUBL21XSD,
		ValidationLayerEN16931,
		ValidationLayerPeppolBISBilling,
	}
	config := StageQueueInfraConfig{Stages: make([]StageQueueConfig, 0, len(stages))}
	for _, stage := range stages {
		config.Stages = append(config.Stages, StageQueueConfig{
			Stage:           stage,
			RequestDelivery: StageDeliveryConfig{AckWait: defaultAckWait, MaxAckPending: defaultMaxAckPending},
			ResultDelivery:  StageDeliveryConfig{AckWait: defaultAckWait, MaxAckPending: defaultMaxAckPending},
		})
	}
	return config
}

// ProvisionStageQueueInfra creates or updates only the new stage-specific
// resources. It never reads, updates, replaces or deletes the legacy document
// stream, so it is safe to run before the split-runtime migration barrier.
func (c *Client) ProvisionStageQueueInfra(ctx context.Context, config StageQueueInfraConfig) error {
	if err := validateStageQueueInfraConfig(config); err != nil {
		return err
	}
	for _, stage := range config.Stages {
		names, _ := c.Namespace.StageQueue(stage.Stage)
		if _, err := c.JS.CreateOrUpdateStream(ctx, stageRequestStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("provision %s request stream: %w", stage.Stage, err)
		}
		if _, err := c.JS.CreateOrUpdateConsumer(ctx, names.RequestStream, stageRequestConsumerConfig(names, stage)); err != nil {
			return fmt.Errorf("provision %s request consumer: %w", stage.Stage, err)
		}
		if _, err := c.JS.CreateOrUpdateStream(ctx, stageResultStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("provision %s result stream: %w", stage.Stage, err)
		}
		if _, err := c.JS.CreateOrUpdateConsumer(ctx, names.ResultStream, stageResultConsumerConfig(names, stage)); err != nil {
			return fmt.Errorf("provision %s result consumer: %w", stage.Stage, err)
		}
		if _, err := c.JS.CreateOrUpdateStream(ctx, stageDeadLetterStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("provision %s dead-letter stream: %w", stage.Stage, err)
		}
	}
	return nil
}

// VerifyStageQueueInfra performs read-only compatibility checks. Runtime
// services use this instead of provisioning or mutating broker definitions.
func (c *Client) VerifyStageQueueInfra(ctx context.Context, config StageQueueInfraConfig) error {
	if err := validateStageQueueInfraConfig(config); err != nil {
		return err
	}
	for _, stage := range config.Stages {
		names, _ := c.Namespace.StageQueue(stage.Stage)
		if err := c.verifyStageStream(ctx, stageRequestStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("verify %s request stream: %w", stage.Stage, err)
		}
		if err := c.verifyStageConsumer(ctx, names.RequestStream, stageRequestConsumerConfig(names, stage)); err != nil {
			return fmt.Errorf("verify %s request consumer: %w", stage.Stage, err)
		}
		if err := c.verifyStageStream(ctx, stageResultStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("verify %s result stream: %w", stage.Stage, err)
		}
		if err := c.verifyStageConsumer(ctx, names.ResultStream, stageResultConsumerConfig(names, stage)); err != nil {
			return fmt.Errorf("verify %s result consumer: %w", stage.Stage, err)
		}
		if err := c.verifyStageStream(ctx, stageDeadLetterStreamConfig(names, stage)); err != nil {
			return fmt.Errorf("verify %s dead-letter stream: %w", stage.Stage, err)
		}
	}
	return nil
}

// VerifyStageWorkerInfra performs read-only verification of the resources one
// stage worker uses or depends on: its request stream and durable consumer,
// plus the result stream it publishes to. Retention and delivery capacity are
// provisioner choices, so this method validates the fixed wire topology and
// returns the request consumer's actual delivery settings to the worker. It
// deliberately does not open or validate the orchestrator-owned result
// consumer.
func (c *Client) VerifyStageWorkerInfra(ctx context.Context, stage ValidationLayer) (StageDeliveryConfig, error) {
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return StageDeliveryConfig{}, err
	}
	requestExpected := stageRequestStreamConfig(names, StageQueueConfig{Stage: stage})
	if err := c.verifyStageWorkerStream(ctx, requestExpected); err != nil {
		return StageDeliveryConfig{}, fmt.Errorf("verify %s request stream: %w", stage, err)
	}
	resultExpected := stageResultStreamConfig(names, StageQueueConfig{Stage: stage})
	if err := c.verifyStageWorkerStream(ctx, resultExpected); err != nil {
		return StageDeliveryConfig{}, fmt.Errorf("verify %s result stream: %w", stage, err)
	}
	deadLetterExpected := stageDeadLetterStreamConfig(names, StageQueueConfig{Stage: stage})
	if err := c.verifyStageWorkerStream(ctx, deadLetterExpected); err != nil {
		return StageDeliveryConfig{}, fmt.Errorf("verify %s dead-letter stream: %w", stage, err)
	}
	delivery, err := c.verifyStageWorkerRequestConsumer(ctx, stage, names)
	if err != nil {
		return StageDeliveryConfig{}, fmt.Errorf("verify %s request consumer: %w", stage, err)
	}
	return delivery, nil
}

// StageRequestConsumer opens the already-provisioned durable worker consumer.
// It cannot create or update broker infrastructure.
func (c *Client) StageRequestConsumer(ctx context.Context, stage ValidationLayer) (jetstream.Consumer, error) {
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return nil, err
	}
	consumer, err := c.JS.Consumer(ctx, names.RequestStream, names.RequestConsumer)
	if err != nil {
		return nil, fmt.Errorf("open %s request consumer: %w", stage, err)
	}
	return consumer, nil
}

// StageResultConsumer opens the already-provisioned durable orchestrator
// consumer. It cannot create or update broker infrastructure.
func (c *Client) StageResultConsumer(ctx context.Context, stage ValidationLayer) (jetstream.Consumer, error) {
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return nil, err
	}
	consumer, err := c.JS.Consumer(ctx, names.ResultStream, names.ResultConsumer)
	if err != nil {
		return nil, fmt.Errorf("open %s result consumer: %w", stage, err)
	}
	return consumer, nil
}

// PublishStageRequest publishes one orchestrator-authorized attempt to the
// exact stage queue using its deterministic request ID for broker deduplication.
func (c *Client) PublishStageRequest(ctx context.Context, request StageRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	names, err := c.Namespace.StageQueue(request.Stage)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal stage request %s: %w", request.RequestID, err)
	}
	message := nats.NewMsg(names.RequestSubject)
	message.Data = payload
	message.Header.Set(nats.MsgIdHdr, request.RequestID)
	if _, err := c.JS.PublishMsg(ctx, message); err != nil {
		return fmt.Errorf("publish stage request %s: %w", request.RequestID, err)
	}
	return nil
}

// PublishStageResult publishes immutable worker evidence to the exact result
// queue using its deterministic result ID for broker deduplication.
func (c *Client) PublishStageResult(ctx context.Context, result StageResult) error {
	if err := result.Validate(); err != nil {
		return err
	}
	names, err := c.Namespace.StageQueue(result.Stage)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal stage result %s: %w", result.ResultID, err)
	}
	message := nats.NewMsg(names.ResultSubject)
	message.Data = payload
	message.Header.Set(nats.MsgIdHdr, result.ResultID)
	if _, err := c.JS.PublishMsg(ctx, message); err != nil {
		return fmt.Errorf("publish stage result %s: %w", result.ResultID, err)
	}
	return nil
}

func validateStageQueueInfraConfig(config StageQueueInfraConfig) error {
	if len(config.Stages) == 0 {
		return errors.New("stage queue infrastructure must contain at least one stage")
	}
	seen := make(map[ValidationLayer]struct{}, len(config.Stages))
	for index, stage := range config.Stages {
		if _, err := DefaultNamespace.StageQueue(stage.Stage); err != nil {
			return fmt.Errorf("stage queue config %d: %w", index, err)
		}
		if _, duplicate := seen[stage.Stage]; duplicate {
			return fmt.Errorf("stage queue config duplicates stage %q", stage.Stage)
		}
		seen[stage.Stage] = struct{}{}
		if stage.RequestMaxAge < 0 || stage.ResultMaxAge < 0 || stage.DeadLetterMaxAge < 0 {
			return fmt.Errorf("stage queue %q max age must be non-negative", stage.Stage)
		}
		if err := validateStageDeliveryConfig("request", stage.Stage, stage.RequestDelivery); err != nil {
			return err
		}
		if err := validateStageDeliveryConfig("result", stage.Stage, stage.ResultDelivery); err != nil {
			return err
		}
	}
	return nil
}

func validateStageDeliveryConfig(direction string, stage ValidationLayer, config StageDeliveryConfig) error {
	if config.AckWait <= 0 {
		return fmt.Errorf("stage queue %q %s AckWait must be positive", stage, direction)
	}
	if config.MaxAckPending <= 0 {
		return fmt.Errorf("stage queue %q %s MaxAckPending must be positive", stage, direction)
	}
	return nil
}

func stageRequestStreamConfig(names StageQueueNames, config StageQueueConfig) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name: names.RequestStream, Description: "Durable compact validation stage requests for " + string(config.Stage) + ".",
		Subjects: []string{names.RequestSubject}, Retention: jetstream.WorkQueuePolicy,
		Storage: jetstream.FileStorage, MaxAge: config.RequestMaxAge, MaxMsgSize: stageRequestMaxMessageSize,
	}
}

func stageResultStreamConfig(names StageQueueNames, config StageQueueConfig) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name: names.ResultStream, Description: "Durable validation stage attempt results for " + string(config.Stage) + ".",
		Subjects: []string{names.ResultSubject}, Retention: jetstream.WorkQueuePolicy,
		Storage: jetstream.FileStorage, MaxAge: config.ResultMaxAge, MaxMsgSize: stageResultMaxMessageSize,
	}
}

func stageDeadLetterStreamConfig(names StageQueueNames, config StageQueueConfig) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name: names.DeadLetterStream, Description: "Permanent exhausted validation stage attempts for " + string(config.Stage) + ".",
		Subjects: []string{names.DeadLetterFilterSubject}, Retention: jetstream.LimitsPolicy,
		Storage: jetstream.FileStorage, MaxAge: config.DeadLetterMaxAge, MaxMsgSize: stageDeadLetterMaxMessageSize,
		Discard: jetstream.DiscardNew, DiscardNewPerSubject: true, MaxMsgsPerSubject: 1,
	}
}

func stageRequestConsumerConfig(names StageQueueNames, config StageQueueConfig) jetstream.ConsumerConfig {
	return stageConsumerConfig(names.RequestConsumer, names.RequestSubject, "Stage worker pool for "+string(config.Stage)+" requests.", config.RequestDelivery)
}

func stageResultConsumerConfig(names StageQueueNames, config StageQueueConfig) jetstream.ConsumerConfig {
	return stageConsumerConfig(names.ResultConsumer, names.ResultSubject, "Validation orchestrator for "+string(config.Stage)+" results.", config.ResultDelivery)
}

func stageConsumerConfig(durable, subject, description string, config StageDeliveryConfig) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable: durable, Description: description, FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy, AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: config.AckWait, MaxDeliver: stageConsumerMaxDeliver,
		MaxAckPending: config.MaxAckPending, ReplayPolicy: jetstream.ReplayInstantPolicy,
	}
}

func (c *Client) verifyStageStream(ctx context.Context, expected jetstream.StreamConfig) error {
	stream, err := c.JS.Stream(ctx, expected.Name)
	if err != nil {
		return err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return err
	}
	actual := info.Config
	limitsMismatch := expected.DiscardNewPerSubject && (actual.Discard != expected.Discard || !actual.DiscardNewPerSubject || actual.MaxMsgsPerSubject != expected.MaxMsgsPerSubject)
	if !reflect.DeepEqual(actual.Subjects, expected.Subjects) || actual.Retention != expected.Retention || actual.Storage != expected.Storage || actual.MaxAge != expected.MaxAge || actual.MaxMsgSize != expected.MaxMsgSize || limitsMismatch {
		return fmt.Errorf("incompatible stream %s: subjects=%v retention=%v storage=%v maxAge=%s maxMsgSize=%d", actual.Name, actual.Subjects, actual.Retention, actual.Storage, actual.MaxAge, actual.MaxMsgSize)
	}
	return nil
}

func (c *Client) verifyStageWorkerStream(ctx context.Context, expected jetstream.StreamConfig) error {
	stream, err := c.JS.Stream(ctx, expected.Name)
	if err != nil {
		return err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return err
	}
	actual := info.Config
	limitsMismatch := expected.DiscardNewPerSubject && (actual.Discard != expected.Discard || !actual.DiscardNewPerSubject || actual.MaxMsgsPerSubject != expected.MaxMsgsPerSubject)
	if !reflect.DeepEqual(actual.Subjects, expected.Subjects) || actual.Retention != expected.Retention || actual.Storage != expected.Storage || actual.MaxMsgSize != expected.MaxMsgSize || limitsMismatch {
		return fmt.Errorf("incompatible stream %s: subjects=%v retention=%v storage=%v maxAge=%s maxMsgSize=%d", actual.Name, actual.Subjects, actual.Retention, actual.Storage, actual.MaxAge, actual.MaxMsgSize)
	}
	return nil
}

func (c *Client) verifyStageWorkerRequestConsumer(ctx context.Context, stage ValidationLayer, names StageQueueNames) (StageDeliveryConfig, error) {
	consumer, err := c.JS.Consumer(ctx, names.RequestStream, names.RequestConsumer)
	if err != nil {
		return StageDeliveryConfig{}, err
	}
	info, err := consumer.Info(ctx)
	if err != nil {
		return StageDeliveryConfig{}, err
	}
	actual := info.Config
	delivery := StageDeliveryConfig{AckWait: actual.AckWait, MaxAckPending: actual.MaxAckPending}
	if err := validateStageDeliveryConfig("request", stage, delivery); err != nil {
		return StageDeliveryConfig{}, err
	}
	expected := stageRequestConsumerConfig(names, StageQueueConfig{Stage: stage, RequestDelivery: delivery})
	if actual.Durable != expected.Durable || actual.FilterSubject != expected.FilterSubject || actual.DeliverPolicy != expected.DeliverPolicy || actual.AckPolicy != expected.AckPolicy || actual.MaxDeliver != expected.MaxDeliver || actual.ReplayPolicy != expected.ReplayPolicy {
		return StageDeliveryConfig{}, fmt.Errorf("incompatible consumer %s on %s: filter=%q AckWait=%s MaxDeliver=%d MaxAckPending=%d", actual.Durable, names.RequestStream, actual.FilterSubject, actual.AckWait, actual.MaxDeliver, actual.MaxAckPending)
	}
	return delivery, nil
}

func (c *Client) verifyStageConsumer(ctx context.Context, stream string, expected jetstream.ConsumerConfig) error {
	consumer, err := c.JS.Consumer(ctx, stream, expected.Durable)
	if err != nil {
		return err
	}
	info, err := consumer.Info(ctx)
	if err != nil {
		return err
	}
	actual := info.Config
	if actual.Durable != expected.Durable || actual.FilterSubject != expected.FilterSubject || actual.DeliverPolicy != expected.DeliverPolicy || actual.AckPolicy != expected.AckPolicy || actual.AckWait != expected.AckWait || actual.MaxDeliver != expected.MaxDeliver || actual.MaxAckPending != expected.MaxAckPending || actual.ReplayPolicy != expected.ReplayPolicy {
		return fmt.Errorf("incompatible consumer %s on %s: filter=%q AckWait=%s MaxDeliver=%d MaxAckPending=%d", actual.Durable, stream, actual.FilterSubject, actual.AckWait, actual.MaxDeliver, actual.MaxAckPending)
	}
	return nil
}
