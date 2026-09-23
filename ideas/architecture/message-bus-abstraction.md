# Message Bus Abstraction Layer

## Objective

Introduce a small messaging abstraction layer so e-ubl21 can switch between different message brokers without coupling business logic to a specific implementation.

Initial targets:

- Kafka
- RabbitMQ
- NATS / JetStream as a possible future adapter
- In-memory implementation for tests

The abstraction should cover the common messaging operations required by e-ubl21 while deliberately avoiding an attempt to normalize every broker-specific feature.

## Difficulty

The implementation is moderate rather than difficult if the interface remains narrow.

A basic production-quality abstraction supporting publish, consume, acknowledgements, retries, headers, serialization and basic routing should take roughly 2 to 4 engineering days including tests.

Trying to expose advanced broker-specific concepts through the same generic interface can turn this into a 1 to 3 week subsystem and should be avoided unless there is a concrete requirement.

## Core Design Principle

Abstract the application's messaging requirements, not the products themselves.

Kafka and RabbitMQ use different underlying models:

- Kafka is primarily a durable distributed log with partitions, offsets, replay and consumer groups.
- RabbitMQ is primarily a message broker centered around exchanges, queues, routing keys and acknowledgements.

The abstraction should therefore expose only the common behaviour e-ubl21 actually needs.

## Proposed Go API

```go
type Message struct {
    Key     string
    Payload []byte
    Headers map[string]string
}

type Publisher interface {
    Publish(ctx context.Context, destination string, msg Message) error
}

type Handler func(ctx context.Context, msg Message) error

type Consumer interface {
    Subscribe(ctx context.Context, destination string, handler Handler) error
}

type MessageBus interface {
    Publisher
    Consumer
    Close() error
}
```

The application services depend only on `MessageBus` and never directly import a Kafka, RabbitMQ or NATS client package.

## Suggested Package Layout

```text
internal/messaging/
    messaging.go

    kafka/
        kafka.go

    rabbitmq/
        rabbitmq.go

    nats/
        nats.go

    memory/
        memory.go
```

The in-memory implementation is particularly useful for unit and integration testing because most tests can run without provisioning a real broker.

## Runtime Configuration

The selected transport should be chosen through configuration rather than compilation.

Example:

```yaml
messaging:
  provider: kafka

  kafka:
    brokers:
      - kafka-1:9092
    topic_prefix: ubl21

  rabbitmq:
    url: amqp://rabbitmq:5672
    exchange: ubl21
```

Application startup can use a factory:

```go
provider, err := messaging.New(cfg.Messaging)
```

The rest of the application remains unchanged when the provider changes.

## Common Feature Set

The first version should support only capabilities common enough to be portable:

- publish a message
- consume a message
- acknowledge successful processing
- retry failed processing
- message headers / metadata
- serialization
- basic destination routing
- graceful shutdown

## Features That Should Remain Broker-Specific

The generic interface should not attempt to directly expose:

- Kafka partitions
- Kafka offsets
- Kafka replay controls
- Kafka consumer-group internals
- RabbitMQ exchanges
- RabbitMQ binding rules
- RabbitMQ priority queues
- broker-specific transaction APIs
- broker-specific exactly-once facilities
- delayed-message implementations

If e-ubl21 later needs one of these capabilities, it should be introduced through a provider-specific extension rather than expanding the core abstraction into a universal messaging API.

## Naming

The abstraction should not be called `Queue`, because Kafka topics are not queues in the RabbitMQ sense.

Preferred name:

`MessageBus`

Other acceptable names would be `Messaging`, `EventBus`, `Transport` or `Broker`.

## Architecture

```text
                  e-ubl21 service
                        |
                        v
                 MessageBus API
                        |
          +-------------+-------------+
          |             |             |
          v             v             v
        Kafka       RabbitMQ       NATS/JetStream
       Adapter       Adapter         Adapter
```

For testing:

```text
                  e-ubl21 service
                        |
                        v
                 MessageBus API
                        |
                        v
                  Memory Adapter
```

## Deployment Implications

A transport abstraction would give e-ubl21 more flexibility across deployment models.

| Deployment model | Likely messaging option |
| --- | --- |
| Lightweight local installation | RabbitMQ or NATS |
| Enterprise Kubernetes | Kafka |
| AWS managed deployment | Amazon MSK or Amazon MQ |
| Azure deployment | Event Hubs or Service Bus adapter |
| Small single-server installation | NATS JetStream or RabbitMQ |

The abstraction also prevents the messaging technology selected for one deployment target from becoming an architectural lock-in for the entire application.

## Recommendation

Implement the MessageBus abstraction early, but keep it intentionally small.

The strategic value is high because it:

- prevents broker-specific code from leaking into business services
- makes Kafka and RabbitMQ interchangeable at the application boundary
- enables lightweight deployments without Kafka
- makes unit and integration testing simpler
- creates a clean extension point for NATS, Azure Service Bus or other transports later

The key constraint is to resist building a universal messaging API. Broker-specific concepts should stay inside adapters unless e-ubl21 has a concrete requirement that justifies exposing them.
