FROM golang:1.23-alpine AS builder
WORKDIR /src

COPY core/go.mod core/go.sum ./core/
WORKDIR /src/core
RUN go mod download

COPY core/ ./
RUN CGO_ENABLED=1 go build -o /out/peerdrive .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/peerdrive /app/peerdrive
EXPOSE 8080

ENTRYPOINT ["/app/peerdrive", "--config", "/data", "--port", "8080"]
