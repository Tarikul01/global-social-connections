# FindNeighbour — Architecture & Design Document

**Project:** GSC — **Global Social Connections**  
**Product:** FindNeighbour  
**Version:** 1.0  
**Status:** Draft for Review  
**Date:** 2026-08-22

---

## 1. Executive Summary

**GSC (Global Social Connections)** is the project brand. **FindNeighbour** is the location-aware professional networking and meetup product built for GSC.

FindNeighbour is a **location-aware professional networking and meetup platform** built as a **modular monolith** in Go. The system prioritizes:

| Priority | Approach |
|----------|----------|
| Security | Server-side authz, Argon2id, CSRF, rate limiting, audit logs |
| Privacy | Tiered location exposure, consent-based live sharing, fuzzing |
| Scalability | PostGIS indexing, Redis ephemeral state, connection pooling |
| Maintainability | MVC layers, dependency injection, module boundaries |

**Core user journey:** Register → Profile → Enable discovery → Find nearby professionals → Connect → Chat → Create/join events → Optional live location during events → Meet in person.

---

## 2. System Architecture

### 2.1 High-Level Architecture

```mermaid
flowchart TB
    subgraph Client
        Browser[Browser<br/>HTML + HTMX + Alpine.js]
    end

    subgraph App["FindNeighbour (Modular Monolith)"]
        Router[Gin Router]
        MW[Middleware Stack]
        CTRL[Controllers]
        SVC[Services]
        REPO[Repositories]
    end

    subgraph Realtime
        WS[WebSocket Handler]
        Hub[Connection Hub]
    end

    subgraph Data
        PG[(Supabase PostgreSQL<br/>+ PostGIS)]
        Redis[(Redis)]
        Storage[(Supabase Storage)]
    end

    subgraph External
        OSM[OpenStreetMap tiles]
        SMTP[Email Provider]
    end

    Browser -->|HTTP/HTTPS| Router
    Browser -->|WebSocket| WS
    Browser -->|tile requests| OSM
    Router --> MW --> CTRL --> SVC --> REPO
    REPO --> PG
    SVC --> Redis
    SVC --> Storage
    SVC --> SMTP
    WS --> Hub
    Hub --> Redis
    Hub --> SVC
```

### 2.2 Request Flow (HTTP)

```
HTTP Request
    ↓
Gin Router (route matching)
    ↓
Global Middleware (request ID, logging, security headers, rate limit)
    ↓
Route Middleware (auth session, CSRF for mutating forms)
    ↓
Controller (parse input, call service, render response)
    ↓
Service (business rules, authorization, orchestration)
    ↓
Repository (parameterized SQL, transactions)
    ↓
PostgreSQL / Redis / Supabase Storage
```

### 2.3 Layer Responsibilities

| Layer | Responsibility | Must NOT |
|-------|----------------|----------|
| **Controller** | HTTP binding, status codes, template rendering | Business logic, SQL |
| **Service** | Business rules, authorization, transactions | Raw HTTP concerns |
| **Repository** | CRUD, queries, spatial SQL | Authorization decisions |
| **Middleware** | Cross-cutting: auth, CSRF, rate limit, logging | Domain logic |

### 2.4 Dependency Injection

All dependencies injected via constructors at application bootstrap (`cmd/server/main.go`):

```go
// Composition root pattern
type App struct {
    Config     *config.Config
    DB         *database.Pool
    Redis      *redis.Client
    Logger     *logger.Logger
    // Module services wired here
}

func NewApp(cfg *config.Config) (*App, error) { ... }
```

No global mutable state. Services receive interfaces (repository interfaces) for testability.

### 2.5 Modular Monolith Boundaries

Each domain module is self-contained with its own controller, service, repository, models, and DTOs. Cross-module communication goes through **service interfaces**, never direct repository access across modules.

```
internal/
├── auth/          # Sessions, registration, login
├── user/          # Core user entity
├── profile/       # Profile CRUD, photos
├── profession/    # Profession catalog
├── tag/           # Tags (user-defined + catalog)
├── skill/         # Skills
├── social/        # Social links (flexible platform)
├── follow/        # One-way follows
├── connection/    # Bidirectional connections
├── location/      # Location storage & privacy
├── discovery/     # Nearby people queries
├── event/         # Events & meetups
├── chat/          # Conversations & messages
├── notification/  # In-app notifications
├── block/         # User blocking
├── report/        # Moderation reports
├── websocket/     # WS hub, auth, routing
├── storage/       # Supabase file uploads
├── map/           # (optional) geocoding helpers; UI uses Leaflet + OSM
├── middleware/    # HTTP middleware
└── config/        # Environment config
```

Shared infrastructure:

```
internal/
├── database/      # pgx pool, migrations runner
├── redis/         # Redis client wrapper
├── security/      # Password hash, CSRF, tokens
└── logger/        # Structured slog wrapper
```

---

## 3. Technology Decisions

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.22+ | Performance, concurrency, stdlib |
| HTTP Framework | Gin | Mature, fast, middleware ecosystem |
| Database | Supabase PostgreSQL + PostGIS | Managed PG, spatial queries, storage co-location |
| ORM | **None** — raw SQL via `pgx` | Full control, optimized PostGIS queries |
| Cache/Realtime | Redis 7+ | Pub/Sub, TTL state, presence |
| Auth | **Custom sessions** (not Supabase Auth) | Full control over session lifecycle, CSRF integration, no vendor lock-in for auth flow; Supabase used only for PG + Storage |
| Password Hash | Argon2id | OWASP recommended |
| WebSocket | `gorilla/websocket` or `nhooyr.io/websocket` | Battle-tested, context support |
| Templates | Go `html/template` | Auto-escaping (XSS protection) |
| Frontend | HTMX + Alpine.js + Tailwind | Server-rendered, minimal JS |
| Maps | Leaflet + OpenStreetMap tiles | Free, no API key, replaceable tile provider |
| Migrations | `golang-migrate/migrate` | Versioned SQL migrations |
| Testing | `testing` + `testify` + `testcontainers-go` | Integration tests with real PG/Redis |
| Containerization | Docker + Docker Compose | Local dev parity |

### 3.1 Why Custom Auth (Not Supabase Auth)

Supabase Auth adds complexity for a server-rendered HTMX app:
- JWT in cookies requires careful CSRF pairing
- Session invalidation is harder to control
- Email verification flow must be customized anyway

**Decision:** Implement auth in Go with sessions stored in PostgreSQL. Supabase provides PostgreSQL and Storage only. This keeps the security model fully under our control and aligns with HttpOnly cookie sessions.

---

## 4. Module Structure

### 4.1 Per-Module File Layout

```
internal/{module}/
├── controller.go    # HTTP handlers
├── service.go       # Business logic + authorization
├── repository.go    # SQL queries
├── model.go         # Domain entities
├── dto.go           # Request/response DTOs
├── validator.go     # Input validation
└── service_test.go  # Unit tests
```

### 4.2 Module Interaction Map

```mermaid
flowchart LR
    auth --> user
    profile --> user
    profile --> profession
    profile --> tag
    profile --> skill
    profile --> social
    profile --> storage
    follow --> user
    follow --> block
    connection --> user
    connection --> block
    connection --> notification
    connection --> chat
    discovery --> location
    discovery --> profile
    discovery --> block
    event --> location
    event --> notification
    event --> storage
    chat --> connection
    chat --> block
    chat --> notification
    chat --> websocket
    location --> websocket
    location --> redis
    websocket --> redis
    websocket --> auth
```

### 4.3 Cross-Cutting: Authorization Service

A shared `internal/security/authorizer.go` provides reusable checks:

```go
type Authorizer interface {
    CanViewProfile(viewerID, targetID uuid.UUID) error
    CanViewLocation(viewerID, targetID uuid.UUID, context LocationContext) error
    CanMessage(senderID, recipientID uuid.UUID) error
    CanManageEvent(userID, eventID uuid.UUID) error
    CanViewEventLiveLocation(viewerID, targetID, eventID uuid.UUID) error
    IsBlocked(userA, userB uuid.UUID) (bool, error)
}
```

Each domain service calls the authorizer before acting.

---

## 5. Database Schema & ERD

### 5.1 Entity Relationship Diagram

```mermaid
erDiagram
    users ||--o| profiles : has
    users ||--o{ sessions : has
    users ||--o{ user_locations : has
    users ||--o{ location_settings : has
    users ||--o{ follows : "follows/followed_by"
    users ||--o{ connections : "requests/receives"
    users ||--o{ blocks : "blocks/blocked_by"
    users ||--o{ reports : reports
    users ||--o{ notifications : receives
    users ||--o{ audit_logs : generates

    profiles ||--o{ user_tags : has
    profiles ||--o{ user_skills : has
    profiles ||--o{ social_links : has
    profiles }o--|| professions : belongs_to

    tags ||--o{ user_tags : tagged
    skills ||--o{ user_skills : skilled

    connections ||--o| conversations : creates
    conversations ||--o{ conversation_members : has
    conversations ||--o{ messages : contains

    users ||--o{ events : creates
    events ||--o{ event_join_requests : receives
    events ||--o{ event_members : has
    events ||--o{ event_live_consent : "consent records"

    users ||--o{ files : uploads
```

### 5.2 Core Tables

#### `users`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK, DEFAULT gen_random_uuid() |
| email | VARCHAR(255) | UNIQUE, NOT NULL |
| email_verified_at | TIMESTAMPTZ | NULL |
| password_hash | VARCHAR(255) | NOT NULL |
| username | VARCHAR(50) | UNIQUE, NOT NULL |
| status | VARCHAR(20) | NOT NULL DEFAULT 'active' — active, suspended, deleted |
| failed_login_attempts | INT | DEFAULT 0 |
| locked_until | TIMESTAMPTZ | NULL |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |
| deleted_at | TIMESTAMPTZ | NULL (soft delete) |

**Indexes:** `email`, `username`, `status` WHERE deleted_at IS NULL

#### `sessions`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| token_hash | VARCHAR(64) | UNIQUE, NOT NULL (SHA-256 of session token) |
| ip_address | INET | |
| user_agent | TEXT | |
| expires_at | TIMESTAMPTZ | NOT NULL |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() |
| revoked_at | TIMESTAMPTZ | NULL |

**Indexes:** `token_hash`, `user_id`, `expires_at`

#### `profiles`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, UNIQUE, NOT NULL |
| full_name | VARCHAR(100) | NOT NULL |
| bio | TEXT | |
| about | TEXT | |
| company | VARCHAR(100) | |
| job_title | VARCHAR(100) | |
| experience_years | INT | CHECK (experience_years >= 0) |
| profession_id | UUID | FK → professions, NULL |
| profile_photo_file_id | UUID | FK → files, NULL |
| cover_photo_file_id | UUID | FK → files, NULL |
| availability_status | VARCHAR(20) | available, busy, away, offline |
| looking_for | VARCHAR(50)[] | networking, mentorship, cofounder, etc. |
| is_verified | BOOLEAN | DEFAULT false |
| onboarding_completed | BOOLEAN | DEFAULT false |
| created_at | TIMESTAMPTZ | NOT NULL |
| updated_at | TIMESTAMPTZ | NOT NULL |

#### `professions`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| name | VARCHAR(100) | UNIQUE, NOT NULL |
| slug | VARCHAR(100) | UNIQUE, NOT NULL |
| is_system | BOOLEAN | DEFAULT true |
| created_by | UUID | FK → users, NULL (for custom) |

Pre-seeded: Software Engineer, Web Developer, Backend Developer, Frontend Developer, Designer, Entrepreneur, Investor, Founder, Student, Freelancer, etc.

#### `tags`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| name | VARCHAR(50) | NOT NULL |
| slug | VARCHAR(50) | UNIQUE, NOT NULL |
| created_by | UUID | FK → users, NULL |

#### `user_tags`

| Column | Type | Constraints |
|--------|------|-------------|
| user_id | UUID | FK → users |
| tag_id | UUID | FK → tags |
| | | PRIMARY KEY (user_id, tag_id) |

#### `skills`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| name | VARCHAR(100) | UNIQUE, NOT NULL |
| created_by | UUID | FK → users, NULL |

#### `user_skills`

| Column | Type | Constraints |
|--------|------|-------------|
| user_id | UUID | FK → users |
| skill_id | UUID | FK → skills |
| proficiency | VARCHAR(20) | beginner, intermediate, advanced, expert |
| | | PRIMARY KEY (user_id, skill_id) |

#### `social_links`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| platform | VARCHAR(50) | NOT NULL (github, linkedin, twitter, etc.) |
| url | VARCHAR(500) | NOT NULL |
| display_order | INT | DEFAULT 0 |
| | | UNIQUE (user_id, platform) |

Flexible — no hardcoded platform columns.

#### `follows`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| follower_id | UUID | FK → users, NOT NULL |
| following_id | UUID | FK → users, NOT NULL |
| created_at | TIMESTAMPTZ | NOT NULL |
| | | UNIQUE (follower_id, following_id) |
| | | CHECK (follower_id != following_id) |

**Indexes:** `follower_id`, `following_id`

#### `connections`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| requester_id | UUID | FK → users, NOT NULL |
| recipient_id | UUID | FK → users, NOT NULL |
| status | VARCHAR(20) | NOT NULL — pending, accepted, rejected, cancelled, blocked |
| message | TEXT | Optional intro message |
| responded_at | TIMESTAMPTZ | NULL |
| created_at | TIMESTAMPTZ | NOT NULL |
| updated_at | TIMESTAMPTZ | NOT NULL |
| | | UNIQUE (requester_id, recipient_id) |
| | | CHECK (requester_id != recipient_id) |

**State machine:**

```
PENDING → ACCEPTED | REJECTED | CANCELLED (by requester)
ACCEPTED → BLOCKED (via block system)
REJECTED → (terminal, can re-request after cooldown — optional)
CANCELLED → (terminal, can re-request)
```

**Indexes:** `requester_id`, `recipient_id`, `status`

#### `user_locations`

Permanent/general location (not live).

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, UNIQUE, NOT NULL |
| city | VARCHAR(100) | |
| region | VARCHAR(100) | |
| country | VARCHAR(100) | |
| geom | GEOGRAPHY(POINT, 4326) | PostGIS point |
| fuzzed_geom | GEOGRAPHY(POINT, 4326) | Pre-computed fuzzed point for discovery |
| updated_at | TIMESTAMPTZ | NOT NULL |

**Spatial index:** `GIST (fuzzed_geom)` for discovery queries

#### `location_settings`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, UNIQUE, NOT NULL |
| sharing_mode | VARCHAR(30) | NOT NULL DEFAULT 'hidden' |
| discovery_enabled | BOOLEAN | DEFAULT false |
| live_sharing_enabled | BOOLEAN | DEFAULT false |
| live_sharing_scope | VARCHAR(30) | connections, events, none |
| show_on_map | BOOLEAN | DEFAULT false |
| updated_at | TIMESTAMPTZ | NOT NULL |

**sharing_mode values:**
- `hidden` — no location exposed (default)
- `city_only` — city/region text only
- `approximate` — fuzzed coordinates for discovery
- `discoverable` — visible in nearby search with approximate coords

#### `events`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| creator_id | UUID | FK → users, NOT NULL |
| title | VARCHAR(200) | NOT NULL |
| description | TEXT | |
| image_file_id | UUID | FK → files, NULL |
| category | VARCHAR(50) | |
| event_date | DATE | NOT NULL |
| start_time | TIME | NOT NULL |
| end_time | TIME | |
| location_name | VARCHAR(200) | |
| geom | GEOGRAPHY(POINT, 4326) | Event location |
| max_participants | INT | CHECK (max_participants > 0) |
| visibility | VARCHAR(20) | public, private, connections_only |
| status | VARCHAR(20) | draft, published, cancelled, completed |
| live_location_enabled | BOOLEAN | DEFAULT false |
| created_at | TIMESTAMPTZ | NOT NULL |
| updated_at | TIMESTAMPTZ | NOT NULL |

**Spatial index:** `GIST (geom)`

#### `event_join_requests`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| event_id | UUID | FK → events, NOT NULL |
| user_id | UUID | FK → users, NOT NULL |
| status | VARCHAR(20) | pending, accepted, rejected, cancelled |
| message | TEXT | |
| responded_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | NOT NULL |
| | | UNIQUE (event_id, user_id) |

#### `event_members`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| event_id | UUID | FK → events, NOT NULL |
| user_id | UUID | FK → users, NOT NULL |
| role | VARCHAR(20) | member, co_host |
| joined_at | TIMESTAMPTZ | NOT NULL |
| | | UNIQUE (event_id, user_id) |

#### `event_live_consent`

Per-event live location consent.

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| event_id | UUID | FK → events, NOT NULL |
| user_id | UUID | FK → users, NOT NULL |
| consented | BOOLEAN | NOT NULL DEFAULT false |
| consented_at | TIMESTAMPTZ | |
| revoked_at | TIMESTAMPTZ | |
| | | UNIQUE (event_id, user_id) |

#### `conversations`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| connection_id | UUID | FK → connections, UNIQUE, NOT NULL |
| created_at | TIMESTAMPTZ | NOT NULL |
| updated_at | TIMESTAMPTZ | NOT NULL |

One conversation per accepted connection.

#### `conversation_members`

| Column | Type | Constraints |
|--------|------|-------------|
| conversation_id | UUID | FK → conversations |
| user_id | UUID | FK → users |
| last_read_at | TIMESTAMPTZ | |
| | | PRIMARY KEY (conversation_id, user_id) |

#### `messages`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| conversation_id | UUID | FK → conversations, NOT NULL |
| sender_id | UUID | FK → users, NOT NULL |
| content | TEXT | NOT NULL |
| content_type | VARCHAR(20) | text, image, file |
| created_at | TIMESTAMPTZ | NOT NULL |
| edited_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ | |

**Indexes:** `(conversation_id, created_at DESC)` for pagination

#### `notifications`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| type | VARCHAR(50) | NOT NULL |
| title | VARCHAR(200) | NOT NULL |
| body | TEXT | |
| data | JSONB | Entity references |
| read_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | NOT NULL |

**Indexes:** `(user_id, created_at DESC)`, `(user_id)` WHERE read_at IS NULL

#### `blocks`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| blocker_id | UUID | FK → users, NOT NULL |
| blocked_id | UUID | FK → users, NOT NULL |
| created_at | TIMESTAMPTZ | NOT NULL |
| | | UNIQUE (blocker_id, blocked_id) |
| | | CHECK (blocker_id != blocked_id) |

#### `reports`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| reporter_id | UUID | FK → users, NOT NULL |
| reported_user_id | UUID | FK → users, NULL |
| reported_event_id | UUID | FK → events, NULL |
| reported_message_id | UUID | FK → messages, NULL |
| reason | VARCHAR(50) | NOT NULL |
| description | TEXT | |
| status | VARCHAR(20) | pending, reviewed, resolved, dismissed |
| created_at | TIMESTAMPTZ | NOT NULL |

#### `files`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, NOT NULL |
| bucket | VARCHAR(50) | NOT NULL |
| storage_path | VARCHAR(500) | NOT NULL |
| original_filename | VARCHAR(255) | |
| mime_type | VARCHAR(100) | NOT NULL |
| size_bytes | BIGINT | NOT NULL |
| purpose | VARCHAR(30) | profile_photo, cover_photo, event_image, document |
| created_at | TIMESTAMPTZ | NOT NULL |

#### `audit_logs`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users, NULL |
| action | VARCHAR(50) | NOT NULL |
| entity_type | VARCHAR(50) | |
| entity_id | UUID | |
| ip_address | INET | |
| metadata | JSONB | |
| created_at | TIMESTAMPTZ | NOT NULL |

**Indexes:** `(user_id, created_at DESC)`, `(action, created_at DESC)`

#### `email_verification_tokens` / `password_reset_tokens`

| Column | Type | Constraints |
|--------|------|-------------|
| id | UUID | PK |
| user_id | UUID | FK → users |
| token_hash | VARCHAR(64) | UNIQUE |
| expires_at | TIMESTAMPTZ | NOT NULL |
| used_at | TIMESTAMPTZ | NULL |
| created_at | TIMESTAMPTZ | NOT NULL |

### 5.3 PostGIS Discovery Query Pattern

```sql
-- Find discoverable users within radius, filtered by profession and tags
SELECT
    u.id,
    p.full_name,
    p.profile_photo_file_id,
    pr.name AS profession,
    ST_Distance(ul.fuzzed_geom, ST_MakePoint($lng, $lat)::geography) AS distance_m,
    p.availability_status
FROM users u
JOIN profiles p ON p.user_id = u.id
JOIN user_locations ul ON ul.user_id = u.id
JOIN location_settings ls ON ls.user_id = u.id
LEFT JOIN professions pr ON pr.id = p.profession_id
WHERE u.deleted_at IS NULL
  AND u.id != $viewer_id
  AND ls.discovery_enabled = true
  AND ls.sharing_mode IN ('approximate', 'discoverable')
  AND ST_DWithin(ul.fuzzed_geom, ST_MakePoint($lng, $lat)::geography, $radius_m)
  AND NOT EXISTS (
      SELECT 1 FROM blocks b
      WHERE (b.blocker_id = u.id AND b.blocked_id = $viewer_id)
         OR (b.blocker_id = $viewer_id AND b.blocked_id = u.id)
  )
  -- Optional profession filter
  AND ($profession_id IS NULL OR p.profession_id = $profession_id)
ORDER BY distance_m
LIMIT $limit OFFSET $offset;
```

### 5.4 Location Fuzzing Strategy

When a user saves their location:
1. Store exact `geom` (never returned to unauthorized users)
2. Compute `fuzzed_geom` by adding random offset of 200–500m
3. Discovery queries use `fuzzed_geom` only
4. Re-fuzz on each location update (prevents triangulation)

---

## 6. Security Model

### 6.1 Authentication Flow

```mermaid
sequenceDiagram
    participant B as Browser
    participant S as Server
    participant DB as PostgreSQL

    B->>S: POST /auth/register (email, password, username)
    S->>S: Validate input, hash password (Argon2id)
    S->>DB: Insert user + send verification email
    S->>B: Redirect to "check email"

    B->>S: GET /auth/verify?token=xxx
    S->>DB: Verify token, mark email verified
    S->>B: Redirect to login

    B->>S: POST /auth/login (email, password)
    S->>DB: Lookup user, verify hash
    alt Failed
        S->>DB: Increment failed_login_attempts
        S->>B: Generic error (no user enumeration)
    else Success
        S->>DB: Create session (token_hash, expiry)
        S->>B: Set-Cookie: session=xxx; HttpOnly; Secure; SameSite=Lax
    end
```

### 6.2 Session Security

| Property | Value |
|----------|-------|
| Token generation | `crypto/rand` 32 bytes → base64url |
| Storage | SHA-256 hash in DB (never store raw token) |
| Cookie flags | HttpOnly, Secure (prod), SameSite=Lax |
| Expiry | 7 days, sliding window on activity |
| Invalidation | Set `revoked_at` on logout; delete expired via cron |
| Concurrent sessions | Allowed (multiple devices) |

### 6.3 CSRF Protection

- All mutating form requests (POST/PATCH/DELETE) require CSRF token
- Token stored in session, rendered in forms via template helper
- HTMX requests include token via `hx-headers` or hidden field
- Double-submit cookie pattern NOT used (session-based token preferred)

### 6.4 Rate Limiting

| Endpoint Group | Limit | Window |
|----------------|-------|--------|
| `/auth/login` | 5 attempts | 15 min per IP + email |
| `/auth/register` | 3 attempts | 1 hour per IP |
| `/auth/forgot-password` | 3 attempts | 1 hour per email |
| General API | 100 requests | 1 min per user |
| WebSocket messages | 30 messages | 1 min per user |
| File upload | 10 uploads | 1 hour per user |

Implementation: Redis sliding window counters.

### 6.5 Account Lockout

After 5 failed login attempts → lock account for 15 minutes (`locked_until`). Log security event.

### 6.6 Authorization Matrix

| Action | Rule |
|--------|------|
| Edit profile | Owner only |
| View full profile | Authenticated; blocked users see limited view |
| View exact location | Authorized per privacy rules (see §7) |
| Follow | Not self, not blocked |
| Send connection | Not self, not blocked, no duplicate pending |
| Accept connection | Recipient only, status must be pending |
| Send message | Connected + not blocked + conversation member |
| Read messages | Conversation member only |
| Create event | Authenticated user |
| Modify event | Creator only |
| Accept join request | Event creator only |
| View event live locations | Event member + target has consented + live ON |
| Upload file | Owner; validated type/size |
| Block | Not self |
| Report | Authenticated; cannot report self |

### 6.7 Input Validation

- All input validated server-side in `validator.go` per module
- Max request body: 1 MB (10 MB for file uploads)
- Username: `^[a-zA-Z0-9_]{3,30}$`
- Email: RFC 5322 simplified regex + DNS check optional
- Password: min 8 chars, complexity not enforced (NIST SP 800-63B)
- SQL: exclusively parameterized queries via pgx

### 6.8 Output Security

- Go `html/template` auto-escapes all template variables
- JSON API responses: no internal error details
- Security headers middleware:
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline' cdn.jsdelivr.net unpkg.com; style-src 'self' 'unsafe-inline' cdn.jsdelivr.net unpkg.com; img-src 'self' data: *.supabase.co *.tile.openstreetmap.org; connect-src 'self' wss:`
  - `Strict-Transport-Security` (production)
  - `Referrer-Policy: strict-origin-when-cross-origin`

### 6.9 WebSocket Authentication

1. Client connects to `GET /ws` with session cookie
2. Server validates session before upgrade
3. On success, register connection in hub with `user_id`
4. All WS messages include typed envelope; server validates sender identity from connection context, **never from message payload**

### 6.10 Audit Logging

Log (structured, no PII in message body):
- Login success/failure
- Password reset requests
- Session creation/revocation
- Account lockout
- Block/report actions
- Location sharing mode changes
- Failed authorization attempts

Do NOT log: passwords, session tokens, message content, exact coordinates.

---

## 7. Location & Privacy Model

### 7.1 Location Tiers

```mermaid
flowchart TD
    A[User Location Data] --> B{Privacy Setting}
    B -->|hidden| C[Nothing exposed]
    B -->|city_only| D[City/region text only]
    B -->|approximate| E[Fuzzed coords ~200-500m<br/>Discovery only]
    B -->|discoverable| F[Fuzzed coords<br/>Visible in nearby search]
    
    G[Live Location] --> H{Scope + Consent}
    H -->|connections + enabled| I[Exact coords to connections]
    H -->|event + consented| J[Exact coords to event members]
    H -->|disabled| K[Not in Redis, not broadcast]
```

### 7.2 Authorization Rules for Location

| Viewer Context | What They See |
|----------------|---------------|
| Unauthenticated | City name only (if profile is public) |
| Random authenticated user | City name OR fuzzed coords (if discoverable) |
| Follower | Same as above (follow ≠ location access) |
| Connection (accepted) | Fuzzed OR exact if live sharing to connections |
| Event member (consented) | Exact live coords during event window |
| Self | Full exact location |

### 7.3 Live Location Lifecycle

```
User enables live sharing
    ↓
WebSocket: { type: "location_update", lat, lng, timestamp }
    ↓
Server validates auth + consent + scope
    ↓
Redis SET event:{eventID}:user:{userID} {lat, lng, ts} EX 120
    ↓
Pub/Sub broadcast to authorized subscribers
    ↓
User disables OR TTL expires OR event ends
    ↓
Redis DEL key
    ↓
Pub/Sub: { type: "location_removed", userID }
```

**Stale detection:** Reject/display locations with timestamp > 60 seconds old.

### 7.4 Default Privacy (Conservative)

New users default to:
- `sharing_mode: hidden`
- `discovery_enabled: false`
- `live_sharing_enabled: false`

User must explicitly opt in to each level.

---

## 8. REST API Design

### 8.1 Route Map

#### Public (No Auth)

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/` | Landing page |
| GET | `/auth/login` | Login form |
| POST | `/auth/login` | Login action |
| GET | `/auth/register` | Registration form |
| POST | `/auth/register` | Register action |
| GET | `/auth/verify` | Email verification |
| GET | `/auth/forgot-password` | Forgot password form |
| POST | `/auth/forgot-password` | Send reset email |
| GET | `/auth/reset-password` | Reset form |
| POST | `/auth/reset-password` | Reset action |

#### Authenticated

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/auth/logout` | Logout |
| GET | `/onboarding` | Onboarding wizard |
| POST | `/onboarding` | Complete onboarding |

#### Profile

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/profile` | Own profile |
| GET | `/profile/edit` | Edit form |
| PATCH | `/profile` | Update profile |
| POST | `/profile/photo` | Upload profile photo |
| POST | `/profile/cover` | Upload cover photo |
| GET | `/users/:username` | Public profile view |

#### Social

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/users/:id/follow` | Follow user |
| DELETE | `/users/:id/follow` | Unfollow |
| GET | `/users/:id/followers` | Followers list |
| GET | `/users/:id/following` | Following list |

#### Connections

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/users/:id/connect` | Send connection request |
| GET | `/connections` | List connections |
| GET | `/connections/requests` | Pending requests |
| POST | `/connections/:id/accept` | Accept request |
| POST | `/connections/:id/reject` | Reject request |
| DELETE | `/connections/:id` | Cancel/remove connection |

#### Discovery

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/discover` | Discovery page (map + list) |
| GET | `/api/discover/nearby` | JSON nearby users (HTMX partial) |

Query params: `lat`, `lng`, `radius_km`, `profession`, `tags[]`, `skills[]`, `availability`, `looking_for`, `verified`, `page`, `limit`

#### Location

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/settings/location` | Location settings |
| PATCH | `/settings/location` | Update location settings |
| POST | `/settings/location/update` | Update general location |

#### Events

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/events` | Event listing |
| GET | `/events/new` | Create event form |
| POST | `/events` | Create event |
| GET | `/events/:id` | Event detail |
| GET | `/events/:id/edit` | Edit form |
| PATCH | `/events/:id` | Update event |
| DELETE | `/events/:id` | Cancel event |
| POST | `/events/:id/join` | Request to join |
| POST | `/events/:id/requests/:reqId/accept` | Accept join request |
| POST | `/events/:id/requests/:reqId/reject` | Reject join request |
| GET | `/events/:id/members` | Event members |
| GET | `/events/:id/map` | Event map view |
| POST | `/events/:id/live-consent` | Toggle live location consent |

#### Chat

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/messages` | Conversation list |
| GET | `/messages/:id` | Conversation view |
| GET | `/api/conversations/:id/messages` | Messages (paginated, HTMX) |
| POST | `/api/conversations/:id/messages` | Send message (also via WS) |

#### Notifications

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/notifications` | Notification list |
| POST | `/notifications/:id/read` | Mark as read |
| POST | `/notifications/read-all` | Mark all read |
| GET | `/api/notifications/unread-count` | Badge count (HTMX poll) |

#### Block & Report

| Method | Route | Description |
|--------|-------|-------------|
| POST | `/users/:id/block` | Block user |
| DELETE | `/users/:id/block` | Unblock |
| POST | `/reports` | Submit report |

#### Settings

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/settings` | Settings hub |
| GET | `/settings/privacy` | Privacy settings |
| PATCH | `/settings/privacy` | Update privacy |
| GET | `/settings/account` | Account settings |
| DELETE | `/settings/account` | Delete account |

### 8.2 Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Username is already taken",
    "fields": {
      "username": "already taken"
    }
  }
}
```

HTTP status codes used consistently:
- 400 — validation error
- 401 — not authenticated
- 403 — not authorized
- 404 — not found
- 409 — conflict (duplicate follow, etc.)
- 422 — business rule violation
- 429 — rate limited
- 500 — internal (generic message only)

### 8.3 Pagination

Cursor-based for messages and notifications:

```
GET /api/conversations/:id/messages?before=2026-08-22T10:00:00Z&limit=50
```

Offset-based for discovery (with max limit cap of 50):

```
GET /api/discover/nearby?page=1&limit=20
```

---

## 9. WebSocket Architecture

### 9.1 Connection Lifecycle

```mermaid
sequenceDiagram
    participant C as Client
    participant H as WS Handler
    participant Hub as Connection Hub
    participant R as Redis Pub/Sub
    participant S as Services

    C->>H: GET /ws (cookie: session)
    H->>S: Validate session
    alt Invalid
        H->>C: 401 Unauthorized
    else Valid
        H->>Hub: Register(userID, conn)
        Hub->>R: SUBSCRIBE user:{userID}
        H->>C: 101 Switching Protocols
        
        loop Message Loop
            C->>H: JSON envelope
            H->>S: Route + authorize
            S->>R: PUBLISH channel
            R->>Hub: Deliver to subscribers
            Hub->>C: Forward message
        end
        
        C->>H: Disconnect
        H->>Hub: Unregister(userID, conn)
        Hub->>R: Update presence
    end
```

### 9.2 Message Envelope

All WebSocket messages use a typed envelope:

```json
{
  "type": "message|location_update|typing|presence|notification|location_removed|error",
  "payload": { ... },
  "timestamp": "2026-08-22T10:00:00Z"
}
```

### 9.3 Message Types

| Type | Direction | Description |
|------|-----------|-------------|
| `message` | Client → Server → Client | Chat message |
| `typing` | Client → Server → Client | Typing indicator |
| `location_update` | Client → Server → Subscribers | Live GPS update |
| `location_removed` | Server → Client | Live sharing stopped |
| `presence` | Server → Client | Online/offline status |
| `notification` | Server → Client | Real-time notification |
| `subscribe_event` | Client → Server | Subscribe to event location channel |
| `unsubscribe_event` | Client → Server | Unsubscribe from event |
| `ping` | Client → Server | Keepalive |
| `pong` | Server → Client | Keepalive response |
| `error` | Server → Client | Error message |

### 9.4 Hub Design

```go
type Hub struct {
    clients    map[uuid.UUID]map[*Client]bool  // userID → connections
    register   chan *Client
    unregister chan *Client
    broadcast  chan *Envelope
    redis      *redis.Client
    logger     *logger.Logger
}
```

- One user may have multiple connections (tabs/devices)
- Broadcast sends to all connections for target user(s)
- Redis Pub/Sub enables multi-instance deployment (horizontal scaling)

### 9.5 Redis Channels

| Channel Pattern | Purpose |
|-----------------|---------|
| `user:{userID}` | Personal notifications, chat |
| `conversation:{convID}` | Chat messages |
| `event:{eventID}:location` | Event live location updates |
| `presence` | Global presence changes |

### 9.6 Authorization in WebSocket

Every incoming message is authorized server-side:

```go
func (h *Handler) handleLocationUpdate(client *Client, payload LocationPayload) error {
    // 1. Verify client.UserID matches (from session, not payload)
    // 2. Check location_settings.live_sharing_enabled
    // 3. If event scope: verify event membership + consent
    // 4. If connections scope: verify at least one connection exists
    // 5. Write to Redis with TTL
    // 6. Publish to authorized channel only
}
```

---

## 10. Real-Time & Redis Design

### 10.1 Redis Key Schema

| Key Pattern | Type | TTL | Purpose |
|-------------|------|-----|---------|
| `session:ratelimit:{ip}:{endpoint}` | String (counter) | 15m | Rate limiting |
| `presence:{userID}` | String (online/offline) | 5m | Presence (refreshed by ping) |
| `live:{userID}` | Hash (lat, lng, ts) | 120s | General live location |
| `event:{eventID}:user:{userID}` | Hash (lat, lng, ts) | 120s | Event live location |
| `typing:{convID}:{userID}` | String | 5s | Typing indicator |
| `ws:connections:{instanceID}` | Set | None | Instance connection tracking |

### 10.2 TTL Strategy

- Live location keys expire after 120 seconds (must be refreshed by client every 30s)
- Presence keys expire after 5 minutes (refreshed by WebSocket ping every 30s)
- On disconnect: immediately remove presence, stop location broadcasting

### 10.3 Multi-Instance Considerations

For future horizontal scaling:
- Redis Pub/Sub bridges WebSocket hubs across instances
- Sticky sessions NOT required (session validated from cookie on each WS connection)
- Each instance subscribes to relevant Redis channels on startup

---

## 11. File Storage Design

### 11.1 Supabase Storage Buckets

| Bucket | Access | Purpose |
|--------|--------|---------|
| `avatars` | Public (via signed URL) | Profile photos |
| `covers` | Public (via signed URL) | Cover photos |
| `events` | Public (via signed URL) | Event images |
| `documents` | Private (signed URL only) | Optional documents |

### 11.2 Upload Flow

```
Client POST /profile/photo (multipart)
    ↓
Controller: parse multipart, size check
    ↓
Service: validate MIME (magic bytes), extension, size
    ↓
Service: generate UUID filename (never trust client name)
    ↓
Storage repo: upload to Supabase Storage API
    ↓
DB: insert files record with metadata
    ↓
Profile repo: update profile_photo_file_id
    ↓
Delete old file from storage (if exists)
```

### 11.3 Validation Rules

| Check | Rule |
|-------|------|
| Max size (avatar) | 5 MB |
| Max size (cover) | 10 MB |
| Max size (event) | 10 MB |
| Allowed MIME | image/jpeg, image/png, image/webp |
| Magic bytes | Verify file header matches declared MIME |
| Extension | Must match MIME (.jpg, .png, .webp) |
| Filename | Generated UUID + extension |

---

## 12. Map Integration

### 12.1 OpenStreetMap + Leaflet (no API key)

Maps use **Leaflet** with **OpenStreetMap** raster tiles (`https://{s}.tile.openstreetmap.org/...`). No Mapbox/Google key or server-side map SDK is required.

Optional geocoding (Nominatim or similar) can be added later if needed; discovery and the public homepage do not need it.

### 12.2 Client-Side Map

- Leaflet loaded from CDN on landing, discovery, and event map pages
- OSM tile layer — no access token
- Markers rendered from server-provided data (never raw coords for unauthorized users)
- Different marker types: approximate (circle), live (pin), event (flag)

### 12.3 Data Sent to Map

Server returns privacy-filtered marker data:

```json
{
  "markers": [
    {
      "id": "uuid",
      "type": "approximate",
      "lat": 23.8103,
      "lng": 90.4125,
      "label": "Software Engineer",
      "avatar_url": "/files/xxx/thumb"
    }
  ]
}
```

Exact coordinates only included when authorization permits.

---

## 13. Frontend Architecture

### 13.1 Template Structure

```
templates/
├── layouts/
│   ├── base.html          # HTML shell, CSS/JS includes
│   ├── auth.html          # Centered auth layout
│   └── app.html           # App layout with nav
├── partials/
│   ├── nav.html
│   ├── flash.html
│   ├── csrf.html
│   └── notification_badge.html
├── auth/
│   ├── login.html
│   ├── register.html
│   └── forgot_password.html
├── profile/
│   ├── show.html
│   ├── edit.html
│   └── onboarding.html
├── discovery/
│   ├── index.html
│   └── _nearby_list.html  # HTMX partial
├── events/
│   ├── index.html
│   ├── show.html
│   ├── new.html
│   └── map.html
├── chat/
│   ├── index.html
│   ├── show.html
│   └── _messages.html     # HTMX partial
└── settings/
    ├── index.html
    ├── location.html
    └── privacy.html
```

### 13.2 HTMX Patterns

- Discovery list: `hx-get="/api/discover/nearby"` with filter params
- Chat messages: `hx-get="/api/conversations/:id/messages"` polling + WS for real-time
- Notifications: `hx-get="/api/notifications/unread-count"` every 30s
- Form submissions: `hx-post` with CSRF token, `hx-swap="outerHTML"` for inline errors

### 13.3 Alpine.js Usage

- Mobile nav toggle
- Filter panel expand/collapse
- Modal dialogs (block, report)
- WebSocket connection management
- Map interaction state

### 13.4 Static Assets

```
static/
├── css/
│   └── app.css            # Tailwind compiled output
├── js/
│   ├── app.js             # WebSocket client, utilities
│   ├── map-home.js        # Public homepage Leaflet + OSM
│   └── discover.js        # Discovery map Leaflet + OSM
└── images/
    └── default-avatar.png
```

Tailwind built at Docker build time or via `tailwindcss` CLI in dev.

---

## 14. Observability

### 14.1 Structured Logging

Using Go `log/slog` with JSON output in production:

```go
logger.Info("user logged in",
    "user_id", userID,
    "ip", ip,
    "request_id", requestID,
)
```

### 14.2 Request ID

Middleware generates UUID per request, propagated through context, included in all logs and error responses.

### 14.3 Metrics (Future-Ready)

Expose `/metrics` endpoint (disabled by default) compatible with Prometheus:
- HTTP request duration histogram
- Active WebSocket connections gauge
- Database pool stats
- Redis operation counters

### 14.4 Health Checks

| Endpoint | Checks |
|----------|--------|
| `GET /healthz` | Server alive |
| `GET /readyz` | PostgreSQL + Redis connectivity |

---

## 15. Deployment Architecture

### 15.1 Docker Compose (Development)

```yaml
services:
  app:
    build: .
    ports: ["8080:8080"]
    env_file: .env
    depends_on: [postgres, redis]
  
  postgres:
    image: postgis/postgis:16-3.4
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql/data]
  
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
```

Production uses Supabase PostgreSQL (remote) — local PostGIS container for dev only.

### 15.2 Production Deployment

```
                    ┌─────────────┐
                    │   CDN/DNS   │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  Railway /  │
                    │  Fly.io /   │
                    │  Render     │
                    │  (Go app)   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────▼──┐  ┌─────▼─────┐
       │Supabase │  │  Redis    │
       │PG+Store │  │ (Upstash) │
       └─────────┘  └───────────┘
       (maps: OpenStreetMap tiles in the browser — no map API service)
```

### 15.3 Environment Variables

```env
# App
APP_ENV=development|production
APP_PORT=8080
APP_URL=http://localhost:8080
APP_SECRET=<random-64-chars>

# Database
DATABASE_URL=postgresql://user:pass@host:5432/findneighbour?sslmode=require

# Redis
REDIS_URL=redis://localhost:6379

# Supabase Storage
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=xxx
SUPABASE_STORAGE_BUCKET=avatars

# Maps: OpenStreetMap via Leaflet — no API key

# Email (SMTP or Resend/SendGrid)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=xxx
SMTP_PASS=xxx
SMTP_FROM=noreply@findneighbour.app

# Security
SESSION_DURATION=168h
CSRF_SECRET=<random-32-chars>
ARGON2_MEMORY=65536
ARGON2_ITERATIONS=3
ARGON2_PARALLELISM=4
```

---

## 16. Implementation Roadmap

### Milestone 1: Project Foundation
**Deliverables:**
- Go module, directory structure
- Config loading (env + validation)
- Structured logging with request IDs
- PostgreSQL connection pool (pgx)
- Redis client
- Gin router with middleware skeleton
- Docker + Docker Compose
- Health check endpoints
- `.env.example`, README

**Tests:** Config validation, health check endpoints

---

### Milestone 2: Database Schema
**Deliverables:**
- All migration files (001–015)
- PostGIS extension enablement
- Seed data (professions, system tags)
- Migration runner in app startup
- Database package with transaction helper

**Tests:** Migration up/down, constraint verification

---

### Milestone 3: Authentication
**Deliverables:**
- User registration with email verification
- Login/logout with session cookies
- Password reset flow
- Argon2id password hashing
- CSRF middleware
- Rate limiting on auth endpoints
- Account lockout
- Auth middleware
- Login/register/forgot-password templates

**Tests:** Registration, login, logout, lockout, CSRF rejection, rate limit, password hash verification

---

### Milestone 4: Profile System
**Deliverables:**
- Profile CRUD
- Profession selection
- Tags (add/remove, custom tags)
- Skills (add/remove)
- Social links (flexible platforms)
- Profile/cover photo upload (Supabase Storage)
- Onboarding wizard
- Profile view/edit templates

**Tests:** Profile CRUD, authorization (can't edit others), file upload validation, tag/skill uniqueness

---

### Milestone 5: Follow System
**Deliverables:**
- Follow/unfollow
- Followers/following lists
- Follow counts on profile
- Block check integration

**Tests:** Follow/unfollow, self-follow prevention, duplicate prevention, blocked user prevention

---

### Milestone 6: Connection System
**Deliverables:**
- Send connection request
- Accept/reject/cancel
- Connection list
- Pending requests view
- State machine enforcement
- Notification on request/accept/reject
- Transactional accept (creates conversation)

**Tests:** Full state machine, duplicate prevention, unauthorized accept, race conditions, block prevention

---

### Milestone 7: Discovery & Map
**Deliverables:**
- PostGIS nearby query service
- Discovery page with filters
- OpenStreetMap (Leaflet) map integration
- HTMX-powered filter/list updates
- Privacy-safe marker data
- Approximate location display

**Tests:** Discovery queries, filter combinations, privacy filtering, distance accuracy, block exclusion

---

### Milestone 8: Location Privacy
**Deliverables:**
- Location settings CRUD
- General location storage with fuzzing
- Discovery enable/disable
- Privacy tier enforcement in all queries
- Location settings UI

**Tests:** Each privacy tier, fuzzing verification, unauthorized exact coord access, default conservative settings

---

### Milestone 9: WebSocket Infrastructure
**Deliverables:**
- WebSocket handler with session auth
- Connection hub
- Redis Pub/Sub integration
- Presence tracking
- Live location Redis storage with TTL
- Client-side WS manager (app.js)

**Tests:** WS auth rejection, presence tracking, location TTL expiry, multi-connection handling

---

### Milestone 10: Chat System
**Deliverables:**
- Conversation creation (on connection accept)
- Message send/receive (HTTP + WS)
- Message persistence
- Unread count tracking
- Typing indicators
- Chat UI templates
- Real-time message delivery

**Tests:** Connection-only messaging, unauthorized access, block prevention, unread counts, message pagination

---

### Milestone 11: Events
**Deliverables:**
- Event CRUD
- Join request flow
- Accept/reject with capacity check (transactional)
- Event members management
- Event listing with filters
- Event detail/create templates
- Notifications for event actions

**Tests:** CRUD authorization, capacity race condition, duplicate join prevention, visibility rules, cancelled event rejection

---

### Milestone 12: Event Map & Live Location
**Deliverables:**
- Event map page
- Per-event live location consent
- Live location broadcasting (WS + Redis)
- Authorized participant markers
- Auto-expire on event end
- Stale location detection

**Tests:** Consent required, non-member cannot see live, creator doesn't auto-get location, TTL expiry, event end cleanup

---

### Milestone 13: Block, Report & Hardening
**Deliverables:**
- Block/unblock with cascade effects
- Report user/event/message
- Privacy settings page
- Security header audit
- Input validation audit
- Audit logging for security events

**Tests:** Block cascade (no follow, connect, message, location), report storage, full security test suite

---

### Milestone 14: Testing & Performance
**Deliverables:**
- Integration test suite (testcontainers)
- Security test suite
- Load test scripts (discovery query, WS connections)
- Query optimization review
- N+1 query elimination

**Tests:** Full end-to-end user journey test

---

### Milestone 15: Production & Documentation
**Deliverables:**
- Production Dockerfile (multi-stage)
- Docker Compose production override
- Deployment guide (Railway/Fly.io)
- API documentation
- Architecture decision records (ADRs)
- README with setup instructions

---

## 17. Assumptions & Open Questions

### Assumptions Made

| # | Assumption | Rationale |
|---|-----------|-----------|
| 1 | Custom auth (not Supabase Auth) | Full session control for server-rendered app |
| 2 | Email via SMTP (not Supabase) | Portable, any provider works |
| 3 | One conversation per connection | Simplicity; group chat deferred |
| 4 | Username-based public URLs | `/users/:username` instead of UUID |
| 5 | Re-request cooldown after rejection | 7 days before new connection request to same user |
| 6 | Event auto-completes | Status → completed day after event_date |
| 7 | Max 500m fuzz radius | Balance privacy vs. utility |
| 8 | No push notifications initially | In-app + WebSocket only; email for critical |
| 9 | Single region deployment initially | Multi-region deferred |
| 10 | No admin panel initially | Reports stored for future moderation UI |

### Open Questions for Review

1. **Email provider preference?** SMTP, Resend, SendGrid, or other?
2. **Verification required before login?** Or allow login with unverified email (limited features)?
3. **Connection re-request policy?** Allow immediately, cooldown period, or one-shot?
4. **Event capacity default?** Fixed default (e.g., 20) or required on creation?
5. **Maximum tags/skills per user?** Suggested: 20 tags, 30 skills?
6. **Profile visibility?** Public by default or connections-only option?
7. **Map tiles?** OpenStreetMap via Leaflet (done); switch tile provider only if rate limits become an issue.
8. **GDPR/data export?** Required for launch or post-MVP?
9. **Moderation workflow?** Manual review of reports or automated actions?
10. **Mobile app future?** API design accommodates it, but confirm no native app in v1?

---

## Next Steps

Once this architecture is reviewed and approved:

1. Begin **Milestone 1** — project scaffolding, Docker, config, logging
2. Each subsequent milestone builds on the previous with tests
3. Architecture document updated as decisions evolve (ADRs in `docs/adr/`)

**Please review and provide feedback on:**
- Database schema completeness
- Security model adequacy
- Location privacy tiers
- API route design
- Open questions above
