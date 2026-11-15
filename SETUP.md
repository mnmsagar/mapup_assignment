# Setup Guide — Geofencing & Real-time Alert System

A Go + PostGIS backend with a WebSocket alert hub, and a React dashboard for drawing geofences,
tracking vehicles and receiving live entry/exit alerts.

| Part | Stack | Local URL |
|---|---|---|
| Backend API | Go 1.23, `net/http`, pgx, gorilla/websocket | http://localhost:8080 |
| Real-time alerts | WebSocket | ws://localhost:8080/ws/alerts |
| Database | PostgreSQL 16 + PostGIS 3.4 | localhost:5432 |
| Frontend | React 18, Vite, TypeScript, Leaflet, Tailwind | http://localhost:5173 |

**Deployed:**
- Frontend: `<add Vercel URL>`
- Backend: `<add backend URL>`
- Docker image: `<dockerhub-user>/geofence-backend:latest`
- Video walkthrough: `<add link>`

---

## 1. Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| Docker + Docker Compose v2 | Docker 24+ | Running everything (recommended path) |
| Node.js + npm | Node 20+ | Frontend development without Docker |
| Go | 1.23+ | Backend development without Docker (optional) |

No other services or API keys are required. Map tiles come from OpenStreetMap.

---

## 2. Quick start with Docker Compose

```bash
git clone <repo-url>
cd Geofencing-Real-time-Alert-System
docker compose up --build
```

This starts three containers:

| Service | Port | Notes |
|---|---|---|
| `db` | 5432 | `postgis/postgis:16-3.4`, data in the `pgdata` volume |
| `backend` | 8080 | Waits for the DB, then applies migrations automatically on startup |
| `frontend` | 5173 | Production build served by nginx, calls the backend at `http://localhost:8080` |

Open **http://localhost:5173**. Check the backend with:

```bash
curl http://localhost:8080/health
# {"status":"ok","ws_clients":0,"time_ns":"648113"}
```

Useful commands:

```bash
docker compose logs -f backend   # structured JSON logs (requests, alerts dispatched)
docker compose down              # stop everything (data is kept)
docker compose down -v           # stop and wipe the database
```

---

## 3. Local development without Docker

### Database
Run only the database from Compose:

```bash
docker compose up -d db
```

### Backend

```bash
cd backend
go mod download
go run ./cmd/server          # uses DATABASE_URL default: postgres://geofence:geofence@localhost:5432/geofence
go test ./...                # unit tests
go vet ./...
```

If Go isn't installed, you can run the same commands in a container:

```bash
docker run --rm -v "$PWD/backend:/src" -w /src golang:1.23 go test ./...
```

On Git Bash for Windows, prefix the command with `MSYS_NO_PATHCONV=1`.

### Frontend

```bash
cd frontend
cp .env.example .env         # VITE_API_URL=http://localhost:8080
npm install
npm run dev                  # http://localhost:5173
npm run build                # typecheck + production build into dist/
```

### Configuration

**Backend** (environment variables):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable` | PostgreSQL/PostGIS connection string |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins, applied to both REST and WebSocket. Set it to the frontend URL in production. |
| `ALERT_RESTRICTED_DEFAULT` | `true` | Alert on every entry into a `restricted_zone`, even without a configured rule |
| `DISPATCHER_WORKERS` | `4` | Background workers that match alert rules and broadcast alerts |
| `LOCATION_RATE_PER_SEC` / `LOCATION_RATE_BURST` | `20` / `40` | Per-client-IP rate limit on `POST /vehicles/location` |
| `API_KEY` | *(unset)* | If set, every non-GET request must send `X-API-Key: <key>` |

**Frontend** (build-time variables):

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Backend base URL. The WebSocket URL is derived from it (`http`→`ws`, `https`→`wss`). |
| `VITE_API_KEY` | *(unset)* | Sent as `X-API-Key` when the backend has `API_KEY` set. It is visible in the JS bundle. |

---

## 4. API testing guide

Every response, including errors, carries `time_ns`: the handler's execution time in nanoseconds,
as a string. IDs are prefixed strings (`geo_`, `veh_`, `alert_`, `viol_`, `evt_`). Coordinates are
always `[latitude, longitude]`.

The walkthrough below runs in order against a fresh database. It builds a delivery zone that
overlaps a restricted zone, then drives a vehicle in and out of both.

```bash
API=http://localhost:8080
```

### 4.1 Create geofences — `POST /geofences`

```bash
curl -s -X POST $API/geofences -H "Content-Type: application/json" -d '{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "delivery_zone"
}'
# 201 {"id":"geo_1","name":"Downtown Delivery Zone","status":"active","time_ns":"..."}

curl -s -X POST $API/geofences -H "Content-Type: application/json" -d '{
  "name": "Airport No-Go",
  "coordinates": [[37.78,-122.416],[37.79,-122.416],[37.79,-122.40],[37.78,-122.40],[37.78,-122.416]],
  "category": "restricted_zone"
}'
# 201 {"id":"geo_2", ...}
```

Validation reports every problem at once:

```bash
curl -s -X POST $API/geofences -H "Content-Type: application/json" \
  -d '{"name":"","coordinates":[[0,0],[1,1],[95,0]],"category":"x"}'
# 400 {"error":"validation failed","details":[
#   "name is required",
#   "category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area",
#   "coordinates: need at least 4 points (3 unique + closing point), got 3",
#   "coordinates[2]: latitude 95 out of range [-90, 90]"], "time_ns":"..."}
```

The API also rejects polygons that are not closed, that have duplicate or collinear points, or that
intersect themselves (PostGIS `ST_IsValid`).

### 4.2 List geofences — `GET /geofences`

```bash
curl -s "$API/geofences"
curl -s "$API/geofences?category=restricted_zone"
```

### 4.3 Register a vehicle — `POST /vehicles`

```bash
curl -s -X POST $API/vehicles -H "Content-Type: application/json" -d '{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}'
# 201 {"id":"veh_1","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"..."}
```

Vehicle numbers are unique and case-insensitive, because they are stored upper-cased. Registering
the same number again returns `409 Conflict`.

### 4.4 List vehicles — `GET /vehicles`

```bash
curl -s $API/vehicles
```

Each vehicle also includes `current_location`, which is `null` until the first update.

### 4.5 Configure alert rules — `POST /alerts/configure`

```bash
# Alert on entry AND exit of the delivery zone, for veh_1 only
curl -s -X POST $API/alerts/configure -H "Content-Type: application/json" \
  -d '{"geofence_id":"geo_1","vehicle_id":"veh_1","event_type":"both"}'
# 201 {"alert_id":"alert_1","geofence_id":"geo_1","vehicle_id":"veh_1","event_type":"both","status":"active","time_ns":"..."}

# Omit vehicle_id to apply the rule to all vehicles
curl -s -X POST $API/alerts/configure -H "Content-Type: application/json" \
  -d '{"geofence_id":"geo_2","event_type":"exit"}'
```

Unknown geofence or vehicle → `404`. Exact duplicate rule → `409`.

### 4.6 List alert rules — `GET /alerts`

```bash
curl -s "$API/alerts"
curl -s "$API/alerts?geofence_id=geo_1&vehicle_id=veh_1"
```

### 4.7 Watch real-time alerts — `WS /ws/alerts`

Open a listener in a second terminal before sending location updates. Any WebSocket client works:

```bash
npx wscat -c ws://localhost:8080/ws/alerts
# or: websocat ws://localhost:8080/ws/alerts
```

You can also just keep the dashboard open. Alerts appear as toasts and in the **Live** tab.

### 4.8 Update location — `POST /vehicles/location`

```bash
# Outside every geofence: no events
curl -s -X POST $API/vehicles/location -H "Content-Type: application/json" \
  -d '{"vehicle_id":"veh_1","latitude":37.70,"longitude":-122.41,"timestamp":"2025-01-15T10:30:00Z"}'
# {"vehicle_id":"veh_1","location_updated":true,"current_geofences":[],"time_ns":"..."}

# Inside both zones: two entry events
curl -s -X POST $API/vehicles/location -H "Content-Type: application/json" \
  -d '{"vehicle_id":"veh_1","latitude":37.782,"longitude":-122.412,"timestamp":"2025-01-15T10:35:00Z"}'
# {"vehicle_id":"veh_1","location_updated":true,"current_geofences":[
#   {"geofence_id":"geo_1","geofence_name":"Downtown Delivery Zone","status":"inside"},
#   {"geofence_id":"geo_2","geofence_name":"Airport No-Go","status":"inside"}],"time_ns":"..."}
```

The WebSocket listener receives one message per alerted event:

```json
{
  "event_id": "evt_1",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": { "vehicle_id": "veh_1", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe" },
  "geofence": { "geofence_id": "geo_1", "geofence_name": "Downtown Delivery Zone", "category": "delivery_zone" },
  "location": { "latitude": 37.782, "longitude": -122.412 }
}
```

The `geo_1` entry fires because of rule `alert_1`. The `geo_2` entry fires because entries into
restricted zones always alert (`ALERT_RESTRICTED_DEFAULT=true`).

```bash
# Leave both zones: exit events (geo_1 via alert_1, geo_2 via the "exit" rule)
curl -s -X POST $API/vehicles/location -H "Content-Type: application/json" \
  -d '{"vehicle_id":"veh_1","latitude":37.70,"longitude":-122.41,"timestamp":"2025-01-15T10:40:00Z"}'
```

Edge cases:
- **Out-of-order updates.** A timestamp older than the vehicle's last known position is saved in
  the location history only. It changes no state, creates no events, and the response says so in a
  `note` field.
- **Future timestamps.** More than 5 minutes ahead of server time → `400`.
- **Unknown vehicle.** `404 {"error":"vehicle veh_9 not found"}`.
- **Rate limit.** Too many requests from one IP → `429` with a `Retry-After` header.

### 4.9 Current location — `GET /vehicles/location/{vehicle_id}`

```bash
curl -s $API/vehicles/location/veh_1
# {"vehicle_id":"veh_1","vehicle_number":"KA-01-AB-1234",
#  "current_location":{"latitude":37.7,"longitude":-122.41,"timestamp":"2025-01-15T10:40:00Z"},
#  "current_geofences":[],"time_ns":"..."}
```

### 4.10 Event history — `GET /violations/history`

```bash
curl -s "$API/violations/history?vehicle_id=veh_1&limit=100"
curl -s "$API/violations/history?geofence_id=geo_2&start_date=2025-01-15T00:00:00Z&end_date=2025-01-16T00:00:00Z"
curl -s "$API/violations/history?limit=10&offset=10"     # page 2
```

`limit` defaults to 50 and is capped at 500. `total_count` is the number of rows matching the
filters, before pagination. The history records **every** entry/exit event, whether or not an alert
rule matched it.

### 4.11 Extra endpoints (beyond the spec)

| Endpoint | Purpose |
|---|---|
| `DELETE /alerts/{alert_id}` | Remove an alert rule (used by the dashboard's "Delete" button) |
| `GET /alerts/events?limit=50` | Recently delivered alerts, so the live feed is backfilled after a page reload |
| `GET /health` | Database connectivity and the number of connected WebSocket clients |
| `?limit=&offset=` | Optional pagination on `GET /geofences`, `/vehicles`, `/alerts` |

---

## 5. Frontend usage guide

The screen has an interactive map (left, or top on mobile) and a tabbed panel.

1. **Geofences.** Fill in the name, description and category. Click **Draw on map**, click to add
   points, then **Finish drawing**. The polygon closes automatically. You can also paste one
   `lat, lng` per line (or a JSON array) into the boundary box. Filter the list by category.
   Colors on the map: blue = delivery, red dashed = restricted, amber = toll, green = customer area.
2. **Vehicles.** Register a vehicle. Click a vehicle in the list (or its marker on the map) to load
   its current location and the geofences it is inside.
3. **Location.** Pick a vehicle. Click **Pick location on map** and click anywhere, or type the
   coordinates. Keep **Use current time** checked, or untick it to set a timestamp. Click **Send
   location update**. The result shows which geofences now contain the vehicle.
4. **Alert rules.** Choose a geofence, optionally a vehicle (default: all vehicles), and a trigger:
   entry, exit or both. Existing rules can be filtered and deleted.
5. **Live.** Alerts arrive over the WebSocket in real time. Each one also shows as a toast,
   increments the 🔔 counter, and briefly highlights its geofence on the map. Restricted-zone entries
   are marked red with ⚠️. The header pill shows the connection state. The client reconnects with
   exponential backoff (1 s → 30 s).
6. **History.** Every entry/exit event, filterable by vehicle, geofence and date range, with page
   size and prev/next paging.

**Demo in about 1 minute:** draw a restricted zone → register a vehicle → open **Location**, pick a
point outside the zone and send it → pick a point inside and send it. A ⚠️ alert toast appears, and
the event shows up in **History**.

---

## 6. Architecture overview

```
 React dashboard ──REST──▶ Go API (net/http) ──pgx──▶ PostgreSQL + PostGIS
        ▲                        │
        └──── WebSocket ◀── Hub ◀┴── Dispatcher (worker pool)
```

```
backend/
  cmd/server/            wiring, graceful shutdown (drains queued alerts before exit)
  internal/config/       env configuration
  internal/db/           connection pool with startup retry + embedded SQL migrations
  internal/httpx/        WriteJSON (adds time_ns), CORS, logging, panic recovery, rate limit, API key
  internal/geo/          pure polygon / coordinate validation (unit tested)
  internal/store/        all SQL: geofences, vehicles, locations, alert rules, violations
  internal/alerts/       WebSocket hub + async alert dispatcher (unit tested)
  internal/api/          HTTP handlers and routes
frontend/src/
  api/                   typed fetch client + shared types
  hooks/                 useAlertsSocket (reconnect, dedupe, backfill), useResource (loading/error)
  components/            MapView, one panel per tab, shared UI primitives
```

### Location update flow

`POST /vehicles/location` runs everything in **one transaction**:

1. Lock the vehicle row (`SELECT … FOR UPDATE`). This serializes concurrent updates for the same
   vehicle, so the entry/exit diff can't race.
2. Append the point to `vehicle_locations`, the full movement history.
3. Point-in-polygon test: `ST_Covers(boundary, point)` against the GIST-indexed `geofences.boundary`.
   Points exactly on the boundary count as inside.
4. Diff the result with `vehicle_geofence_state` (the set of zones the vehicle was in) → entry and
   exit events. Update the state and write each event to `violations`.
5. Commit and respond. The events then go to the **Dispatcher**, a buffered channel served by a
   worker pool. The workers match `alert_rules` (same geofence; vehicle unset or equal; event type
   equal or `both`), store each alert in `alert_events`, and broadcast it through the hub. The HTTP
   response never waits for alert delivery.

### Database schema

| Table | Purpose | Key indexes |
|---|---|---|
| `geofences` | Polygon as `geometry(Polygon,4326)` + the original coordinates as JSONB | GIST(boundary), category |
| `vehicles` | Registry + last known position | unique vehicle_number |
| `vehicle_locations` | Every reported point | (vehicle_id, recorded_at DESC) |
| `vehicle_geofence_state` | Zones each vehicle is currently inside | PK (vehicle_id, geofence_id) |
| `alert_rules` | Alert configuration | unique (geofence, vehicle, event_type) |
| `violations` | Every entry/exit event | occurred_at; (vehicle, time); (geofence, time) |
| `alert_events` | Every delivered alert + its payload | created_at |

### WebSocket hub

A single goroutine owns the set of connected clients, so the hub needs no locks. Each client has a
read pump (handles pong and close frames) and a write pump with a buffered queue. The server pings
every 50 s and drops a connection after 60 s of silence. A slow client is disconnected instead of
blocking the broadcast to everyone else.

### Design decisions and limitations

- PostGIS with `geometry` (planar on lat/lng) is accurate enough for city-scale zones. Very large or
  antimeridian-crossing polygons would need `geography`.
- The WebSocket hub runs in memory, which is fine for a single instance. To scale out
  horizontally, publish alerts through Redis pub/sub or NATS so that every instance can broadcast
  them.
- Rate limiting is per instance and keyed on the client IP (or the first `X-Forwarded-For` entry,
  so set this only behind a trusted proxy).
- Deleting a geofence is not part of the spec and isn't exposed. If one is deleted in the database,
  its state rows are removed by cascade, without generating exit events.

---

## 7. Docker Hub & deployment

```bash
# Build and push the backend image
docker build -t <dockerhub-user>/geofence-backend:latest ./backend
docker push <dockerhub-user>/geofence-backend:latest
```

**Backend** (any Docker host: Render, Railway, Fly.io, a VM):
- Provision PostgreSQL with the PostGIS extension available. The migration runs
  `CREATE EXTENSION IF NOT EXISTS postgis` itself.
- Set `DATABASE_URL` and `CORS_ORIGINS=<frontend URL>`. Most platforms inject `PORT` automatically.
- The platform must support WebSocket upgrades on the same port. The server sets no write timeout,
  so long-lived connections stay open.

**Frontend** (Vercel / Netlify / Cloudflare Pages):
- Root directory `frontend`, build command `npm run build`, output directory `dist`.
- Set `VITE_API_URL=https://<backend-host>` (use `https`, so the WebSocket uses `wss`).
- `frontend/vercel.json` adds the SPA rewrite.
