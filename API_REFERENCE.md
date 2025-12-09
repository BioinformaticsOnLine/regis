# Regis API Reference & Frontend Guide

Base URL: `http://localhost:3000/api/v1`

## 1. Job Submission
**Endpoint:** `POST /jobs/submit`
**Description:** Starts the pipeline.
**Content-Type:** `application/json`

**Request Body (Comprehensive):**
```json
{
  "run_id": "optional-uuid",    // Optional: Provide your own UUID
  
  // -- Required --
  "email": "user@example.com",  // REQUIRED: User email for notification
  "data_type": "paired",        // "paired" or "single"
  "method": "denovo",           // "denovo" or "reference"
  "file1": "/abs/path/to/R1.fq",
  "file2": "/abs/path/to/R2.fq", // Required for paired
  
  // -- Mode-Specific --
  "reference": "/path/to/genome.fa", // Required for reference mode
  "gtf": "/path/to/annotation.gtf",  // Required for reference mode
  
  // -- General Options --
  "output_dir": "",             // Optional: Custom output path
  "threads": 8,                 // Defalt: 0 (Use all cores)
  "species": "human",           // Default: human (for CPAT)
  
  // -- Advanced Validations --
  "enable_sortmerna": true,     // Default: false (Filter rRNA)
  "skip_cpat": false,           // Default: false (Use CPC2 only)
  "cpat_hex": "/path/hex.tsv",  // Custom CPAT model
  "cpat_logit": "/path/logit.RData",
  
  // -- Target Prediction (Heavy) --
  "enable_lnctar": true,        // Default: false
  "lnctar_best_only": true,     // Default: false (Top 1% targets)
  "lnctar_comprehensive": false,// Default: false (Verify with IntaRNA)
  
  "enable_intarna": false,      // Default: false
  "intarna_best_only": true,
  "intarna_comprehensive": false,
  
  // -- Execution Mode (Local vs Slurm) --
  "execution_mode": "slurm",    // Default: "local"
  "slurm": {                    // Required if execution_mode="slurm"
    "partition": "compute",
    "nodes": 1,
    "cpus": 40,
    "memory": "120G",
    "time": "24:00:00",
    "email": "pranjal.p@example.com",
    "extra_script": [
      "export PATH=\"/home/pranjal.p/test:$PATH\"",
      "module load something"
    ]
  }
}
```

### HPC/Slurm Example (Custom Environment)
If you need to load modules or set paths (like your example), use `extra_script` as a list of strings:

```json
{
  "email": "pranjal.p@example.com",
  "execution_mode": "slurm",
  "slurm": {
    "partition": "compute",
    "nodes": 1,
    "cpus": 40,
    "memory": "120G",
    "extra_script": [
      "export PATH=\"/home/pranjal.p/test:$PATH\"",
      "export PATH=\"/usr/bin:$PATH\"",
      "module load java"
    ]
  },
  "data_type": "paired",
  "method": "denovo",
  "file1": "..."
}
```

**Response:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "output_dir": "/home/user/regis/jobs/550e8400..."
}
```

---

## 2. Job Status (Polling)
**Endpoint:** `GET /jobs/:uuid/status`
**Description:** Poll this every 2-5 seconds to check progress.

**Response:**
```json
{
  "job_id": "550e8400...",
  "status": "running", // "queued", "running", "completed", "failed"
  "config": { ... },   // Original config
  "created_at": "2023-10-27T10:00:00Z",
  "updated_at": "2023-10-27T10:05:00Z"
}
```

---

## 3. Dashboard Metrics (The "Card" View)
**Endpoint:** `GET /jobs/:uuid/results/metrics`
**Description:** Returns the full `pipeline_summary.json`. Use this to populate your UI dashboard. This endpoint returns 404 until the job is `completed`.

**Key JSON Fields for UI Mapping:**

| UI Component | JSON Field Path | Example Value |
|--------------|-----------------|---------------|
| **Status Badge** | `status` | "completed" |
| **Duration** | `total_duration` | "1h 15m" |
| **Total LncRNAs** | `lncrna_filtering.final_lncrnas` | `1523` |
| **Novel / Known** | `lncrna_filtering.novel_intergenic` / `known` | `120` / `1403` |
| **Alignment Rate** | `hisat2.overall_alignment_rate` | `98.5` |
| **GC Content** | `fastqc.percent_gc` | `45.2` |
| **Coding Potential** | `cpc2.noncoding_percent` | `12.5` |

**Full Response Structure (Example):**
```json
{
  "run_id": "uuid",
  "status": "completed",
  "fastqc": {
    "total_sequences": 1500000,
    "percent_gc": 45.0,
    "quality_pass": true
  },
  "hisat2": {
    "overall_alignment_rate": 98.2
  },
  "lncrna_filtering": {
    "final_lncrnas": 1200,
    "novel_intergenic": 50,
    "all_class_codes": {
      "u": {"code": "u", "count": 50, "description": "Unknown/novel"},
      "x": {"code": "x", "count": 10, "description": "Antisense"}
    }
  },
  "step_timings": [
    {"step_number": 1, "step_name": "FastQC", "duration": "2m"}
  ]
}
```

---

## 4. Downloads
**Endpoint:** `GET /jobs/:uuid/results/download`

**Options:**
1.  **Full Download (Heavy):**
    *   `GET /jobs/:uuid/results/download`
    *   Returns: 2GB+ ZIP (BAMs, FASTQs, all intermediates)
    
2.  **Report Only (Lightweight - Recommended):**
    *   `GET /jobs/:uuid/results/download?light=true`
    *   Returns: ~50MB ZIP (HTML Reports, Tables, PDFs only)
    *   **Use this for the "Download Results" button in your UI.**

---

## Frontend Integration Tips (React/TanStack)

1.  **Polling Hook:**
    Create a `useJobStatus(uuid)` hook that hits `/status` every 3s.
    *   If `status === 'completed'`, stop polling and fetch `/results/metrics`.
    *   If `status === 'failed'`, show error.

2.  **Dashboard Layout:**
    *   **Header:** Job ID, Status Badge (Green/Red), Duration.
    *   **Top Row (Cards):** Total LncRNAs, Novel Count, Alignment Rate, GC Content.
    *   **Charts:** 
        *   Pie Chart: Novel (u) vs Known (=) vs Antisense (x) (Source: `lncrna_filtering.all_class_codes`).
        *   Bar Chart: Coding vs Non-Coding (Source: `cpc2`).
    *   **Timeline:** List of Key Steps (Source: `step_timings`).

3.  **Download Actions:**
    *   Primary Button: "Download Report" (`?light=true`).
    *   Secondary Option: "Download Raw Data" (Full zip).
