--1.检查是你的锁吗？
--2.是就续期
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("EXPIRE", KEYS[1] ,ARGV[2])
end
return 0