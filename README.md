# earthquake-notifier

Polls the [USGS Earthquake API](https://earthquake.usgs.gov/fdsnws/event/1/) and sends alerts when earthquakes occur within a configurable radius of a watch point.

## Features

- Geographic watch area (latitude, longitude, radius in km)
- Minimum magnitude filter
- Multiple notification backends: Discord, generic webhook, or log-only
- Deduplication with persistent state (no repeat alerts for the same event)
- Re-notifies when magnitude changes by 0.2 or more
- Batches multiple events from one check into a single message
- Docker image published to GHCR on push to `main`

## Quick start

### Local

```bash
cp .env.example .env
# Edit .env with your watch area and notification settings

go run .
```

### Docker

```bash
docker build -t earthquake-notifier .
docker run -d --name earthquake-notifier \
  --env-file .env \
  -v earthquake-notifier-data:/data \
  earthquake-notifier
```

Pre-built image:

```bash
docker run -d --name earthquake-notifier \
  --env-file .env \
  -v earthquake-notifier-data:/data \
  ghcr.io/ardean/earthquake-notifier:latest
```

Mount `/data` (or set `STATE_FILE`) so seen-event state survives restarts.

## Configuration

All settings are read from environment variables. See `.env.example` for a template.

| Variable | Required | Default | Description |
|---|---|---|---|
| `NOTIFY_METHODS` | No | `discord` | Comma-separated list: `discord`, `webhook`, `log` |
| `DISCORD_TOKEN` | If using discord | — | Discord bot token |
| `DISCORD_CHANNEL_ID` | If using discord | — | Channel ID to post messages to |
| `WEBHOOK_URL` | If using webhook | — | URL for generic HTTP POST notifications |
| `WATCH_LATITUDE` | Yes | — | Watch point latitude (-90 to 90) |
| `WATCH_LONGITUDE` | Yes | — | Watch point longitude (-180 to 180) |
| `WATCH_RADIUS_KM` | Yes | — | Alert radius in kilometres |
| `MIN_MAGNITUDE` | No | `3.0` | Minimum magnitude to alert on |
| `CHECK_INTERVAL` | No | `2m` | How often to poll USGS (Go duration, e.g. `30s`, `5m`) |
| `LOOKBACK` | No | `24h` | On first run, how far back to fetch events |
| `NOTIFY_STARTUP_SHUTDOWN` | No | `true` | Send messages when the watcher starts/stops |
| `STATE_FILE` | No | `data/seen_events.json` | Path to persisted seen-event state |
| `SERVER_HOSTNAME` | No | system hostname | Prefix added to notification messages |

## Discord setup

1. Create an application at [Discord Developer Portal](https://discord.com/developers/applications)
2. Add a bot and copy the token → `DISCORD_TOKEN`
3. Enable **Message Content Intent** under Bot settings
4. Invite the bot to your server with permission to send messages in the target channel
5. Enable Developer Mode in Discord, right-click the channel → Copy Channel ID → `DISCORD_CHANNEL_ID`

## How it works

On each check, the app fetches earthquakes updated since the previous check (or since `LOOKBACK` ago on the first run). Events within the watch radius and above `MIN_MAGNITUDE` that haven't been seen before trigger a notification. If USGS revises the magnitude by 0.2 or more, a follow-up alert is sent.

Multiple events from the same check are combined into one message (sorted by magnitude, largest first). Very long lists are split across messages to stay within Discord's 2000-character limit.

Seen events are stored in `STATE_FILE` and pruned after 30 days.
