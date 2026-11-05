local active_others = 0
for _, raw in ipairs(redis.call('HVALS', KEYS[1])) do
  local participant = cjson.decode(raw)
  if participant.user_id ~= ARGV[1] and redis.call('HEXISTS', KEYS[2], participant.user_id) == 0 then
    active_others = active_others + 1
  end
end
if active_others == 0 then
  return {'rejected', ''}
end

local previous_room = redis.call('HGET', KEYS[3], ARGV[1])
local moved_from = ''
local entry = ARGV[2]

if previous_room and previous_room ~= ARGV[3] then
  redis.call('HDEL', ARGV[4] .. ':room:' .. previous_room .. ':away', ARGV[1])
  moved_from = previous_room
end

if previous_room == ARGV[3] then
  local existing = redis.call('HGET', KEYS[2], ARGV[1])
  if existing then
    local merged = cjson.decode(entry)
    merged.since = cjson.decode(existing).since
    entry = cjson.encode(merged)
  end
end

redis.call('HSET', KEYS[2], ARGV[1], entry)
redis.call('HSET', KEYS[3], ARGV[1], ARGV[3])

return {'accepted', moved_from}
