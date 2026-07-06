# Container Supply Chain Security — build, attest, verify

[![CI](https://github.com/alice101-dev/supply-chain-secure-build/actions/workflows/ci.yml/badge.svg)](https://github.com/alice101-dev/supply-chain-secure-build/actions/workflows/ci.yml)

## Why this matters

Modern breaches increasingly skip your firewall and walk in through the
**build pipeline**. The attack doesn't have to come from outside:

- **A malicious insider** (or one stolen laptop / CI token) builds an image
  with a backdoor and `kubectl apply`s it straight to production — no review,
  no scan, no trace of where the binary came from.
- **A poisoned dependency** — one `go get` of a typosquatted or compromised
  package (the xz-utils / event-stream / SolarWinds pattern) and the backdoor
  is compiled into your binary *by your own CI*, signed off by nobody.
- **A rogue image** — retagged, tampered, or pulled from an unvetted registry —
  lands in the cluster because Kubernetes, by default, **runs whatever it is
  told to run**. `image: attacker/nginx:latest` schedules just as happily as
  yours.

The common thread: without provenance, signatures, and admission-time
verification, the cluster cannot tell *your* build from an attacker's. This
repo closes that gap end to end — an **SBOM** for every image, a
**vulnerability gate** before publish, **keyless signing** that cryptographically
ties the image to *this repo's CI workflow* (an insider can't reproduce it from
a laptop), **SLSA provenance** recording exactly which commit and runner built
it, and a **Kyverno policy** that makes Kubernetes reject anything unsigned,
unscanned, or built anywhere else.

```mermaid
graph LR
    C["commit / PR"]
    C --> G["🔎 SAST<br>(Semgrep CE)"]
    C --> GV["🔗 SCA<br>(govulncheck)"]
    C --> GL["🔑 secrets<br>(Gitleaks)"]
    C --> CK["📐 IaC scan<br>(Checkov)"]
    C --> GO["🧪 go<br>build / vet / test"]
    G & GV & GL & CK & GO --> B["🔨 build<br>(distroless, digest-pinned)"]
    C -.PR only.-> SK["🧩 Socket dep risk<br>(gates merge, not build)"]
    B --> T["🛡️ Trivy gate<br>CRITICAL/HIGH ⇒ fail"]
    T --> S["📋 SBOM<br>(Syft, SPDX)"]
    S --> P["📦 push to GHCR"]
    P --> K["🔏 Cosign sign<br>keyless via OIDC"]
    K --> A["📜 attest<br>SBOM + SLSA provenance"]
    A --> R["🧾 Rekor<br>transparency log"]
    A --> D["☸️ deploy"]
    V["Kyverno verifyImages"] -->|signature valid| D
    V -.rejected.-> U["❌ unsigned /<br>foreign image"]
```

## What the pipeline enforces

| Stage | Tool | Guarantee |
| --- | --- | --- |
| SAST | Semgrep CE (`p/golang`, `p/gosec`, `p/cwe-top-25`) | insecure code patterns fail the build before anything is compiled |
| SCA | govulncheck | call-graph aware: vulnerabilities in dependencies & stdlib that the code **actually reaches** fail the build |
| Supply-chain risk | Socket | behavioral analysis of dependency code on PRs (malware, install scripts, abandoned packages) — the day-zero risk CVE databases miss; blocking is policy-driven |
| Secret scanning | Gitleaks | full git history scanned every run — a leaked key fails the build, even if it was committed and later removed |
| IaC scan | Checkov | Dockerfile + Kubernetes manifest misconfigurations (root user, missing limits, mutable tags) fail the build |
| Build | Docker multi-stage → distroless/static | no shell, no package manager, ~2 MB attack surface; base images pinned by digest |
| Vulnerability gate | Trivy | **any** CRITICAL/HIGH ⇒ the image is **never published** — unfixed CVEs included (no silent `ignore-unfixed`); the only way past is a documented, time-boxed exception in `.trivyignore` |
| Inventory | Syft | SPDX SBOM generated and attached to the image as a signed attestation |
| Signing | Cosign **keyless** | GitHub OIDC proves *which repo & workflow* built it; Fulcio issues a short-lived cert; the signature is logged in Rekor. **No key to store, rotate, or leak** |
| Provenance | GitHub Attestations (SLSA) | signed statement of the exact commit, workflow, and runner that produced the image |
| Admission | Kyverno `verifyImages` | the cluster **fails closed**: only images signed by this repo's CI are schedulable; tags are mutated to verified digests |

The pipeline also defends **itself**: every third-party action is pinned to a
full commit SHA (with the version as a comment), so a hijacked action tag —
the `tj-actions/changed-files` attack pattern — cannot inject code into this
build. Same principle as the digest-pinned base images. The gate order in the
diagram is enforced with `needs:`, not just implied: the Docker build does not
start until SAST, SCA, secret scanning, the IaC scan, and `go build/vet/test`
have all passed, so a poisoned dependency stops the pipeline before Docker ever
runs. Socket is the one exception — it runs on PRs only, so it gates the merge
to `main` (via required status checks) rather than the build itself.

On a **PR**, every gate above runs *except* publish/sign/attest — the image is
built and scanned but never pushed. Signing, attestation, and the registry push
happen only when the commit lands on `main`, so nothing unsigned or unverified
ever reaches GHCR. (Go build/vet/test runs on both.)

### Supply-chain risk on every PR (Socket)

When a PR adds or changes a dependency, Socket analyses the package's actual
code and posts a risk scorecard on the PR. Whether a finding blocks is set in
the Socket **Security Policy** (`error` fails the check, `warn` only reports) —
not in the workflow. Below: a test PR that added deliberately dangerous
dependencies, blocked with `High CVE` alerts once the policy was set to `error`:

![Socket Security dependency-overview scorecard on a pull request, scoring the four added packages (gogs, casdoor, x/crypto, go-base) with their Vulnerability columns flagged red](scan.png)

### After publish: continuous re-scanning

Build-time scanning only knows the CVEs that existed when the image was built.
New ones are disclosed daily, and an already-published image has no commit to
re-trigger CI. A scheduled workflow (`.github/workflows/rescan.yml`) closes that
window: every day it re-scans the published image's **signed SBOM** against the
current Trivy database *and* re-verifies that its Cosign signature, SBOM
attestation, and SLSA provenance still hold. It scans the SBOM, **not the image**
— the SPDX inventory `cosign attest` attached at build time is already the package
list, so re-scanning is a cheap DB match with no image pull or layer unpack. That
is what keeps it light at scale: across many services a nightly SBOM scan is a DB
match per service, not an image pull per service (and for a large fleet, feeding
those SBOMs into a central monitor like Dependency-Track is lighter still). Because
the image is already out, this is **detective, not preventive** — a regression
opens (or updates) one tracking GitHub issue rather than blocking a build, and that
issue auto-closes once a later scan comes back clean. The fix is a rebuild from
`main`, or a justified, time-boxed `.trivyignore` entry. The workflow can also be
run on demand against any published tag (`workflow_dispatch` with an `image_ref`
input), not just on the daily schedule.

## Verify it yourself

Anyone can verify the image — that's the point of keyless + transparency logs:

```bash
IMAGE=ghcr.io/alice101-dev/supply-chain-secure-build:latest

# Signature: was this built by THIS repo's workflow?
cosign verify \
  --certificate-identity-regexp '^https://github.com/alice101-dev/supply-chain-secure-build/\.github/workflows/ci\.yml@refs/heads/main$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "$IMAGE"

# SBOM: what exactly is inside?
cosign verify-attestation --type spdxjson \
  --certificate-identity-regexp '^https://github.com/alice101-dev/supply-chain-secure-build/\.github/workflows/ci\.yml@refs/heads/main$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "$IMAGE" | jq -r '.payload' | base64 -d | jq '.predicate.packages[].name'

# Provenance: which commit, which workflow, which runner?
gh attestation verify oci://$IMAGE --repo alice101-dev/supply-chain-secure-build
```

What success looks like (output abbreviated):

| Command | Key lines in a good result | What they prove |
| --- | --- | --- |
| `cosign verify` | `The cosign claims were validated` · `Existence of the claims in the transparency log was verified offline` · `The code-signing certificate was verified using trusted certificate authority certificates` | the signature is genuine, publicly logged in Rekor, and chained to Sigstore's CA — not self-asserted |
| `cosign verify-attestation` | JSON entries with `"type": "https://spdx.dev/Document"` and `"https://slsa.dev/provenance/v1"`, all bound to the **same** `docker-manifest-digest` | the SBOM and provenance are attached to *exactly this image*, byte for byte — swap the image and the digest no longer matches |
| `gh attestation verify` | `✓ Verification succeeded!` · `Build repo: alice101-dev/supply-chain-secure-build` · `Build workflow: .github/workflows/ci.yml@refs/heads/main` | the image was built by *this repo's* CI on `main` — not on someone's laptop, not by a fork |

If any check fails — wrong repo, tampered layer, unsigned rebuild — the
commands exit non-zero, and the same failure is what makes Kyverno reject the
pod at admission.

## Enforce it in a cluster

```bash
# Requires Kyverno (https://kyverno.io) installed in the cluster
kubectl apply -f k8s/kyverno-verify-image-signature.yaml

# This deploys fine — the image is signed by this repo's CI:
kubectl apply -f k8s/deployment.yaml

# This is REJECTED at admission — unsigned image:
kubectl run bad --image=nginx:latest
```

The policy fails **closed** (`failurePolicy: Fail`) and rewrites tags to the
verified digest (`mutateDigest`), so even `:latest` deploys are reproducible.

## Repository layout

```
.
├── .github/workflows/ci.yml                  # SAST · SCA · Socket · Gitleaks · Checkov · build→scan→SBOM→sign→attest→verify
├── Dockerfile                                # multi-stage, distroless, digest-pinned, version-stamped
├── cmd/server/main.go                        # entrypoint: wiring + build identity
├── internal/
│   ├── config/                               # env-based config (twelve-factor)
│   ├── handler/                              # routes, probes, request logging (+ unit tests)
│   └── server/                               # hardened timeouts, graceful shutdown
└── k8s/
    ├── kyverno-verify-image-signature.yaml   # admission: only OUR signatures pass
    └── deployment.yaml                       # hardened consumer (backend-api)
```

## The service itself

Not a hello-world in one file — a production-shaped Go backend:

- **Structured JSON logs** (`log/slog`) with per-request logging that skips
  probe endpoints.
- **Hardened `http.Server` timeouts** (read/write/idle/header) — one slow
  client can't pin connections.
- **Graceful shutdown**: on SIGTERM, `/readyz` flips to 503 so Kubernetes
  drains traffic *first*, then in-flight requests finish within
  `SHUTDOWN_TIMEOUT`.
- **`/version` reports the exact commit** stamped at build time via
  `-ldflags` — the same commit the image's SLSA provenance attests to, so
  runtime identity and supply chain evidence line up.

## Related

- [terraform-pr-gates](https://github.com/alice101-dev/terraform-pr-gates) — the same
  shift-left philosophy applied to Terraform PRs.
- [gke-pgbouncer-hardened](https://github.com/alice101-dev/gke-pgbouncer-hardened) — the
  runtime-hardening counterpart of the images this pipeline produces.
