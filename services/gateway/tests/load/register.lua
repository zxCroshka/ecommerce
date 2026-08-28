local thread_counter = 0

setup = function(thread)
    thread_counter = thread_counter + 1

    thread:set("worker_id", thread_counter)
    thread:set(
        "run_id",
        tostring(os.time()) .. "_" .. tostring(math.random(100000, 999999))
    )
end

init = function(args)
    counter = 0
end

request = function()
    counter = counter + 1

    local email = string.format(
        "loadtest_%s_%d_%d@test.com",
        run_id,
        worker_id,
        counter
    )

    local body = string.format(
        '{"email":"%s","password":"TestPassword123!"}',
        email
    )

    local headers = {
        ["Content-Type"] = "application/json"
    }

    return wrk.format(
        "POST",
        "/api/v1/auth/register",
        headers,
        body
    )
end
