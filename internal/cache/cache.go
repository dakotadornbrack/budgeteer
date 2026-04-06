package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/dakotadornbrack/budgeteer/internal/model"
	"github.com/dakotadornbrack/budgeteer/internal/store"
)

const summaryTTL = 5 * time.Minute

// CachedStore wraps a Store and caches expensive summary queries in Redis.
// All other operations pass through directly to the underlying store.
type CachedStore struct {
	store.Store
	rdb *redis.Client
}

func NewCachedStore(s store.Store, rdb *redis.Client) *CachedStore {
	return &CachedStore{Store: s, rdb: rdb}
}

func summaryKey(month, year int) string {
	return fmt.Sprintf("summary:%d:%d", year, month)
}

// GetSummary returns a cached summary if available, otherwise fetches from
// the database, stores the result in Redis, and returns it.
func (c *CachedStore) GetSummary(month, year int) ([]*model.Summary, error) {
	ctx := context.Background()
	key := summaryKey(month, year)

	// Cache hit
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == nil {
		var summaries []*model.Summary
		if jsonErr := json.Unmarshal(data, &summaries); jsonErr == nil {
			return summaries, nil
		}
	}

	if !errors.Is(err, redis.Nil) {
		// Redis is unavailable — log and fall through to DB rather than failing hard.
		fmt.Printf("redis get error (falling through): %v\n", err)
	}

	// Cache miss: fetch from database
	summaries, err := c.Store.GetSummary(month, year)
	if err != nil {
		return nil, err
	}

	// Populate cache — best effort, don't fail the request on cache write error
	if encoded, encErr := json.Marshal(summaries); encErr == nil {
		_ = c.rdb.Set(ctx, key, encoded, summaryTTL).Err()
	}

	return summaries, nil
}

// InvalidateSummary removes a cached summary after a write operation.
// Called whenever a transaction or budget is created/deleted.
func (c *CachedStore) InvalidateSummary(month, year int) {
	ctx := context.Background()
	_ = c.rdb.Del(ctx, summaryKey(month, year)).Err()
}
