if #ARGV == 0 or redis.call('EXISTS', KEYS[1]) == 1 then
  return 0
end

redis.call('HSET', KEYS[1], unpack(ARGV))
return 1
