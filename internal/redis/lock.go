package redisclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLockNotAcquired = errors.New("slot lock not acquired")
)

// Locker is used by the appointment service to guard critical sections per slot
type Locker interface {
	WithSlotLock(ctx context.Context, slotID uuid.UUID, fn func(ctx context.Context) error) error
}

type redisSlotLocker struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisSlotLocker creates a locker that uses a per slot Redis key
func NewRedisSlotLocker(client *redis.Client, ttl time.Duration) Locker {
	return &redisSlotLocker{
		client: client,
		ttl:    ttl,
	}
}

func (l *redisSlotLocker) WithSlotLock(ctx context.Context, slotID uuid.UUID, fn func(ctx context.Context) error) error {
	key := fmt.Sprintf("lock:slot:%s", slotID.String())
	token := uuid.NewString()

	ok, err := l.client.SetNX(ctx, key, token, l.ttl).Result()
	if err != nil {
		return fmt.Errorf("acquire slot lock: %w", err)
	}
	if !ok {
		return ErrLockNotAcquired
	}

	defer func() {
		_ = l.release(ctx, key, token)
	}()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, l.ttl)
	defer cancel()

	return fn(ctxWithTimeout)
}

var unlockScript = redis.NewScript(`
local val = redis.call("GET", KEYS[1])
if val == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

func (l *redisSlotLocker) release(ctx context.Context, key, token string) error {
	_, err := unlockScript.Run(ctx, l.client, []string{key}, token).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("release slot lock: %w", err)
	}
	return nil
}
