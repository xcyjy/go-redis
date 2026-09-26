package cache

import (
	"context"
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
	"github.com/redis/go-redis/v9"
	"github.com/xcyjy/go-redis/cache/mocks"
	"go.uber.org/mock/gomock"
)

func TestClientTryLock(t *testing.T) {
	expiration := time.Minute

	testsCase := []struct {
		name     string
		mock     func(ctrl *gomock.Controller) redis.Cmdable
		key      string
		wantErr  error
		wantLock *Lock
	}{
		{
			name: "set nx error",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(false, context.DeadlineExceeded)

				cmd.EXPECT().
					SetNX(context.Background(), "key1", gomock.Any(), expiration).
					Return(res)

				return cmd
			},
			key:     "key1",
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "lock already exists",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(false, nil)

				cmd.EXPECT().
					SetNX(context.Background(), "key1", gomock.Any(), expiration).
					Return(res)

				return cmd
			},
			key:     "key1",
			wantErr: ErrFailedToPreemtLock,
		},
		{
			name: "success",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolResult(true, nil)

				cmd.EXPECT().
					SetNX(context.Background(), "key1", gomock.Any(), expiration).
					Return(res)

				return cmd
			},
			key: "key1",
			wantLock: &Lock{
				key:        "key1",
				expiration: expiration,
			},
		},
	}

	for _, tc := range testsCase {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := NewClient(tc.mock(ctrl))

			lock, err := client.TryLock(
				context.Background(),
				tc.key,
				expiration,
			)
			assert.Equal(t, tc.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tc.wantLock.key, lock.key)
			assert.Equal(t, tc.wantLock.expiration, lock.expiration)
			assert.NotEqual(t, "", lock.val)
		})
	}
}

func TestLockUnlock(t *testing.T) {
	testsCase := []struct {
		name    string
		mock    func(ctrl *gomock.Controller) redis.Cmdable
		key     string
		val     string
		wantErr error
	}{
		{
			name: "unlock success",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmdResult(int64(1), nil)

				cmd.EXPECT().
					Eval(context.Background(), luaUnlock, []string{"key1"}, "lock-value").
					Return(res)

				return cmd
			},
			key: "key1",
			val: "lock-value",
		},
		{
			name: "lock does not exist",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmdResult(int64(0), nil)

				cmd.EXPECT().
					Eval(context.Background(), luaUnlock, []string{"key1"}, "lock-value").
					Return(res)

				return cmd
			},
			key:     "key1",
			val:     "lock-value",
			wantErr: ErrLockNotExist,
		},
		{
			name: "eval error",
			mock: func(ctrl *gomock.Controller) redis.Cmdable {
				cmd := mocks.NewMockCmdable(ctrl)
				res := redis.NewCmdResult(nil, context.DeadlineExceeded)

				cmd.EXPECT().
					Eval(context.Background(), luaUnlock, []string{"key1"}, "lock-value").
					Return(res)

				return cmd
			},
			key:     "key1",
			val:     "lock-value",
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tc := range testsCase {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := tc.mock(ctrl)

			lock := &Lock{
				client: client,
				key:    tc.key,
				val:    tc.val,
			}

			err := lock.Unlock(context.Background())
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
