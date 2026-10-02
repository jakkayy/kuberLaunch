# ADR 002: Provision database และ cache ให้ service ที่ generate

**Date:** 2026-10-02
**Status:** Accepted

## Context

ฟอร์มสร้าง project ให้เลือก database (`postgres`, `mysql`, `none`) และ cache (`redis`, `none`)
แต่ก่อนหน้านี้ค่าที่เลือกไม่มีผลอะไรเลย Helm chart ไม่ได้สร้าง DB หรือ cache ให้ และไม่ได้บอก
service ว่าจะเชื่อมต่อที่ไหน ทีมต้องไปหา DB เองแล้วต่อเองทุกครั้ง

ทางเลือกที่พิจารณา:

1. **Inject connection string อย่างเดียว:** ให้ทีมสร้าง DB เอง แล้วใส่ `DATABASE_URL` ใน Vault
2. **Provision ใน cluster ด้วย operator:** chart สร้าง DB ให้ใน namespace ของแต่ละ environment
3. **Provision บน cloud (RDS/Cloud SQL) ผ่าน Crossplane/Terraform:** ต้องผูกกับ cloud provider
   ซึ่งโปรเจคนี้ยังไม่มี (รันบน Kind)

## Decision

ใช้แบบผสม: provision ให้ตอนที่ทำได้ดีใน cluster และ inject จาก Vault ในกรณีอื่น
ทุกกรณีเปิดปิดได้ราย environment ผ่าน `database.provision` / `cache.provision`

| เลือก | Provision | Connection |
|-------|-----------|------------|
| `postgres` | CloudNativePG `Cluster` ชื่อ `<slug>-db` (dev/staging: 1 instance, prod: 3 instances + 10Gi) | `DATABASE_URL` จาก Secret `<slug>-db-app` ที่ operator สร้าง |
| `mysql` | ไม่ provision | ใส่ `DATABASE_URL` ใน Vault (`secret/kuberlauncher/<slug>`) แล้วถูก inject ผ่าน External Secrets |
| `redis` | `Deployment` + `Service` ชื่อ `<slug>-redis` แบบไม่มี persistence (`allkeys-lru`) | `REDIS_URL=redis://<slug>-redis:6379` |

ตัวอย่างการใช้ managed DB ใน prod ให้ตั้งค่าใน `values-prod.yaml`:

```yaml
database:
  provision: false   # ใช้ DATABASE_URL จาก Vault แทน
```

## Reasons

1. CloudNativePG ทำ HA, failover และ backup ให้โดยไม่ผูกกับ cloud และเป็น namespaced resource
   จึงอยู่ในขอบเขตของ ArgoCD AppProject ของแต่ละ project (ไม่ต้องเปิดสิทธิ์ cluster-scope เพิ่ม)
2. Redis ที่ใช้เป็น cache ไม่ต้องเก็บข้อมูลถาวร Deployment ธรรมดาจึงพอ และไม่ต้องติดตั้ง operator เพิ่ม
3. MySQL operator ที่มีอยู่ยังไม่ลงตัวพอสำหรับใช้เป็น default จึงให้ใช้ DB ภายนอกผ่าน Vault ไปก่อน
4. ทุก environment ได้ DB ของตัวเองใน namespace ของตัวเอง ข้อมูลของ dev กับ prod จึงไม่ปนกัน

## Consequences

- Cluster ต้องติดตั้ง CloudNativePG operator ก่อน (ดูวิธีใน README) ไม่อย่างนั้น ArgoCD จะ sync
  resource `Cluster` ไม่ผ่าน
- Service ต้องอ่าน `DATABASE_URL` / `REDIS_URL` จาก environment variable
- ยังไม่มี backup ของ DB ที่ provision ให้ service ในส่วนนี้ ทีมที่ต้องการต้องเพิ่ม `backup` ใน
  `Cluster` เอง ส่วน DB ของตัว platform มี backup แยก (ดู Helm chart ของ kuberLaunch)
- Template version 5 ขึ้นไป project ที่สร้างก่อนหน้านี้จะได้ผ่าน PR อัปเกรด template
