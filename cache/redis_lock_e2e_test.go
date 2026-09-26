//go:build e2e

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientTryLockE2E(t *testing.T) {
	rdb := newE2ERedisClient(t)
	client := NewClient(rdb)

	testsCase := []struct {
		name       string
		before     func(t *testing.T)
		after      func(t *testing.T)
		key        string
		expiration time.Duration
		wantErr    error
		wantLock   bool
	}{
		{
			name: "key exists",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				ok, err := rdb.SetNX(ctx, "key1", "other-lock-value", time.Minute).Result()
				require.NoError(t, err)
				assert.True(t, ok)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				res, err := rdb.GetDel(ctx, "key1").Result()
				require.NoError(t, err)
				assert.Equal(t, "other-lock-value", res)
			},
			key:        "key1",
			expiration: time.Minute,
			wantErr:    ErrFailedToPreemtLock,
			wantLock:   false,
		},
		{
			name: "lock success",
			before: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				res, err := rdb.GetDel(ctx, "key2").Result()
				require.NoError(t, err)
				assert.Empty(t, res)
			},
			after: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()

				_, err := rdb.GetDel(ctx, "key2").Result()
				require.NoError(t, err)
			},
			key:        "key2",
			expiration: time.Minute,
			wantLock:   true,
		},
	}

	for _, tc := range testsCase {
		t.Run(tc.name, func(t *testing.T) {
			if tc.before != nil {
				tc.before(t)
			}
			if tc.after != nil {
				defer tc.after(t)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			lock, err := client.TryLock(ctx, tc.key, tc.expiration)
			assert.Equal(t, tc.wantErr, err)

			if !tc.wantLock {
				assert.Nil(t, lock)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, lock)
			assert.Equal(t, tc.key, lock.key)
			assert.Equal(t, tc.expiration, lock.expiration)
			assert.NotEmpty(t, lock.val)

			err = lock.Unlock(ctx)
			assert.NoError(t, err)
		})
	}
}

func newE2ERedisClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := "127.0.0.1:6379"
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, rdb.Ping(ctx).Err())

	t.Cleanup(func() {
		_ = rdb.Close()
	})

	return rdb
}
