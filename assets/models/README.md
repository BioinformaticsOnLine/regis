# CPAT Models Directory

This directory contains pre-trained CPAT models for different species.

## 📁 Structure

```
models/
├── README.md                    # This file
├── Human_Hexamer.tsv           # Human k-mer model (~123 KB)
├── Human_logitModel.RData      # Human logit model (~3.5 MB)
├── Mouse_Hexamer.tsv           # Mouse k-mer model (optional)
├── Mouse_logitModel.RData      # Mouse logit model (optional)
├── Zebrafish_Hexamer.tsv       # Zebrafish k-mer model (optional)
├── Zebrafish_logitModel.RData  # Zebrafish logit model (optional)
├── Fly_Hexamer.tsv             # Fly k-mer model (optional)
└── Fly_logitModel.RData        # Fly logit model (optional)
```

## 📥 Download Models

### Required: Human Models

```bash
cd models/

# Human (required - most common use case)
wget -O Human_Hexamer.tsv \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Human_Hexamer.tsv/download

wget -O Human_logitModel.RData \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Human_logitModel.RData/download
```

### Optional: Mouse Models

```bash
wget -O Mouse_Hexamer.tsv \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Mouse_Hexamer.tsv/download

wget -O Mouse_logitModel.RData \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Mouse_logitModel.RData/download
```

### Optional: Zebrafish Models

```bash
wget -O Zebrafish_Hexamer.tsv \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Zebrafish_Hexamer.tsv/download

wget -O Zebrafish_logitModel.RData \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Zebrafish_logitModel.RData/download
```

### Optional: Fly Models

```bash
wget -O Fly_Hexamer.tsv \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Fly_Hexamer.tsv/download

wget -O Fly_logitModel.RData \
  https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/Fly_logitModel.RData/download
```

## ✅ Verify Downloads

After downloading, verify file sizes:

```bash
ls -lh models/

# Expected sizes:
# Human_Hexamer.tsv       ~123 KB
# Human_logitModel.RData  ~3.5 MB
# Mouse_Hexamer.tsv       ~123 KB
# Mouse_logitModel.RData  ~3.5 MB
# ...
```

**Important:** Files should NOT be 0 bytes!

## 📦 Conda Package Installation

When the conda package is built, these models will be installed to:

```
$PREFIX/share/regis/models/
```

The `regis.sh` script automatically detects bundled models at this location.

## 🎯 Bundling Recommendations

| Configuration | Size | Use Case |
|---------------|------|----------|
| **Human only** | ~3.6 MB | Most users (90%+) |
| **Human + Mouse** | ~7.2 MB | Biomedical research |
| **All 4 species** | ~14.4 MB | Full coverage |

**Recommendation:** Bundle Human models at minimum. Other species can auto-download on demand.

## 🔍 Model Source

All models are from the official CPAT project:
- **Source:** https://sourceforge.net/projects/rna-cpat/files/prebuilt_models/
- **Reference:** Wang et al. (2013) *Nucleic Acids Research*
- **DOI:** https://doi.org/10.1093/nar/gkt006

## ⚠️ Important Notes

1. **Species names are case-sensitive:** Use `Human`, `Mouse`, `Zebrafish`, `Fly`
2. **Both files required:** Each species needs both `.tsv` and `.RData` files
3. **No redistribution restrictions:** These are open-source research models
4. **Version:** CPAT v3.0.0 models

