# Doctor

`wsectl doctor` provides actionable setup diagnostics without contacting Worksection.

```bash
wsectl doctor
wsectl doctor --json
```

Offline checks cover:

- Config location, parsing, validation, and file permissions.
- Active profile and HTTPS account URL.
- Auth type and secret-reference parsing.
- Secret backend availability and required credential presence.
- OAuth expiry and refreshability.
- Plaintext-store warnings.
- Rate-limit and timeout validity.

Use `--api` to add one authenticated `me` request:

```bash
wsectl doctor --api
wsectl doctor --api --json
```

Text output uses `[ok]`, `[warn]`, and `[fail]` plus remediation commands. JSON includes the full report in `data` and an error body when failing checks make the setup unhealthy. Machine-readable doctor output is a single parseable envelope and does not append an extra plaintext error line.

Exit classifications use the normal CLI contract:

```text
2 config or profile
3 credentials
4 authorization
5 network
6 Worksection API
7 rate limit
```

Doctor never prints token or client-secret values.
