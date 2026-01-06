# Regis Database Documentation

Regis uses an embedded **SQLite** database (`regis.db`) to track job history, status, and configuration.

## 1. Schema Overview

The database allows the API to persist state across restarts and manage the job queue.
It primarily consists of a single `jobs` table, plus internal tables for the `goqite` persistent queue.

### Table: `jobs`

| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `TEXT` (PK) | Unique UUIDv4 for the job. |
| `job_name` | `TEXT` | User-provided friendly name (optional). |
| `user_email` | `TEXT` | User email (indexed for fast filtering). |
| `status` | `TEXT` | Current state: `queued`, `running`, `completed`, `failed`, `submitted` (slurm). |
| `config` | `JSON` | Full JSON payload of the job configuration. |
| `start_time` | `DATETIME` | Timestamp when processing started. |
| `end_time` | `DATETIME` | Timestamp when processing finished. |
| `external_id`| `TEXT` | External scheduler ID (e.g. Slurm Job ID). |
| `error` | `TEXT` | Error message if the job failed. |

### Table: `goqite` (Queue)
Internal table used by the message queue system. **Do not modify manually.**

---

## 2. Inspecting the Database

You can inspect the database using the `sqlite3` command-line tool or any SQLite GUI (like DB Browser for SQLite).

### Connect
```bash
sqlite3 regis.db
```

### Common Commands

**List all jobs:**
```sql
SELECT id, status, job_name, start_time FROM jobs ORDER BY start_time DESC;
```

**Find failed jobs:**
```sql
SELECT id, error FROM jobs WHERE status = 'failed';
```

**Count jobs by status:**
```sql
SELECT status, COUNT(*) FROM jobs GROUP BY status;
```

**View JSON Config for a specific job:**
```sql
SELECT json_extract(config, '$') FROM jobs WHERE id = 'YOUR_UUID';
```

---

## 3. Operations

### Resetting the Database
If you are in a development environment and want to clear all history:
1. Stop the `regis serve` process.
2. Delete the `regis.db` file.
3. Restart `regis serve`. The DB will be automatically recreated.

```bash
rm regis.db
```
