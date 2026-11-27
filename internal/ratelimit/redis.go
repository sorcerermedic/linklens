package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var tokenBucketScript = redis.NewScript(`
local rate  = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])
local ttl   = tonumber(ARGV[4])

local state  = redis.call('HMGET', KEYS[1], 't', 'l')
local tokens = tonumber(state[1])
local last   = tonumber(state[2])
if tokens == nil or last == nil then
  tokens = burst
  last = now
end

local elapsed = (now - last) / 1000
if elapsed > 0 then
  tokens = tokens + elapsed * rate
end
if tokens > burst then
  tokens = burst
end

local allowed = 0
local retry = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  retry = math.ceil((1 - tokens) / rate * 1000)
end

redis.call('HSET', KEYS[1], 't', tostring(tokens), 'l', tostring(now))
redis.call('PEXPIRE', KEYS[1], ttl)
return {allowed, math.floor(tokens), retry}
`)

type Redis struct {
	client redis.UniversalClient
	cfg    Config
	now    Clock
	prefix string
}

func NewRedis(client redis.UniversalClient, cfg Config, clock Clock) *Redis {
	if clock == nil {
		clock = time.Now
	}
	return &Redis{client: client, cfg: cfg, now: clock, prefix: "ratelimit:"}
}

func (r *Redis) Allow(ctx context.Context, key string) (Decision, error) {

	ttl := time.Duration(float64(r.cfg.Burst)/r.cfg.Rate*float64(time.Second)) + time.Second
	res, err := tokenBucketScript.Run(ctx, r.client, []string{r.prefix + key},
		r.cfg.Rate, r.cfg.Burst, r.now().UnixMilli(), ttl.Milliseconds()).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: redis: %w", err)
	}
	if len(res) != 3 {
		return Decision{}, fmt.Errorf("ratelimit: unexpected script result %v", res)
	}
	return Decision{
		Allowed:    res[0] == 1,
		Remaining:  int(res[1]),
		RetryAfter: time.Duration(res[2]) * time.Millisecond,
	}, nil
}
