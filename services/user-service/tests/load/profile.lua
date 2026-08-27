local token = os.getenv("ACCESS_TOKEN")

if token == nil or token == "" then
    error("ACCESS_TOKEN environment variable is required")
end

local headers = {
    ["Authorization"] = "Bearer " .. token,
    ["Accept"] = "application/json"
}

request = function()
    return wrk.format(
        "GET",
        "/api/v1/user/profile",
        headers
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