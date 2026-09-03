FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/service \
    ./services/user-service/cmd

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/service ./service
COPY services/user-service/config ./config

EXPOSE 9090 3366

ENTRYPOINT ["./service"]
