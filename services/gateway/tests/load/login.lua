local email = "loadtest@example.com"
local password = "TestPassword123!"

local headers = {
    ["Content-Type"] = "application/json"
}

local body = string.format(
    '{"email":"%s","password":"%s"}',
    email,
    password
)

request = function()
    return wrk.format(
        "POST",
        "/api/v1/auth/login",
        headers,
        body
    )
end

local errors_printed = 0

response = function(status, headers, body)
    if (status < 200 or status >= 400) and errors_printed < 10 then
        errors_printed = errors_printed + 1

        io.write(
            string.format(
                "\nERROR status=%d body=%s\n",
                status,
                body
            )
        )
    end
end
