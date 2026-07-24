# Multi-stage Docker build for TwinTube Production VPS Deployment
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install ca-certificates and git
RUN apk add --no-cache ca-certificates git

# Copy module files & download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Build lightweight static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o twintube .

# Production runtime image
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy binary and static assets
COPY --from=builder /app/twintube .
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./twintube"]
