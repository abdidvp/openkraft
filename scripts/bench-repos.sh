#!/usr/bin/env bash
set -euo pipefail

# bench-repos.sh — Clone Go repos, score each with openkraft, generate comparison report.
# Usage: ./scripts/bench-repos.sh [--keep-repos]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPENKRAFT="${SCRIPT_DIR}/../openkraft"

if [[ ! -x "$OPENKRAFT" ]]; then
  echo "ERROR: openkraft binary not found at $OPENKRAFT" >&2
  echo "Run 'go build -o openkraft .' first." >&2
  exit 1
fi

KEEP_REPOS=false
[[ "${1:-}" == "--keep-repos" ]] && KEEP_REPOS=true

REPOS=(
  # Web frameworks
  "go-chi/chi"
  "gofiber/fiber"
  "charmbracelet/bubbletea"
  "ThreeDotsLabs/wild-workouts-go-ddd-example"
  "kataras/iris"
  # Diverse profiles
  "spf13/cobra"              # CLI library (small, clean API)
  "hashicorp/terraform"      # Infra tool (massive, professional quality)
  "ollama/ollama"            # Modern Go app (recent, popular)
  "gohugoio/hugo"            # Mature tool (10+ years, large codebase)
  "nektos/act"               # CLI tool (medium size)
)

TMPDIR=$(mktemp -d)
trap '[[ "$KEEP_REPOS" == false ]] && rm -rf "$TMPDIR"' EXIT

RESULTS_DIR="${TMPDIR}/_results"
mkdir -p "$RESULTS_DIR"

echo "Cloning and scoring ${#REPOS[@]} repos..."
echo "Temp dir: $TMPDIR"
echo ""

REPO_ORDER=()

for repo in "${REPOS[@]}"; do
  name="${repo##*/}"
  echo "--- $repo ---"

  echo "  Cloning..."
  if ! git clone --depth 1 --quiet "https://github.com/${repo}.git" "${TMPDIR}/${name}" 2>/dev/null; then
    echo "  FAILED to clone, skipping."
    echo "{\"error\":\"clone failed\"}" > "${RESULTS_DIR}/${name}.json"
    REPO_ORDER+=("$repo")
    continue
  fi

  echo "  Scoring..."
  if "$OPENKRAFT" score "${TMPDIR}/${name}" --json > "${RESULTS_DIR}/${name}.json" 2>/dev/null; then
    echo "  Done."
  else
    echo "  FAILED to score, skipping."
    echo "{\"error\":\"score failed\"}" > "${RESULTS_DIR}/${name}.json"
  fi
  REPO_ORDER+=("$repo")
done

echo ""
echo "Generating report..."
echo ""

# Build a manifest: repo_name -> json file path
MANIFEST="${RESULTS_DIR}/_manifest.txt"
for repo in "${REPO_ORDER[@]}"; do
  name="${repo##*/}"
  echo "${repo}|${RESULTS_DIR}/${name}.json" >> "$MANIFEST"
done

python3 -c '
import json, sys

manifest_path = sys.argv[1]
rows = []

with open(manifest_path) as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        repo, json_path = line.split("|", 1)
        with open(json_path) as jf:
            data = json.load(jf)

        if "error" in data:
            rows.append({"repo": repo, "error": data["error"]})
            continue

        cats = {c["name"]: c["score"] for c in data.get("categories", [])}
        issues = []
        for c in data.get("categories", []):
            issues.extend(c.get("issues", []))

        severity_counts = {}
        for iss in issues:
            sev = iss.get("severity", "unknown")
            severity_counts[sev] = severity_counts.get(sev, 0) + 1

        rows.append({
            "repo": repo,
            "overall": data.get("overall", "?"),
            "code_health": cats.get("code_health", "-"),
            "discoverability": cats.get("discoverability", "-"),
            "structure": cats.get("structure", "-"),
            "verifiability": cats.get("verifiability", "-"),
            "context_quality": cats.get("context_quality", "-"),
            "predictability": cats.get("predictability", "-"),
            "issues": len(issues),
            "errors": severity_counts.get("error", 0),
            "warnings": severity_counts.get("warning", 0),
            "info": severity_counts.get("info", 0),
        })

header = "| Repo | Overall | Health | Discover | Structure | Verify | Context | Predict | Issues | Err | Warn | Info |"
sep    = "|------|---------|--------|----------|-----------|--------|---------|---------|--------|-----|------|------|"
print(header)
print(sep)

for r in rows:
    if "error" in r:
        print("| %s | %s | | | | | | | | | | |" % (r["repo"], r["error"]))
        continue
    print("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |" % (
        r["repo"], r["overall"], r["code_health"], r["discoverability"],
        r["structure"], r["verifiability"], r["context_quality"],
        r["predictability"], r["issues"], r["errors"], r["warnings"], r["info"]))
' "$MANIFEST"
