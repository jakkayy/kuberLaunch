# kuberLaunch — Project Plan

> Internal Developer Platform (IDP) ที่ให้ developer สร้างและ deploy service ได้เองใน 1 คลิก

---

## สถานะปัจจุบัน

| ส่วน | สถานะ |
|------|--------|
| `api/` | go.mod พร้อม, folder structure พร้อม, ยังไม่มี code |
| `web/` | Next.js default, ยังไม่มี UI จริง |
| `templetes/` | folder พร้อม (nestjs/nextjs/go/fastapi), ยังไม่มีไฟล์ |
| `k8s/`, `scripts/`, `docs/` | ว่างทั้งหมด |

---

## Core Data Model

```
Project
├── id           string
├── name         string          "my-api"
├── slug         string          "my-api" (unique, used as k8s namespace prefix)
├── runtime      enum            nextjs | nestjs | go | fastapi
├── database     enum            postgres | mysql | mongodb | none
├── cache        enum            redis | none
├── repo_url     string          GitHub repo URL (Phase 2)
├── status       enum            creating | ready | failed
└── created_at   timestamp

Environment  (belongs to Project)
├── id           string
├── project_id   string
├── name         enum            dev | staging | prod
├── namespace    string          "{slug}-{env}"   e.g. "my-api-dev"
└── status       enum            active | inactive

Deployment   (belongs to Project + Environment)
├── id           string
├── project_id   string
├── env_id       string
├── branch       string          "main"
├── image        string          "ghcr.io/org/my-api:sha-abc123"
├── triggered_by string          "user" | "github-actions"
├── status       enum            pending | building | deploying | success | failed
└── created_at   timestamp
```

> **Database:** PostgreSQL (ข้อมูล relational ชัดเจน, query deployment history ง่าย)
> ลบ `go.mongodb.org/mongo-driver` ออกจาก go.mod เมื่อเริ่ม Phase 1

---

## API Contract

### Phase 1 — Template Generator

```
POST   /api/v1/projects                    Create project + generate templates
GET    /api/v1/projects                    List all projects
GET    /api/v1/projects/:id                Get project detail
DELETE /api/v1/projects/:id                Delete project
GET    /api/v1/projects/:id/download       Download generated files as .zip
GET    /api/v1/projects/:id/preview        Preview generated file tree (JSON)
```

**Request body ตัวอย่าง:**
```json
POST /api/v1/projects
{
  "name": "user-service",
  "runtime": "go",
  "database": "postgres",
  "cache": "redis"
}
```

**Response:**
```json
{
  "id": "proj_abc123",
  "name": "user-service",
  "slug": "user-service",
  "status": "ready",
  "download_url": "/api/v1/projects/proj_abc123/download",
  "files_generated": [
    "Dockerfile",
    ".github/workflows/ci.yml",
    "helm/Chart.yaml",
    "helm/values.yaml",
    "helm/templates/deployment.yaml",
    "helm/templates/service.yaml",
    "helm/templates/ingress.yaml",
    "argocd/application.yaml"
  ]
}
```

### Phase 2 — GitOps Integration

```
POST   /api/v1/projects/:id/repo           Create GitHub repo + push templates
GET    /api/v1/projects/:id/repo           Get repo status
POST   /api/v1/projects/:id/argocd         Register project in ArgoCD
```

### Phase 3 — Self-Service Deployment

```
POST   /api/v1/projects/:id/deployments           Trigger deployment
GET    /api/v1/projects/:id/deployments           List deployment history
GET    /api/v1/projects/:id/deployments/:dep_id   Get deployment status + logs
POST   /api/v1/projects/:id/deployments/:dep_id/rollback  Rollback
```

### Phase 4 — Observability

```
POST   /api/v1/projects/:id/monitoring     Provision Grafana dashboard
GET    /api/v1/projects/:id/metrics        Get current metrics (proxy to Prometheus)
GET    /api/v1/projects/:id/logs           Stream logs (proxy to Loki)
```

### Phase 5 — Secrets

```
POST   /api/v1/projects/:id/secrets        Create/update secret
GET    /api/v1/projects/:id/secrets        List secret keys (ไม่ return value)
DELETE /api/v1/projects/:id/secrets/:key   Delete secret
```

---

## Phase Plan

### Phase 1 — Core Foundation (2-3 สัปดาห์)
**เป้าหมาย:** กรอกชื่อ service → ได้ไฟล์พร้อม deploy

**งานที่ต้องทำ:**

**Backend (api/)**
- [ ] เชื่อม PostgreSQL + migration (projects, environments table)
- [ ] `POST /api/v1/projects` — รับ input, generate templates, บันทึก DB
- [ ] Template engine สำหรับแต่ละ runtime (Go text/template)
- [ ] `GET /api/v1/projects/:id/download` — zip ไฟล์ที่ generate แล้ว
- [ ] `GET /api/v1/projects/:id/preview` — return file tree
- [ ] Health check endpoint `GET /health`

**Templates (templetes/)**
- [ ] Dockerfile template × 4 runtimes (nextjs, nestjs, go, fastapi)
- [ ] `.github/workflows/ci.yml` template (build → push GHCR → update helm values)
- [ ] `helm/` chart template (deployment, service, ingress, hpa)
- [ ] `argocd/application.yaml` template

**Frontend (web/)**
- [ ] หน้า Create Project form (name, runtime, database, cache)
- [ ] หน้า Project List
- [ ] หน้า Project Detail + preview file tree + ปุ่ม Download

**Infrastructure**
- [ ] `docker-compose.yml` — api + postgres + pgadmin (local dev)
- [ ] Kind cluster config สำหรับ local test

**Definition of Done Phase 1:**
> Demo ได้ในภายใน 10 นาที: กรอกชื่อ → เลือก runtime → กด Create → ดูไฟล์ที่ generate → Download zip → เอาไป deploy บน Kind ได้

---

### Phase 2 — GitOps Integration (2 สัปดาห์)
**เป้าหมาย:** สร้าง GitHub repo + register ArgoCD อัตโนมัติ

**งานที่ต้องทำ:**
- [ ] GitHub App integration (หรือ Personal Access Token สำหรับ demo)
- [ ] `POST /api/v1/projects/:id/repo` — สร้าง repo, push generated files
- [ ] ArgoCD API client — register Application CRD
- [ ] webhook receiver สำหรับ GitHub push event
- [ ] บันทึก repo_url, argocd_app_name ใน Project record

**Definition of Done Phase 2:**
> กด "Connect to GitHub" → ระบบสร้าง repo, push code, register ArgoCD → ArgoCD dashboard แสดง app status

---

### Phase 3 — Self-Service Deployment (2-3 สัปดาห์)
**เป้าหมาย:** Developer เลือก branch → กด Deploy → ติดตาม status

**งานที่ต้องทำ:**
- [ ] GitHub Actions trigger via API (workflow_dispatch)
- [ ] Deployment record lifecycle (pending → building → deploying → success/failed)
- [ ] WebSocket หรือ SSE สำหรับ real-time status update
- [ ] ArgoCD sync status polling
- [ ] Rollback endpoint (update helm values ไป image version ก่อนหน้า)
- [ ] Web UI: Deployment history table, status badge, log viewer

**Definition of Done Phase 3:**
> กด Deploy บน web UI → เห็น status เปลี่ยน real-time → service ขึ้น บน Kind

---

### Phase 4 — Observability (2 สัปดาห์)
**เป้าหมาย:** ทุก project ได้ Grafana dashboard + log viewer อัตโนมัติ

**งานที่ต้องทำ:**
- [ ] ติดตั้ง kube-prometheus-stack บน Kind via Helm
- [ ] ติดตั้ง Grafana Loki stack
- [ ] Grafana API client — สร้าง dashboard จาก template per project
- [ ] Prometheus ServiceMonitor template (inject ตอน create project)
- [ ] Log query proxy `/api/v1/projects/:id/logs?env=dev`
- [ ] Web UI: embed Grafana panel หรือ custom metrics widget

**Definition of Done Phase 4:**
> สร้าง project ใหม่ → deploy → เข้า web portal → เห็น CPU/Memory/Error Rate ทันที

---

### Phase 5 — Secrets + Golden Path (2 สัปดาห์)
**เป้าหมาย:** จัดการ secret ผ่าน UI + Golden Path flow ครบวงจร

**งานที่ต้องทำ:**
- [ ] HashiCorp Vault dev mode บน Kind
- [ ] Vault API client — create/read/delete secrets per project path
- [ ] External Secrets Operator integration (sync Vault → K8s Secret)
- [ ] Web UI: Secret management page (key-value form)
- [ ] Golden Path UI: wizard "Create Service" ครบ 5 ขั้นตอน

**Definition of Done Phase 5:**
> Demo full flow: กรอก form เดียว → ได้ repo + CI/CD + Helm + monitoring + secret พร้อม → service live ใน 5 นาที

---

## File Structure เป้าหมาย

```
kuberLaunch/
├── api/                          Go (gin) Platform API
│   ├── cmd/server/main.go
│   ├── config/config.go          env vars
│   ├── internal/
│   │   ├── handler/              HTTP handlers (projects, deployments, secrets)
│   │   ├── service/              business logic
│   │   ├── model/                DB models
│   │   ├── repository/           DB queries
│   │   ├── github/               GitHub API client
│   │   ├── argocd/               ArgoCD API client
│   │   ├── vault/                Vault API client
│   │   └── generator/            template engine
│   └── migrations/               SQL migration files
│
├── web/                          Next.js + TypeScript Portal
│   └── app/
│       ├── projects/             project list + create
│       ├── projects/[id]/        project detail + deploy
│       └── projects/[id]/secrets/
│
├── templetes/                    Go text/template files
│   ├── go/
│   │   ├── Dockerfile.tmpl
│   │   ├── ci.yml.tmpl
│   │   └── helm/
│   ├── nestjs/
│   ├── nextjs/
│   └── fastapi/
│
├── k8s/                          Local Kind cluster configs
│   ├── kind-config.yaml
│   └── argocd/                   ArgoCD install + initial config
│
├── scripts/
│   ├── setup-kind.sh             bootstrap local cluster
│   └── install-stack.sh          prometheus + loki + vault via Helm
│
├── docs/
│   ├── PLAN.md                   (ไฟล์นี้)
│   └── adr/                      Architecture Decision Records
│
└── docker-compose.yml            api + postgres + pgadmin (local dev)
```

---

## Tech Stack (ยืนยัน)

| Component | เทคโนโลยี | หมายเหตุ |
|-----------|-----------|---------|
| Platform API | Go 1.25 + gin | ใช้ได้เลย |
| Database | **PostgreSQL** | เปลี่ยนจาก MongoDB |
| Frontend | Next.js + TypeScript | ใช้ได้เลย |
| Local K8s | Kind | เริ่มที่นี่ก่อน |
| GitOps | ArgoCD | install บน Kind |
| CI/CD | GitHub Actions | ใช้ GHCR ฟรี |
| Secret | HashiCorp Vault | dev mode ก่อน |
| Monitoring | kube-prometheus-stack | Helm chart |
| Logging | Grafana Loki | Helm chart |
| IaC | Terraform | Phase 2+ |

---

## สิ่งที่ต้องทำแรกสุด (เริ่มพรุ่งนี้)

1. แก้ `go.mod` — เอา MongoDB ออก, เพิ่ม `lib/pq` หรือ `pgx`
2. เขียน `docker-compose.yml` — api + postgres + pgadmin
3. สร้าง `api/internal/model/project.go` — Project struct
4. สร้าง `api/migrations/001_create_projects.sql`
5. เขียน Dockerfile.tmpl สำหรับ Go ก่อน 1 runtime
6. ทำ `POST /api/v1/projects` → generate → return zip

**ห้ามไปแตะ Phase 2 จนกว่า Phase 1 จะ demo ได้จริง**
