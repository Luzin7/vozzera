local function open_session(key, ms, mode)
  local current = redis.call('GET', key)
  if not current or (mode == 'exact' and (tonumber(current) or 0) < tonumber(ms)) then
    redis.call('SET', key, ms)
  end
end

redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('INCR', KEYS[4])
open_session(KEYS[3], ARGV[6], ARGV[7])

local cleared_room = ''
local cleared_entry = ''

local away_room = redis.call('HGET', KEYS[2], ARGV[3])
if away_room then
  local away_key = ARGV[5] .. ':room:' .. away_room .. ':away'
  local raw = redis.call('HGET', away_key, ARGV[3])
  if raw and tonumber(cjson.decode(raw).since) <= tonumber(ARGV[4]) then
    redis.call('HDEL', away_key, ARGV[3])
    redis.call('HDEL', KEYS[2], ARGV[3])
    cleared_room = away_room
    cleared_entry = raw
  end
end

return {redis.call('HGETALL', KEYS[1]), cleared_room, cleared_entry, redis.call('GET', KEYS[3]) or ''}
