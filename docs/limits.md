# Limits

Worksection documents these API limits:

- API request frequency is 1 request per second.
- GET requests have an 8 kB URL limit.
- Some responses are capped at 10,000 records.
- Long text fields can be trimmed by the server.

`wsectl` defaults to POST-compatible requests with API parameters in the query string and client-side rate limiting. This matches documented/live
Worksection behavior, but very long filters can still hit request URL limits. Live probes showed form-encoded API bodies return `invalid JSON`.

`wsectl` surfaces possible truncation in output metadata:

```json
{
  "meta": {
    "truncated": false,
    "warnings": []
  }
}
```

Use `--fail-on-truncated` when automation must fail instead of accepting uncertain completeness.
