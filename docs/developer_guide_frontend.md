# Regis Frontend Developer Guide

This guide documents the Backend API, Data Models, and workflows to assist in building a modern frontend for **Regis** using **TanStack Start, Vite, Shadcn UI, and TanStack Query**.

## 1. API Overview

- **Base URL**: `/api/v1`
- **Authentication**: All protected routes require an API Key.
- **Content-Type**: `application/json`

### Authentication
The server generates an API Key on startup if one is not provided in `config.yaml`.
You must include this key in the header of all requests to `/jobs/*`.

**Header:**
```http
X-API-Key: <YOUR_API_KEY>
```

---

## 2. API Reference

### A. Health Check
**Endpoint:** `GET /api/v1/health`

**Description:** Simple ping to verify backend connectivity. Does **not** require API Key.

**Response (200 OK):**
```json
{
  "status": "ok",
  "message": "Regis API is running"
}
```

---

### B. Submit a Job
**Endpoint:** `POST /api/v1/jobs/submit`

**Description:** Submits a new pipeline job. The payload is the `Config` object.

### C. Config Payload Reference (Full)
*(See table below)*

| Field | Type | Required? | Description |
| :--- | :--- | :--- | :--- |
| **Core settings** | | | |
| `email` | `string` | **Yes** | User email for job tracking. |
| `data_type` | `string` | **Yes** | `single` or `paired`. |
| `method` | `string` | **Yes** | `denovo` (Assemble from reads) or `reference` (Use genome/GTF). |
| `execution_mode`| `string` | No | `local` (default) or `slurm` (High Performance Computing). |
| `threads` | `number` | No | Number of CPU threads (default: all available). |
| **Input Files** | | | |
| `file1` | `string` | **Yes** | Absolute path to Read 1 (or single-end file). |
| `file2` | `string` | Conditional | Absolute path to Read 2 (Required if `data_type="paired"`). |
| `reference` | `string` | Conditional | Absolute path to Genome FASTA (Required if `method="reference"`). |
| `gtf` | `string` | Conditional | Absolute path to Annotation GTF (Required if `method="reference"`). |
| **Filtering & Validation** | | | |
| `species` | `string` | No | Species for CPAT coding potential model. Options: `Human`, `Mouse`, `Fly`, `Zebrafish` (Default: `Human`). |
| `skip_cpat` | `boolean` | No | If `true`, skips CPAT and uses only CPC2 for validation (faster but less rigorous). |
| `enable_sortmerna`| `boolean`| No | If `true`, filters out ribosomal RNA (rRNA) before assembly. Highly recommended. |
| **Target Prediction (LncTar)** | | | |
| `enable_lnctar` | `boolean` | No | Enable LncTar tool for lncRNA-mRNA interaction prediction. |
| `lnctar_best_only`| `boolean`| No | If `true`, reports only the *single best* target for each lncRNA. |
| `lnctar_highly` | `boolean` | No | If `true`, runs LncTar only on *highly expressed* lncRNAs (TPM > 1.0). |
| `lnctar_comprehensive`|`boolean`| No | If `true`, runs on *all* candidates (Computationally expensive!). |
| **Target Prediction (IntaRNA)** | | | |
| `enable_intarna` | `boolean` | No | Enable IntaRNA tool (more accurate thermodynamics). |
| `intarna_best_only`| `boolean`| No | If `true`, reports only the *single best* target. |
| `intarna_highly` | `boolean` | No | If `true`, runs IntaRNA only on *highly expressed* lncRNAs. |
| `intarna_comprehensive`|`boolean`| No | If `true`, runs on *all* candidates (Very slow!). |

> [!TIP]
> **Validation Logic**:
> - You cannot enable `lnctar_best_only` AND `lnctar_comprehensive` at the same time.
> - `method="reference"` requires `gtf` and `reference`. `method="denovo"` ignores them.
> - `species` defaults to "Human" if not provided, but it's good UX to let the user choose.

**Request Body (JSON Example with All Flags):**
```json
{
  "email": "user@example.com",
  "data_type": "paired",
  "method": "denovo",
  "file1": "/abs/data/R1.fq",
  "file2": "/abs/data/R2.fq",
  "threads": 16,
  "execution_mode": "local",
  "enable_sortmerna": true,
  "species": "Mouse",
  "enable_lnctar": true,
  "lnctar_highly": true,
  "enable_intarna": true,
  "intarna_best_only": true
}
```

**Response (202 Accepted):**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Job submitted successfully",
  "output_dir": "/path/to/output"
}
```

**Frontend Integration (TanStack Query):**
Use `useMutation` to submit the job.

---

### D. Get Job Status
**Endpoint:** `GET /api/v1/jobs/:uuid/status`

**Description:** Returns the current status and metadata of a job.

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "running",         // "queued" | "running" | "completed" | "failed"
  "start_time": "2023-10-27T10:00:00Z",
  "end_time": "0001-01-01T00:00:00Z", // Zero if running
  "error": "",                 // Error message if failed
  "config": { ... }            // The full config object submitted
}
```

**Frontend Integration (TanStack Query):**
Use `useQuery` with `refetchInterval` (polling) while status is `running` or `queued`.

```typescript
useQuery({
  queryKey: ['jobStatus', id],
  queryFn: () => fetchJobStatus(id),
  refetchInterval: (data) => 
    (data.status === 'completed' || data.status === 'failed') ? false : 2000
})
```

---

### E. Get Job Results (Summary)
**Endpoint:** `GET /api/v1/jobs/:uuid/results`

**Description:** Returns a lightweight summary of the completed job, primarily used to verify the output folder exists before fetching metrics or downloading.

**Response (200 OK):**
```json
{
  "job_id": "550e8400...",
  "status": "completed",
  "output_dir": "/abs/path/to/output"
}
```

---

### F. Get Job Results (Metrics)
**Endpoint:** `GET /api/v1/jobs/:uuid/results/metrics`

**Description:** Returns the content of `pipeline_summary.json` generated by the pipeline. available only when status is `completed`.

**Response (200 OK):**
```json
{
  "total_reads": 1050000,
  "mapped_reads": 980000,
  "lncrna_candidates": 145,
  "novel_lncrnas": 12,
  "steps": {
    "01_quality_control": "Pass",
    "06_cpat": "145 identified"
  }
}
```

---

### G. Server Statistics
**Endpoint:** `GET /api/v1/stats`

**Description:** Returns real-time system metrics (CPU, Memory, Uptime) and Job counts (Queued, Running, etc).

**Response (200 OK):**
```json
{
  "server": {
    "uptime_human": "2h 15m",
    "version": "1.0.5"
  },
  "jobs": {
    "running": 2,
    "queued": 5
  },
  "system": {
    "cpus": 16,
    "cpu_percent": 45.2,
    "memory_used_mb": 4096,
    "memory_total_mb": 16384,
    "memory_percent": 25.0
  }
}
```

---

### H. Download Results
**Endpoint:** `GET /api/v1/jobs/:uuid/results/download`

**Query Parameters:**
- `light=true`: (Optional) Skips large raw files (FASTQ, BAM) and downloads only reports, logs, and final sequences.

**Behavior:** Streams a `.zip` file.

**Frontend Integration:**
Create a standard `<a>` tag or `window.open` link for the download button.
`href="/api/v1/jobs/id/results/download?light=true"`

---

## 3. Data Models (TypeScript Interfaces)

*(See [Database Schema](./database_schema.md) for backend storage details)*

You can copy these into your frontend `types` folder.

```typescript
// Enums based on validation rules
export type DataType = 'single' | 'paired';
export type AnalysisMethod = 'denovo' | 'reference';
export type ExecutionMode = 'local' | 'slurm';
export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'submitted';

// The main Configuration Object
export interface PipelineConfig {
  email: string;
  data_type: DataType;
  method: AnalysisMethod;
  file1: string;
  file2?: string;
  reference?: string;
  gtf?: string;
  output_dir?: string;
  
  // Advanced Options
  threads?: number;
  species?: string;
  skip_cpat?: boolean;
  enable_sortmerna?: boolean;
  
  // Target Prediction
  enable_lnctar?: boolean;
  lnctar_best_only?: boolean;
  lnctar_highly?: boolean;
  lnctar_comprehensive?: boolean;
  
  enable_intarna?: boolean;
  intarna_best_only?: boolean;
  intarna_highly?: boolean;
  intarna_comprehensive?: boolean;
  
  // System
  execution_mode?: ExecutionMode;
  api_key?: string;
}

// The Job Object stored in DB
export interface Job {
  id: string;
  status: JobStatus;
  config: PipelineConfig;
  start_time: string; // ISO Date
  end_time: string;   // ISO Date
  error?: string;
  external_id?: string; // Slurm ID
}
```

## 4. Frontend Workflow Recommendations

1.  **Job Submission Form**:
    *   Use **React Hook Form** + **Zod** to validate the `PipelineConfig` inputs.
    *   Ensure `file1`, `reference`, etc. are valid absolute paths (or implement a file picker if running locally/electron).
    *   On Success: Redirect to `/jobs/$JOB_ID`.

2.  **Job Status Page (`/jobs/$ID`)**:
    *   Poll `GET /jobs/$ID/status` every 2-5 seconds.
    *   Show a Progress Bar or Spinner.
    *   Display "Logs" or "Current Step" if you implement a log streaming endpoint (Current implementation logs to `pipeline.log` on disk, you might want to add a `GET /jobs/$ID/logs` endpoint later).

3.  **Results Dashboard**:
    *   Once status is `completed`, switch view to "Results".
    *   Fetch metrics via `GET /jobs/$ID/results/metrics`.
    *   Use **TanStack Table** to display the lncRNA candidates if you add an endpoint to parse the final `.bed` or `.gtf` file.
    *   Show "Download Report" button linking to the download endpoint.

## 5. Development Tips

*   **Mocking**: Use the provided JSON responses to mock the API during UI development.
*   **Validation**: The backend uses strict validation. If you get a `400 Bad Request`, check the `error` field in the response.
