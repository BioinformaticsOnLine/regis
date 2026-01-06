# REGIS Frontend Developer Guide

This guide documents the Backend API, Data Models, and workflows to assist in building a modern frontend for **REGIS v1.0.5** using **TanStack Start, Vite, Shadcn UI, and TanStack Query**.

---

## 1. API Overview

- **Base URL**: `/api/v1`
- **Authentication**: All `/jobs/*` routes require an API Key
- **Content-Type**: `application/json`
- **Swagger Docs**: `http://localhost:3000/swagger/index.html`

### Authentication

The server generates an API Key on startup if one is not provided. You **must** include this key in the header of all requests to `/jobs/*` endpoints.

**Header Format:**
```http
X-API-Key: <YOUR_API_KEY>
```

**Alternative (Query Parameter):**
```
?api_key=<YOUR_API_KEY>
```

> [!IMPORTANT]
> The API Key is printed to the console when the server starts. Copy it for frontend configuration.
> Example: `API Key: 550e8400-e29b-41d4-a716-446655440000`

---

## 2. API Endpoints Reference

### A. Health Check (No Auth Required)

**Endpoint:** `GET /api/v1/health`

**Description:** Simple ping to verify backend connectivity. Does **NOT** require API Key.

**Response (200 OK):**
```json
{
  "status": "ok",
  "message": "Regis API is running"
}
```

**Frontend Use:** Call on app load to check if backend is reachable.

---

### B. Submit a Job

**Endpoint:** `POST /api/v1/jobs/submit`

**Description:** Submits a new pipeline job. Requires authenticated request with valid JSON payload.

**Headers:**
```http
Content-Type: application/json
X-API-Key: <YOUR_API_KEY>
```

**Request Body:** See [Config Payload Reference](#3-config-payload-reference-complete) below.

**Response (202 Accepted):**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "message": "Job submitted successfully",
  "output_dir": "/absolute/path/to/output"
}
```

**Error Responses:**
- `400 Bad Request` - Invalid JSON or validation error
- `500 Internal Server Error` - Database or queue error

---

### C. Get Job Status

**Endpoint:** `GET /api/v1/jobs/:uuid/status`

**Description:** Returns the current status and metadata of a job.

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "running",
  "start_time": "2025-12-15T10:00:00Z",
  "end_time": "0001-01-01T00:00:00Z",
  "error": "",
  "external_id": "",
  "config": { ... }
}
```

**Status Values:**
| Status | Description |
|--------|-------------|
| `queued` | Job is in queue, waiting to start |
| `running` | Pipeline is actively executing |
| `completed` | Pipeline finished successfully |
| `failed` | Pipeline encountered an error |
| `submitted` | (Slurm only) Job submitted to HPC scheduler |

**Frontend Integration (TanStack Query):**
```typescript
useQuery({
  queryKey: ['jobStatus', jobId],
  queryFn: () => fetchJobStatus(jobId),
  refetchInterval: (data) => 
    (data.status === 'completed' || data.status === 'failed') 
      ? false 
      : 3000 // Poll every 3 seconds
})
```

---

### D. Get Job Results Summary

**Endpoint:** `GET /api/v1/jobs/:uuid/results`

**Description:** Returns lightweight summary. Only available when `status === "completed"`.

**Response (200 OK):**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "output_dir": "/path/to/output"
}
```

**Error Responses:**
- `400 Bad Request` - Job not completed yet
- `404 Not Found` - Job not found

---

### E. Get Job Metrics

**Endpoint:** `GET /api/v1/jobs/:uuid/results/metrics`

**Description:** Returns detailed pipeline metrics from `pipeline_summary.json`. Available only when pipeline has completed step 16.

**Response (200 OK):**
```json
{
  "run_id": "550e8400-e29b-41d4-a716-446655440000",
  "sample_name": "regis_out_20251215",
  "duration": "1h23m45s",
  "validation_mode": "consensus",
  "total_transcripts": 12500,
  "lncrna_candidates": 147,
  "novel_lncrnas": 45,
  "highly_expressed": 23,
  "best_candidates": 8,
  "associated_genes": 234,
  "fastqc_metrics": { ... },
  "trimmomatic_metrics": { ... },
  "hisat2_metrics": { ... },
  "cpc2_metrics": { ... },
  "cpat_metrics": { ... },
  "lncrna_filtering_metrics": { ... },
  "errors": []
}
```

**Error Responses:**
- `404 Not Found` - Job or metrics file not found

---

### F. Download Job Results

**Endpoint:** `GET /api/v1/jobs/:uuid/results/download`

**Description:** Streams a ZIP archive of the output directory.

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `light` | `boolean` | If `true`, skips large files (FASTQ, BAM) and includes only reports |

**Response:** Streams `application/zip` file.

**Frontend Integration:**
```html
<!-- Full Download -->
<a href="/api/v1/jobs/{id}/results/download?api_key=KEY">
  Download Full Results
</a>

<!-- Light Download (Reports Only) -->
<a href="/api/v1/jobs/{id}/results/download?light=true&api_key=KEY">
  Download Reports Only
</a>
```

---

## 3. Config Payload Reference (Complete)

This is the complete configuration object accepted by `POST /jobs/submit`.

### Core Settings (Required)

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| `email` | `string` | **Yes** | Valid email format | User email for job tracking and notifications |
| `data_type` | `string` | **Yes** | `"single"` or `"paired"` | Type of RNA-seq data |
| `method` | `string` | **Yes** | `"denovo"` or `"reference"` | Assembly method |
| `file1` | `string` | **Yes** | Absolute path, file must exist | Path to Read 1 (or single-end file) |

### Input Files (Conditional)

| Field | Type | Required When | Description |
|-------|------|--------------|-------------|
| `file2` | `string` | `data_type = "paired"` | Path to Read 2 |
| `reference` | `string` | `method = "reference"` | Path to genome FASTA |
| `gtf` | `string` | `method = "reference"` | Path to annotation GTF/GFF |

### Optional Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `output_dir` | `string` | Auto-generated | Output directory path (will be created) |
| `threads` | `number` | All available | Number of CPU threads to use |
| `species` | `string` | `"Human"` | Species for CPAT model (`Human`, `Mouse`, `Fly`, `Zebrafish`) |
| `execution_mode` | `string` | `"local"` | `"local"` or `"slurm"` (HPC cluster) |

### Validation & Filtering Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skip_cpat` | `boolean` | `false` | If `true`, uses only CPC2 (faster, less rigorous) |
| `enable_sortmerna` | `boolean` | `false` | If `true`, filters rRNA before assembly (recommended) |

### LncTar Target Prediction

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_lnctar` | `boolean` | `false` | Enable LncTar tool for interaction prediction |
| `lnctar_best_only` | `boolean` | `false` | Run only on BEST candidates (fastest, ~5 min) |
| `lnctar_highly` | `boolean` | `false` | Run on highly expressed lncRNAs (TPM > 1.0) |
| `lnctar_comprehensive` | `boolean` | `false` | Run on ALL candidates (slow, ~60 min) |

### IntaRNA Target Prediction

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enable_intarna` | `boolean` | `false` | Enable IntaRNA tool (more accurate) |
| `intarna_best_only` | `boolean` | `false` | Run only on BEST candidates |
| `intarna_highly` | `boolean` | `false` | Run on highly expressed lncRNAs |
| `intarna_comprehensive` | `boolean` | `false` | Run on ALL candidates |

### Slurm Configuration (For HPC)

| Field | Type | Description |
|-------|------|-------------|
| `slurm.partition` | `string` | Slurm partition name |
| `slurm.nodes` | `number` | Number of nodes |
| `slurm.cpus` | `number` | CPUs per node |
| `slurm.memory` | `string` | Memory allocation (e.g., `"120G"`) |
| `slurm.time` | `string` | Time limit (e.g., `"24:00:00"`) |
| `slurm.extra_script` | `string[]` | Additional bash commands to run before pipeline |

---

## 4. Complete Request Example

```json
{
  "email": "researcher@university.edu",
  "data_type": "paired",
  "method": "reference",
  "file1": "/data/samples/SRR123_R1.fastq.gz",
  "file2": "/data/samples/SRR123_R2.fastq.gz",
  "reference": "/data/genomes/fly/dm6.fasta",
  "gtf": "/data/genomes/fly/annotation.gtf",
  "threads": 16,
  "species": "Fly",
  "enable_sortmerna": true,
  "enable_lnctar": true,
  "lnctar_best_only": true,
  "enable_intarna": true,
  "intarna_best_only": true,
  "execution_mode": "local"
}
```

---

## 5. TypeScript Interfaces

Copy these into your frontend `types/` folder:

```typescript
// ============ Enums ============
export type DataType = 'single' | 'paired';
export type AnalysisMethod = 'denovo' | 'reference';
export type ExecutionMode = 'local' | 'slurm';
export type JobStatus = 'queued' | 'running' | 'completed' | 'failed' | 'submitted';
export type Species = 'Human' | 'Mouse' | 'Fly' | 'Zebrafish';

// ============ Slurm Config ============
export interface SlurmConfig {
  partition?: string;
  job_name?: string;
  time?: string;
  memory?: string;
  cpus?: number;
  nodes?: number;
  email?: string;
  extra_args?: string;
  extra_script?: string[];
}

// ============ Pipeline Config ============
export interface PipelineConfig {
  // Required
  email: string;
  data_type: DataType;
  method: AnalysisMethod;
  file1: string;
  
  // Conditional (based on data_type/method)
  file2?: string;
  reference?: string;
  gtf?: string;
  
  // Optional
  output_dir?: string;
  threads?: number;
  species?: Species;
  
  // Validation
  skip_cpat?: boolean;
  enable_sortmerna?: boolean;
  
  // LncTar
  enable_lnctar?: boolean;
  lnctar_best_only?: boolean;
  lnctar_highly?: boolean;
  lnctar_comprehensive?: boolean;
  
  // IntaRNA
  enable_intarna?: boolean;
  intarna_best_only?: boolean;
  intarna_highly?: boolean;
  intarna_comprehensive?: boolean;
  
  // Execution
  execution_mode?: ExecutionMode;
  slurm?: SlurmConfig;
  
  // Security (usually not sent from frontend)
  api_key?: string;
}

// ============ Job Object ============
export interface Job {
  id: string;
  status: JobStatus;
  config: PipelineConfig;
  start_time: string;  // ISO Date string
  end_time: string;    // ISO Date string
  error?: string;
  external_id?: string; // Slurm Job ID
}

// ============ API Responses ============
export interface SubmitJobResponse {
  job_id: string;
  status: 'queued';
  message: string;
  output_dir: string;
}

export interface JobResultsResponse {
  job_id: string;
  status: 'completed';
  output_dir: string;
}

export interface HealthResponse {
  status: 'ok';
  message: string;
}

export interface ErrorResponse {
  error: string;
  details?: string;
}
```

---

## 6. Form Design Guide

### Job Submission Form Fields

#### Section 1: User Information
| Field | Input Type | Required | Notes |
|-------|-----------|----------|-------|
| Email | `<input type="email">` | Yes | Validate email format |

#### Section 2: Data Type Selection
| Field | Input Type | Options | Notes |
|-------|-----------|---------|-------|
| Data Type | `<RadioGroup>` | Single-end, Paired-end | Show File2 input only if Paired |

#### Section 3: Analysis Method
| Field | Input Type | Options | Notes |
|-------|-----------|---------|-------|
| Method | `<RadioGroup>` | De Novo, Reference-based | Show Reference/GTF inputs only if Reference |

#### Section 4: Input Files
| Field | Input Type | Required | Notes |
|-------|-----------|----------|-------|
| Read 1 (File1) | `<Input>` + Browse | Yes | Absolute path |
| Read 2 (File2) | `<Input>` + Browse | If Paired | Absolute path |
| Reference Genome | `<Input>` + Browse | If Reference | FASTA file |
| Annotation | `<Input>` + Browse | If Reference | GTF/GFF file |

#### Section 5: Species (For CPAT)
| Field | Input Type | Options | Default |
|-------|-----------|---------|---------|
| Species | `<Select>` | Human, Mouse, Fly, Zebrafish | Human |

#### Section 6: Advanced Options (Collapsible)
| Field | Input Type | Default | Notes |
|-------|-----------|---------|-------|
| CPU Threads | `<NumberInput>` | Auto | 0 = use all |
| Skip CPAT | `<Switch>` | Off | CPC2-only mode |
| Enable rRNA Filtering | `<Switch>` | Off | Recommended for cleaner data |

#### Section 7: Target Prediction (Collapsible)
| Field | Input Type | Default | Notes |
|-------|-----------|---------|-------|
| Enable LncTar | `<Switch>` | Off | |
| LncTar Mode | `<RadioGroup>` | - | Best Only / Highly Expressed / All |
| Enable IntaRNA | `<Switch>` | Off | |
| IntaRNA Mode | `<RadioGroup>` | - | Best Only / Highly Expressed / All |

### Form Validation Rules

```typescript
import { z } from 'zod';

export const jobFormSchema = z.object({
  email: z.string().email('Valid email required'),
  data_type: z.enum(['single', 'paired']),
  method: z.enum(['denovo', 'reference']),
  file1: z.string().min(1, 'File 1 is required'),
  file2: z.string().optional(),
  reference: z.string().optional(),
  gtf: z.string().optional(),
  species: z.enum(['Human', 'Mouse', 'Fly', 'Zebrafish']).optional(),
  threads: z.number().min(0).optional(),
  skip_cpat: z.boolean().optional(),
  enable_sortmerna: z.boolean().optional(),
  enable_lnctar: z.boolean().optional(),
  lnctar_best_only: z.boolean().optional(),
  lnctar_highly: z.boolean().optional(),
  lnctar_comprehensive: z.boolean().optional(),
  enable_intarna: z.boolean().optional(),
  intarna_best_only: z.boolean().optional(),
  intarna_highly: z.boolean().optional(),
  intarna_comprehensive: z.boolean().optional(),
}).refine((data) => {
  // If paired, file2 is required
  if (data.data_type === 'paired' && !data.file2) {
    return false;
  }
  return true;
}, { message: 'File 2 required for paired-end data', path: ['file2'] })
.refine((data) => {
  // If reference, genome and gtf are required
  if (data.method === 'reference' && (!data.reference || !data.gtf)) {
    return false;
  }
  return true;
}, { message: 'Reference genome and GTF required for reference mode', path: ['reference'] });
```

---

## 7. Frontend Workflow Recommendations

### Page Structure

```
/                     → Landing Page
/submit               → Job Submission Form
/jobs/:id             → Job Status & Progress
/jobs/:id/results     → Results Dashboard (when completed)
```

### Workflow

1. **Landing Page**
   - Hero section explaining REGIS
   - Quick start button → `/submit`
   - Recent jobs (if authenticated)

2. **Job Submission (`/submit`)**
   - Multi-step form with validation
   - On success: Redirect to `/jobs/:id`

3. **Job Status (`/jobs/:id`)**
   - Poll status every 3 seconds
   - Show progress indicator
   - Display current step (if available)
   - On completion: Show "View Results" button

4. **Results Dashboard (`/jobs/:id/results`)**
   - Fetch metrics from `/results/metrics`
   - Display key statistics
   - Download buttons (Full / Light)
   - Link to MultiQC report

---

## 8. API Client Example

```typescript
// lib/api.ts
const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:3000/api/v1';
const API_KEY = process.env.NEXT_PUBLIC_API_KEY || '';

async function apiRequest<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': API_KEY,
      ...options?.headers,
    },
  });
  
  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error || 'API request failed');
  }
  
  return response.json();
}

export const api = {
  health: () => apiRequest<HealthResponse>('/health'),
  
  submitJob: (config: PipelineConfig) => 
    apiRequest<SubmitJobResponse>('/jobs/submit', {
      method: 'POST',
      body: JSON.stringify(config),
    }),
  
  getJobStatus: (id: string) => 
    apiRequest<Job>(`/jobs/${id}/status`),
  
  getJobResults: (id: string) => 
    apiRequest<JobResultsResponse>(`/jobs/${id}/results`),
  
  getJobMetrics: (id: string) => 
    apiRequest<any>(`/jobs/${id}/results/metrics`),
  
  getDownloadUrl: (id: string, light = false) => 
    `${API_BASE}/jobs/${id}/results/download?api_key=${API_KEY}${light ? '&light=true' : ''}`,
};
```

---

## 9. Development & Testing

### Mock Data

Use these responses for frontend development before backend is ready:

```typescript
// mocks/job.ts
export const mockJob: Job = {
  id: '550e8400-e29b-41d4-a716-446655440000',
  status: 'completed',
  config: {
    email: 'test@example.com',
    data_type: 'paired',
    method: 'reference',
    file1: '/data/R1.fastq.gz',
    file2: '/data/R2.fastq.gz',
    reference: '/data/genome.fa',
    gtf: '/data/annotation.gtf',
    species: 'Fly',
  },
  start_time: '2025-12-15T10:00:00Z',
  end_time: '2025-12-15T11:23:45Z',
  error: '',
};
```

### Local Development

1. Start backend: `regis serve --port 3000 --job-dir ./jobs`
2. Note the API Key printed to console
3. Set `NEXT_PUBLIC_API_KEY` in `.env.local`
4. Start frontend: `npm run dev`

---

## 10. Error Handling

### Common Error Codes

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Validation Error | Show field-level errors from `details` |
| 401 | Unauthorized | Check API Key configuration |
| 404 | Not Found | Job doesn't exist or was deleted |
| 500 | Server Error | Display generic error, log details |

### Error Display

```typescript
// Show validation errors per field
if (error.details) {
  toast.error(`Validation Error: ${error.details}`);
} else {
  toast.error(error.error || 'An unexpected error occurred');
}
```

---

*Last Updated: December 2025 | REGIS v1.0.5*
