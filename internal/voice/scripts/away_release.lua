local room = redis.call('HGET', KEYS[1], ARGV[1])
if not room then
  return {}
end

local away_key = ARGV[4] .. ':room:' .. room .. ':away'
local raw = redis.call('HGET', away_key, ARGV[1])
if not raw then
  return {}
end

local entry = cjson.decode(raw)
if entry.conn_id ~= ARGV[2] or entry.instance_id ~= ARGV[3] then
  return {}
end

redis.call('HDEL', away_key, ARGV[1])
redis.call('HDEL', KEYS[1], ARGV[1])
return {room, raw}
