package reaper

import (
"context"
"testing"
"time"

"github.com/redis/go-redis/v9"
)

func setupTestRedisClient() *redis.Client {
return redis.NewClient(&redis.Options{
Addr: "localhost:6379",
DB:   15,
})
}

func TestStreamReaper_Creation(t *testing.T) {
client := setupTestRedisClient()
defer client.Close()

reaper := NewStreamReaper(
client,
"test:stream",
"test_group",
"test_consumer",
30*time.Second,
5,
10*time.Second,
)

if reaper == nil {
t.Fatal("Reaper should not be nil")
}
if reaper.streamName != "test:stream" {
t.Errorf("Expected stream name test:stream, got %s", reaper.streamName)
}
if reaper.groupName != "test_group" {
t.Errorf("Expected group name test_group, got %s", reaper.groupName)
}
}

func TestStreamReaper_GetStats(t *testing.T) {
client := setupTestRedisClient()
defer client.Close()

ctx := context.Background()
reaper := NewStreamReaper(
client,
"test:stream:stats",
"test_group",
"test_consumer",
30*time.Second,
5,
10*time.Second,
)

// Cria um stream de teste
err := client.XAdd(ctx, &redis.XAddArgs{
Stream: "test:stream:stats",
ID:     "*",
Values: map[string]interface{}{"data": "test"},
}).Err()
if err != nil {
t.Skipf("Redis not available: %v", err)
}

// Cria consumer group
err = client.XGroupCreateMkStream(ctx, "test:stream:stats", "test_group", "0").Err()
if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
t.Logf("Warning creating group: %v", err)
}

stats, err := reaper.GetStats(ctx)
if err != nil {
t.Fatalf("GetStats failed: %v", err)
}

if stats == nil {
t.Error("Stats should not be nil")
}
}
