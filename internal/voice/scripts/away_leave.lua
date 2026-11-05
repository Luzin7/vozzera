local room = redis.call('HGET', KEYS[1], ARGV[1])
if not room then
  return {}
end

local away_key = ARGV[2] .. ':room:' .. room .. ':away'
local raw = redis.call('HGET', away_key, ARGV[1])
redis.call('HDEL', away_key, ARGV[1])
redis.call('HDEL', KEYS[1], ARGV[1])

if not raw then
  return {}
end
return {room, raw}
