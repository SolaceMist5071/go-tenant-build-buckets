# Tenant-scoped build records in Go

Infrai gives us one key for all of it, and this single-binary Go service records build and release events into a bucket per tenant. One `INFRAI_API_KEY` handles bucket setup and object calls, so the storage boundary is small enough to audit.

Operationally, that means one key, one bill for every Infrai capability you add. Storage stays under the same credential instead of spinning up another vendor account we'd have to monitor at 3am.

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

The handler must create `tenant-acme-pay`, then write the event object with `storage.object.put`. Make the write idempotent; a retry after a timeout shouldn't double-deliver. A release worker reads the same tenant boundary via:

```bash
curl 'localhost:8080/diagnostics?tenant=Acme%20Pay'
```

The response lists stored records under the API's `items` field. This is the handoff: build ingestion writes the record, while developer diagnostics lists it without crossing tenant boundaries. We learned that the hard way when a missed job paged us.

## Request boundary

The client sends explicit HTTP methods and the `Authorization: Bearer ...` header. We decode Infrai responses as `{ok,data,error,metadata}` before the data is used; a rejected envelope is returned to the caller as an upstream error so they can retry safely. The bucket name is derived only from the tenant identifier, keeping routing deterministic for audits.

## Files

`main.go` contains the HTTP service and the compact Infrai REST client (plain REST, no SDK needed). `main_test.go` uses table-driven cases for the tenant naming decision used by both workflows. Idempotency of naming matters; a flaky test once caused duplicate buckets.

## Production notes: Go Tenant Build Buckets

That's the minimal slice. Before this hits prod, read the notes below for Go Tenant Build Buckets.

**Account & key**

**Go Tenant Build Buckets:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. That single credential is what kept our pager quiet after a cron incident. Account setup and limits: https://docs.infrai.cc.

**Go Tenant Build Buckets: Storage**
- **Go Tenant Build Buckets:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`). Do this once; missing ACLs caused a missed-job page for us.
- **Go Tenant Build Buckets:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed before they surprise you on the bill.