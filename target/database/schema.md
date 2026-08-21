# Target Database

The synthetic target currently uses SQLite via `modernc.org/sqlite` (pure Go,
no CGO). The schema is created automatically on startup.

## Tables

**sessions**

| column | type | note |
| --- | --- | --- |
| id | TEXT PK | e.g. `sess-<unixnano>` |
| created_at | TEXT | RFC3339 UTC |
| expires_at | TEXT | RFC3339 UTC |

**products**

| column | type | note |
| --- | --- | --- |
| id | TEXT PK | `p1`, `p2`, `p3` |
| name | TEXT | |
| description | TEXT | |
| price | REAL | |
| stock | INTEGER | |

**cart_items**

| column | type | note |
| --- | --- | --- |
| session_id | TEXT | PK part |
| product_id | TEXT | PK part |
| qty | INTEGER | |

**orders**

| column | type | note |
| --- | --- | --- |
| id | TEXT PK | e.g. `ord-<unixnano>` |
| session_id | TEXT | |
| status | TEXT | `confirmed` |
| total | REAL | |
| created_at | TEXT | RFC3339 UTC |

**order_items**

| column | type | note |
| --- | --- | --- |
| order_id | TEXT | PK part |
| product_id | TEXT | PK part |
| name | TEXT | denormalized snapshot |
| qty | INTEGER | |
| price | REAL | |

## PostgreSQL

A `postgres` service is available in `docker-compose.yml` behind the
`postgres` profile for later experiments. The benchmark itself does not
require it.

## Determinism

The store seeds the same three products on every fresh database, so every run
starts from an identical catalog regardless of machine or time.
