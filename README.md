# Codebreaker

A private, real-time, two-player deduction game. One global game can be active
at a time. Players enter the shared password, then create a game or join the
waiting player.

It listens privately on `127.0.0.1:8081`. Nginx on the VPS will expose it at
`https://rishabhdaga.com/codebreaker/`.

## Local run

```sh
CODEBREAKER_PASSWORD=choose-a-password go run ./cmd/codebreaker
```

Then open `http://127.0.0.1:8081/`.

Use a private/incognito window to join as the second player during local
testing. The development session key is generated when the server starts.

## Configuration

Production requires these environment variables:

```text
CODEBREAKER_ENV=production
CODEBREAKER_PASSWORD=<the shared game password>
CODEBREAKER_SESSION_KEY=<at least 32 random bytes>
```

The VPS service reads them from `/etc/codebreaker.env`. That file must be owned
by root and must not be committed to this repository.

## Rules in v1

- Two players receive five private tiles each.
- Four question cards are available at a time.
- A player asks one question or guesses the opponent's complete code each turn.
- Answers are recorded literally. The game never fills or checks notebook marks.
- Only the player can edit their private notebook.
- A correct guess follows the two-player final-response tie rule.
- The waiting game expires after 15 minutes; an inactive game expires after 60.

## Deployment

Every push to `main` is tested, cross-compiled for Linux, and deployed by the
self-hosted GitHub Actions runner on the VPS. This uses no GitHub-hosted
runner minutes and needs no repository secrets.

The workflow installs the binary at `/srv/codebreaker/bin/codebreaker` and
restarts `codebreaker.service` on the VPS.

Feature work is delivered through pull requests. Merging an approved PR into
`main` triggers deployment.
