local function open_session(key, ms, mode)
  local current = redis.call('GET', key)
  if not current or (mode == 'exact' and (tonumber(current) or 0) < tonumber(ms)) then
    redis.call('SET', key, ms)
  end
end

local expected = tonumber(ARGV[3])
local revision = tonumber(redis.call('GET', KEYS[3]) or '0')
if revision ~= expected then
  return {0, redis.call('GET', KEYS[2]) or '', 1, {}}
end

local had_presence = redis.call('EXISTS', KEYS[1])
local changed = 0
local removed_away = {}

if #ARGV <= 4 then
  if had_presence == 1 then
    redis.call('DEL', KEYS[1])
    changed = 1
  end
  redis.call('DEL', KEYS[2])
  local entries = redis.call('HGETALL', KEYS[4])
  redis.call('DEL', KEYS[4])
  for i = 1, #entries, 2 do
    if redis.call('HGET', KEYS[5], entries[i]) == ARGV[4] then
      redis.call('HDEL', KEYS[5], entries[i])
    end
    table.insert(removed_away, entries[i])
    table.insert(removed_away, entries[i + 1])
  end
else
  local current = redis.call('HGETALL', KEYS[1])
  if #current / 2 ~= (#ARGV - 4) / 2 then
    changed = 1
  else
    local fields = {}
    for i = 1, #current, 2 do
      fields[current[i]] = current[i + 1]
    end
    for i = 5, #ARGV, 2 do
      if fields[ARGV[i]] ~= ARGV[i + 1] then
        changed = 1
        break
      end
    end
  end
  if changed == 1 then
    redis.call('DEL', KEYS[1])
    redis.call('HSET', KEYS[1], unpack(ARGV, 5))
  end
end

if changed == 1 then
  redis.call('INCR', KEYS[3])
end

if had_presence == 0 and #ARGV > 4 then
  open_session(KEYS[2], ARGV[1], ARGV[2])
end

return {changed, redis.call('GET', KEYS[2]) or '', 0, removed_away}
