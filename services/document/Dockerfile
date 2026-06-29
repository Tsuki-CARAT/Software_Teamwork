FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/document-service ./cmd/server

FROM alpine:3.20

RUN adduser -D -H document
USER document
COPY --from=build /out/document-service /usr/local/bin/document-service
EXPOSE 8085
ENTRYPOINT ["document-service"]
