package cache

import (
	"context"
	_ "embed"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	client redis.Cmdable
}

var (
	ErrFailedToPreemtLock = errors.New("redis-lock: failed to acquire lock")
	ErrLockNotExist       = errors.New("redis-lock: lock does not exist")
)

var (
	//go:embed lua/unlock.lua
	luaUnlock string
	//go:embed lua/refresh.lua
	luaRefresh string
)

func NewClient(client redis.Cmdable) *Client {
	return &Client{client: client}
}
func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()

	ok, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrFailedToPreemtLock
	}

	return &Lock{
		client:     c.client,
		key:        key,
		val:        val,
		expiration: expiration,
	}, nil
}

// Unlock removes the lock only when it still belongs to this Lock instance.
func (l *Lock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.val).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotExist
	}
	return nil
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(
		ctx,
		luaRefresh,
		[]string{l.key},
		l.val,
		int64(l.expiration.Seconds()),
	).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotExist
	}
	return nil
}

type Lock struct {
	client     redis.Cmdable
	key        string
	val        string
	expiration time.Duration
}
