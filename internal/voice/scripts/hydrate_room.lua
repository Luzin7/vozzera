local function open_session(key, ms, mode)
  local current = redis.call('GET', key)
  if not current or (mode == 'exact' and (tonumber(current) or 0) < tonumber(ms)) then
    redis.call('SET', key, ms)
  end
end

if #ARGV <= 2 then
  return {0, redis.call('GET', KEYS[2]) or ''}
end

if redis.call('EXISTS', KEYS[1]) == 1 then
  return {0, redis.call('GET', KEYS[2]) or ''}
end

redis.call('HSET', KEYS[1], unpack(ARGV, 3))
open_session(KEYS[2], ARGV[1], ARGV[2])
return {1, redis.call('GET', KEYS[2]) or ''}
