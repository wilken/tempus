FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0 GOOS=linux

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/      cmd/
COPY internal/ internal/
RUN go build -trimpath -ldflags="-s -w" -o tempus ./cmd

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/tempus ./
COPY templates/ templates/
COPY static/    static/

RUN mkdir -p /storage && \
    addgroup -S tempus && adduser -S tempus -G tempus && \
    chown -R tempus:tempus /app /storage
USER tempus

EXPOSE 8080
CMD ["./tempus"]
