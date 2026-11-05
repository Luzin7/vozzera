local removed = {}
local pairs_flat = redis.call('HGETALL', KEYS[1])
local limit = tonumber(ARGV[2])

for i = 1, #pairs_flat, 2 do
  if (i + 1) / 2 > limit then
    break
  end

  local user = pairs_flat[i]
  local room = pairs_flat[i + 1]
  local away_key = ARGV[1] .. ':room:' .. room .. ':away'
  local raw = redis.call('HGET', away_key, user)

  if not raw then
    redis.call('HDEL', KEYS[1], user)
  else
    local entry = cjson.decode(raw)
    if redis.call('EXISTS', ARGV[1] .. ':instance:' .. entry.instance_id) == 0 then
      redis.call('HDEL', away_key, user)
      redis.call('HDEL', KEYS[1], user)
      table.insert(removed, room)
      table.insert(removed, user)
      table.insert(removed, raw)
    end
  end
end

return removed
