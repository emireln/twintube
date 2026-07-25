# Multi-stage Docker build for TwinTube Production VPS Deployment
FROM golang:1.21-alpine AS builder

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s \
    -X twintube/internal/version.Version=${VERSION} \
    -X twintube/internal/version.Commit=${GIT_COMMIT} \
    -X twintube/internal/version.Build=${BUILD_DATE}" \
    -o twintube .

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/twintube .
COPY --from=builder /app/static ./static
COPY --from=builder /app/VERSION ./VERSION

EXPOSE 8080

CMD ["./twintube"]
