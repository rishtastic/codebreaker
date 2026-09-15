# Codebreaker

The Codebreaker game service.

It listens privately on `127.0.0.1:8081`. Nginx on the VPS will expose it at
`https://rishabhdaga.com/codebreaker/`.

## Local run

```sh
go run ./cmd/codebreaker
```

Then open `http://127.0.0.1:8081/`.

## Deployment

Every push to `main` is tested, cross-compiled for Linux, and deployed by the
self-hosted GitHub Actions runner on the VPS. This uses no GitHub-hosted
runner minutes and needs no repository secrets.

The workflow installs the binary at `/srv/codebreaker/bin/codebreaker` and
restarts `codebreaker.service` on the VPS.
