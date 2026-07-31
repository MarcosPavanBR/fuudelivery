package queue

import (
	"testing"
	"time"
)

func TestNew_ReturnsSingleton(t *testing.T) {
	q1 := New()
	q2 := New()
	if q1 != q2 {
		t.Fatal("Expected New() to return the same singleton instance")
	}
}

func TestPublishSubscribe_GoChannels(t *testing.T) {
	// Force Go channels fallback (no REDIS_URL set)
	q := New()

	received := make(chan []byte, 1)
	q.Subscribe("test-queue", func(msg []byte) {
		received <- msg
	})

	// Give subscriber time to start
	time.Sleep(50 * time.Millisecond)

	err := q.Publish("test-queue", []byte(`{"test":"data"}`))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case msg := <-received:
		if string(msg) != `{"test":"data"}` {
			t.Fatalf("Expected '{\"test\":\"data\"}', got '%s'", string(msg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestPublishJSON_GoChannels(t *testing.T) {
	q := New()

	type TestMsg struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	received := make(chan []byte, 1)
	q.Subscribe("test-json", func(msg []byte) {
		received <- msg
	})

	time.Sleep(50 * time.Millisecond)

	err := q.PublishJSON("test-json", TestMsg{ID: "123", Name: "test"})
	if err != nil {
		t.Fatalf("PublishJSON failed: %v", err)
	}

	select {
	case msg := <-received:
		if string(msg) != `{"id":"123","name":"test"}` {
			t.Fatalf("Unexpected message: %s", string(msg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestIsRedis_FalseWithoutConfig(t *testing.T) {
	q := New()
	if q.IsRedis() {
		t.Fatal("Expected IsRedis() to be false without REDIS_URL")
	}
}
