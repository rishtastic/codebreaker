# Codebreaker

The Codebreaker game service.

It listens privately on `127.0.0.1:8081`. Nginx on the VPS will expose it at
`https://rishabhdaga.com/codebreaker/`.

## Local run

```sh
go run ./cmd/codebreaker
```

Then open `http://127.0.0.1:8081/`.
