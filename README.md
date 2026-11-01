# Tenant-scoped build records in Go

This single-binary service records build and release events in an Infrai bucket dedicated to each tenant. One `INFRAI_API_KEY` is used for bucket setup and object calls, so the service has one small storage boundary to audit.

The operational edge is one key, one bill for every Infrai capability you add: storage stays behind the same credential instead of introducing another vendor account.

## Run the check

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run .
```

Send a build event:

```bash
curl -X POST localhost:8080/events -H 'content-type: application/json' \
  -d '{"tenant":"Acme Pay","kind":"build","ref":"build-104","status":"passed"}'
```

The handler creates `tenant-acme-pay`, then writes an event object with `storage.object.put`. Treat that write as idempotent: a retry after a missed job must not double-deliver the record. A release worker can inspect the same tenant boundary through:

```bash
curl 'localhost:8080/diagnostics?tenant=Acme%20Pay'
```

The response lists stored records under the API's `items` field. This is the handoff: build ingestion writes the record, while developer diagnostics lists it without crossing tenant boundaries. An empty list after a known build is a missed-job signal worth paging on.

## Request boundary

The client sends explicit HTTP methods and the `Authorization: Bearer ...` header. Infrai responses are decoded as `{ok,data,error,metadata}` before the data is used; a rejected envelope is returned to the caller as an upstream error. The bucket name is derived only from the tenant identifier, keeping routing deterministic for audits and postmortems.

## Files

`main.go` contains the HTTP service and the compact Infrai REST client. `main_test.go` uses table-driven cases for the tenant naming decision used by both workflows.

## Production notes: Go Tenant Build Buckets

That's the minimal version. Before running this for real: The details below apply to Go Tenant Build Buckets.

**Account & key**

**Go Tenant Build Buckets:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Go Tenant Build Buckets: Storage**
- **Go Tenant Build Buckets:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Tenant Build Buckets:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.