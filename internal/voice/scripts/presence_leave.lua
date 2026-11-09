local function open_session(key, ms, mode)
  local current = redis.call('GET', key)
  if not current or (mode == 'exact' and (tonumber(current) or 0) < tonumber(ms)) then
    redis.call('SET', key, ms)
  end
end

local removed = redis.call('HDEL', KEYS[1], ARGV[1])
redis.call('INCR', KEYS[5])

local active = 0
for _, raw in ipairs(redis.call('HVALS', KEYS[1])) do
  local participant = cjson.decode(raw)
  if redis.call('HEXISTS', KEYS[2], participant.user_id) == 0 then
    active = active + 1
  end
end

local removed_away = {}
if active > 0 then
  open_session(KEYS[4], ARGV[3], ARGV[4])
end
if active == 0 then
  redis.call('DEL', KEYS[4])
  local entries = redis.call('HGETALL', KEYS[2])
  redis.call('DEL', KEYS[2])
  for i = 1, #entries, 2 do
    if redis.call('HGET', KEYS[3], entries[i]) == ARGV[2] then
      redis.call('HDEL', KEYS[3], entries[i])
    end
    table.insert(removed_away, entries[i])
    table.insert(removed_away, entries[i + 1])
  end
end

return {removed, redis.call('HGETALL', KEYS[1]), removed_away, active, redis.call('GET', KEYS[4]) or ''}
