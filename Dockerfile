FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/earthquake-notifier .

FROM alpine:3.21

RUN addgroup -S eqnotifier && adduser -S eqnotifier -G eqnotifier && \
    apk add --no-cache ca-certificates && \
    mkdir -p /data

WORKDIR /app

COPY --from=builder /out/earthquake-notifier ./earthquake-notifier

RUN chown -R eqnotifier:eqnotifier /app /data

USER eqnotifier

ENV STATE_FILE=/data/seen_events.json

VOLUME /data

CMD ["./earthquake-notifier"]
