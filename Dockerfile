FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o validator ./cmd/validator

# Runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/validator .

# Create non-root user
RUN addgroup -g 1000 validator && \
    adduser -D -u 1000 -G validator validator && \
    chown -R validator:validator /root

USER validator

EXPOSE 8080

CMD ["./validator"]
