FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qa-server ./cmd/server

FROM alpine:3.21

RUN addgroup -S qa && adduser -S -G qa qa && mkdir -p /workspace && chown qa:qa /workspace
COPY --from=build /out/qa-server /usr/local/bin/qa-server

USER qa
WORKDIR /workspace
EXPOSE 8084

ENTRYPOINT ["qa-server"]
