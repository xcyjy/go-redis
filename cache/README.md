# cache 测试说明

这个目录实现了一个简单的 Redis 分布式锁：

- `TryLock`：尝试加锁
- `Unlock`：释放锁
- `redis_lock_test.go`：测试加锁逻辑

## 如何运行测试

在项目根目录 `D:\go-redis` 执行：

```powershell
go test ./cache
```

查看每个测试用例的详细名称：

```powershell
go test ./cache -v
```

只运行 `TryLock` 测试：

```powershell
go test ./cache -run TestClientTryLock -v
```

## 测试函数的基本格式

Go 测试函数必须以 `Test` 开头，并接收一个 `*testing.T` 参数：

```go
func TestClientTryLock(t *testing.T) {
	// 测试代码
}
```

`t *testing.T` 是 Go 测试框架提供的对象，可以用来创建子测试、报告错误和终止测试。

## `testsCase` 是什么

下面的代码定义了一个测试用例数组：

```go
testsCase := []struct {
	name     string
	mock     func(ctrl *gomock.Controller) redis.Cmdable
	key      string
	wantErr  error
	wantLock *Lock
}{}
```

这里的 `struct` 可以理解为一个自定义的数据类型。每个字段表示测试需要准备的信息：

| 字段 | 含义 |
| --- | --- |
| `name` | 测试用例名称 |
| `mock` | 创建 Redis Mock 对象，并配置 Redis 应该返回什么 |
| `key` | 本次测试使用的 Redis key |
| `wantErr` | 期望出现的错误 |
| `wantLock` | 期望成功返回的锁 |

`testsCase` 里面的每一个 `{ ... }` 就是一条测试用例。

例如：

```go
{
	name:     "lock already exists",
	key:      "key1",
	wantErr:  ErrFailedToPreemtLock,
	wantLock: nil,
},
```

表示：Redis 中已经存在这个 key，所以加锁失败，并且应该返回
`ErrFailedToPreemtLock`。

## `mock` 字段是什么意思

```go
mock: func(ctrl *gomock.Controller) redis.Cmdable {
	cmd := mocks.NewMockCmdable(ctrl)
	return cmd
},
```

这里的 `mock` 不是一个已经创建好的对象，而是一个函数。

它的作用是：每个测试用例运行时，重新创建一个 Redis 模拟对象。

参数：

```go
ctrl *gomock.Controller
```

`ctrl` 是 GoMock 的控制器，用来管理 Mock 的调用和校验。

返回值：

```go
redis.Cmdable
```

`Client` 依赖的就是 `redis.Cmdable`，所以 Mock 也只需要实现这个接口。

## `NewMockCmdable(ctrl)` 是什么

```go
cmd := mocks.NewMockCmdable(ctrl)
```

这个函数来自自动生成的文件：

```text
cache/mocks/mock_redis_cmdable_gen.go
```

它创建了一个假的 Redis 客户端。测试时不会真的连接 Redis，而是由我们提前规定 Redis 应该如何返回。

## `EXPECT().SetNX().Return()` 的含义

```go
cmd.EXPECT().
	SetNX(context.Background(), "key1", gomock.Any(), expiration).
	Return(res)
```

这段代码可以拆开理解：

```go
cmd.EXPECT()
```

表示：下面要设置一个预期调用。

```go
SetNX(context.Background(), "key1", gomock.Any(), expiration)
```

表示：测试代码应该调用：

```go
SetNX(ctx, "key1", 某个值, expiration)
```

四个参数分别是：

1. `context.Background()`：上下文
2. `"key1"`：Redis key
3. `gomock.Any()`：任意值
4. `expiration`：锁的过期时间

这里使用 `gomock.Any()` 是因为锁的 value 是 UUID，每次运行都会随机生成，无法写死：

```go
val := uuid.New().String()
```

```go
.Return(res)
```

表示 Redis 执行 `SetNX` 后返回 `res`。

## `redis.NewBoolResult` 是什么

```go
res := redis.NewBoolResult(false, context.DeadlineExceeded)
```

Redis 的 `SetNX` 返回一个布尔值和一个错误：

```text
是否加锁成功，执行过程中是否出错
```

上面的代码表示：

```text
加锁结果：false
错误：context.DeadlineExceeded
```

成功情况：

```go
redis.NewBoolResult(true, nil)
```

Redis 没有报错，并且成功加锁。

锁已存在的情况：

```go
redis.NewBoolResult(false, nil)
```

Redis 没有报错，但 `SetNX` 返回 `false`，说明 key 已经存在。

## `t.Run` 是什么

```go
for _, tc := range testsCase {
	t.Run(tc.name, func(t *testing.T) {
		// 执行当前测试用例
	})
}
```

这段代码会遍历所有测试用例，并为每一个用例创建一个子测试。

例如，最终会看到：

```text
TestClientTryLock/set_nx_error
TestClientTryLock/lock_already_exists
TestClientTryLock/success
```

## `gomock.NewController` 和 `defer`

```go
ctrl := gomock.NewController(t)
defer ctrl.Finish()
```

`NewController` 创建 GoMock 控制器。

`defer ctrl.Finish()` 表示当前子测试结束时，检查所有预期的 Mock 调用是否真的发生。

例如我们设置了：

```go
cmd.EXPECT().SetNX(...).Return(res)
```

如果 `TryLock` 没有调用 `SetNX`，测试就会失败。

## `NewClient` 和真正的测试调用

```go
client := NewClient(tc.mock(ctrl))

lock, err := client.TryLock(
	context.Background(),
	tc.key,
	expiration,
)
```

第一行把假的 Redis 客户端传给 `Client`。

第二行调用真正的业务代码 `TryLock`。

测试不会直接修改 `TryLock` 的实现，而是通过 Mock 控制 Redis 的返回结果。

## `assert.Equal` 是什么

```go
assert.Equal(t, tc.wantErr, err)
```

这表示：

```text
期望值：tc.wantErr
实际值：err
```

如果两者不相等，测试失败。

成功时还会检查：

```go
assert.Equal(t, tc.wantLock.key, lock.key)
assert.Equal(t, tc.wantLock.expiration, lock.expiration)
```

UUID 不能这样比较：

```go
assert.Equal(t, tc.wantLock, lock)
```

因为 `lock.val` 是运行时随机生成的 UUID，每次都不同。

因此当前测试只检查：

```go
assert.NotEqual(t, "", lock.val)
```

也就是确认 UUID 不为空。

## 三个测试用例

### 1. Redis 发生错误

```go
redis.NewBoolResult(false, context.DeadlineExceeded)
```

预期：

```text
TryLock 返回 context.DeadlineExceeded
lock 返回 nil
```

### 2. 锁已经存在

```go
redis.NewBoolResult(false, nil)
```

预期：

```text
TryLock 返回 ErrFailedToPreemtLock
lock 返回 nil
```

### 3. 成功加锁

```go
redis.NewBoolResult(true, nil)
```

预期：

```text
TryLock 不返回错误
返回一个 Lock
Lock 的 key 和 expiration 正确
Lock 的 val 是非空 UUID
```

## 从测试中记住这条主线

每个测试用例都按照下面的顺序执行：

```text
1. 创建 Mock Redis
2. 规定 Mock Redis 收到什么调用
3. 规定 Mock Redis 返回什么结果
4. 调用真正的 TryLock
5. 用 assert 检查结果
```

