## rd-mirror-sync v1.0.3

### Fixes

- **Pagination EOF on page boundary:** Real-Debrid drops the TCP connection instead of returning an empty array when the total torrent count is exactly divisible by `PAGE_LIMIT` (e.g. 500 torrents with the default `PAGE_LIMIT=250`). The client now treats EOF on any page after the first as end-of-list rather than a fatal error. Added `Unwrap()` to the internal `retryErr` type so `errors.Is` correctly traverses the error chain. Test added to cover this case.
- **`/healthz` always returns HTTP 200:** Previously returned 503 when unhealthy, which caused `customapi` widgets (e.g. homepage) to reject the response entirely. The `healthy` field in the JSON body still reflects the actual health state.

Same install and config as v1.0.2.
