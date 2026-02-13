// Package queue wraps Kafka access: topic administration, producers, and
// consumers, plus the message types exchanged on the wire.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
)

// Topic names — a fixed contract between server and worker.
const (
	TopicRuns      = "runs"
	TopicRunEvents = "run-events"
)

// Run-event types carried on TopicRunEvents.
const (
	EventStatus = "status" // status transition (e.g. queued -> running)
	EventChunk  = "chunk"  // a piece of live program output
	EventDone   = "done"   // final verdict
)

// RunMessage is produced by the API server when a run is created. It carries
// only the id: the worker loads the run from Postgres.
type RunMessage struct {
	RunID string `json:"run_id"`
}

// RunEvent is produced by the worker while a run executes and is forwarded
// verbatim to the owning user's websockets.
//   - type=status: Status only.
//   - type=chunk:  Stream ("stdout"|"stderr") + Data, emitted live.
//   - type=done:   final Status + ExitCode + RuntimeMS + Error.
type RunEvent struct {
	RunID     string `json:"run_id"`
	UserID    int64  `json:"user_id"`
	Type      string `json:"type"`
	Status    string `json:"status,omitempty"`
	Stream    string `json:"stream,omitempty"`
	Data      string `json:"data,omitempty"`
	ExitCode  int    `json:"exit_code"`
	RuntimeMS int    `json:"runtime_ms"`
	Error     string `json:"error,omitempty"`
}

// EnsureTopics creates the given topics (1 partition, RF 1) if they do not
// already exist.
func EnsureTopics(ctx context.Context, brokers []string, topics ...string) error {
	if len(brokers) == 0 {
		return errors.New("no kafka brokers configured")
	}

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("dial kafka: %w", err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get controller: %w", err)
	}
	ctrlConn, err := kafka.DialContext(ctx, "tcp",
		net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("dial controller: %w", err)
	}
	defer ctrlConn.Close()

	configs := make([]kafka.TopicConfig, 0, len(topics))
	for _, t := range topics {
		configs = append(configs, kafka.TopicConfig{
			Topic:             t,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	if err := ctrlConn.CreateTopics(configs...); err != nil &&
		!errors.Is(err, kafka.TopicAlreadyExists) {
		return fmt.Errorf("create topics: %w", err)
	}
	return nil
}

// EnsureTopicsWithRetry keeps trying EnsureTopics with backoff until maxWait
// elapses, so binaries can boot before Kafka is reachable.
func EnsureTopicsWithRetry(ctx context.Context, brokers []string, maxWait time.Duration, topics ...string) error {
	deadline := time.Now().Add(maxWait)
	backoff := time.Second
	for {
		err := EnsureTopics(ctx, brokers, topics...)
		if err == nil {
			slog.Info("kafka topics ready", "topics", topics)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("kafka not reachable after %s: %w", maxWait, err)
		}
		slog.Warn("kafka not ready, retrying", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

// NewWriter returns a producer for one topic. Writers connect lazily, so this
// never fails even when Kafka is down.
func NewWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           50 * time.Millisecond, // low latency for single messages
	}
}

// NewReader returns a consumer-group reader for one topic.
func NewReader(brokers []string, topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // explicit commits
	})
}

// Publish marshals v to JSON and writes it with the given key.
func Publish(ctx context.Context, w *kafka.Writer, key string, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	return w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: payload})
}
