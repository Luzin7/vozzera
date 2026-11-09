redis.call('DEL', KEYS[1], KEYS[4])
redis.call('INCR', KEYS[5])

local removed_away = {}
local entries = redis.call('HGETALL', KEYS[2])
redis.call('DEL', KEYS[2])
for i = 1, #entries, 2 do
  if redis.call('HGET', KEYS[3], entries[i]) == ARGV[1] then
    redis.call('HDEL', KEYS[3], entries[i])
  end
  table.insert(removed_away, entries[i])
  table.insert(removed_away, entries[i + 1])
end

return removed_away
