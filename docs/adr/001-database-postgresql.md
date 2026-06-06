# ADR 001: ใช้ PostgreSQL แทน MongoDB

**Date:** 2026-06-06
**Status:** Accepted

## Context

go.mod เริ่มต้นมี `go.mongodb.org/mongo-driver` แต่ข้อมูลหลักของ IDP นี้เป็น relational:
- Project มี many Environments
- Environment มี many Deployments
- Deployment ต้องการ history และ rollback

## Decision

ใช้ PostgreSQL พร้อม `pgx` driver

## Reasons

1. Query deployment history พร้อม JOIN ทำได้ง่ายกว่า
2. Foreign key constraints ป้องกัน orphaned records
3. Helm chart ของ PostgreSQL บน Kind มี community support ดีกว่า
4. JSONB column ใช้เก็บ generated file content ได้ถ้าต้องการ

## Consequences

- ลบ `go.mongodb.org/mongo-driver` ออกจาก go.mod
- เพิ่ม `github.com/jackc/pgx/v5` และ `github.com/pressly/goose/v3` (migration)
- เขียน SQL migration files ใน `api/migrations/`
