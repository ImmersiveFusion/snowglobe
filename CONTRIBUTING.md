# Contributing to Snowglobe

Thanks for your interest in Snowglobe, a single-binary OpenTelemetry trace generator. Bug reports, feature ideas, code, and docs are all welcome.

## Ways to contribute

- **Report a bug** or **request a feature**: open an issue with one of the [issue templates](.github/ISSUE_TEMPLATE).
- **Add your deployment**: running Snowglobe somewhere? Add it to [`WHERE-SNOWGLOBE-RUNS.md`](WHERE-SNOWGLOBE-RUNS.md) via a pull request, or use the "Add a deployment" issue template and we'll add it for you.
- **Send a pull request**: see below.

## Building from source

Snowglobe is a single Go module with no runtime dependencies.

```bash
git clone https://github.com/ImmersiveFusion/snowglobe.git
cd snowglobe
go build -o snowglobe ./cmd/snowglobe
./snowglobe -insecure   # send to a local collector on localhost:4317
```

Cross-compile for another platform:

```bash
GOOS=linux   GOARCH=arm64 go build -o snowglobe     ./cmd/snowglobe
GOOS=darwin  GOARCH=arm64 go build -o snowglobe     ./cmd/snowglobe
GOOS=windows GOARCH=amd64 go build -o snowglobe.exe ./cmd/snowglobe
```

## Pull requests

1. Fork the repo and branch from `main`.
2. Keep changes focused: one logical change per PR.
3. Run `go build ./...` and `go vet ./...` before pushing.
4. Use clear, conventional commit messages (`feat:`, `fix:`, `docs:`, …).
5. **Sign every commit** (see below). One unsigned commit blocks the merge.
6. Open the PR against `main` and describe what changed and why.

## Commit signing and branch rules

`main` requires a **verified signature on every commit**, so a single unsigned commit
anywhere in your branch blocks the merge even after the change is approved. Commits made
in the GitHub web editor are signed automatically; commits pushed from your own machine
are not, unless you have set that up. If you already push over SSH, reuse that key:

```bash
git config --global gpg.format ssh
git config --global user.signingkey ~/.ssh/id_ed25519.pub
git config --global commit.gpgsign true
```

Then add that same public key to <https://github.com/settings/keys> a second time as a
**signing** key. `git log --show-signature -1` confirms it locally, and every commit in
the pull request should show a green **Verified** badge.

Already pushed something unsigned? `git rebase --exec 'git commit --amend --no-edit -S' main`
re-signs the branch, then force-push it with lease.

The rest of what `main` enforces: linear history (rebase onto `main`, do not merge it
into your branch), one approving review, all review threads resolved, and squash or
rebase merges only. A new push dismisses existing approvals, so batch your review fixes
into one push where you can.

The full version, including the GPG route, is in the
[organization contribution guide](https://github.com/ImmersiveFusion/.github/blob/main/CONTRIBUTING.md).

## Releasing

Releases are cut by pushing a version tag. The CI workflow
([`.github/workflows/release.yml`](.github/workflows/release.yml)) builds the
cross-platform binaries, publishes a GitHub Release, and pushes the multi-arch
container image to Docker Hub.

**Tag format: use the `v`-prefixed form, e.g. `v0.7.4`.** This is the canonical
scheme going forward (it matches the Go ecosystem and what GoReleaser expects).

```bash
git tag v0.7.4
git push origin v0.7.4
```

The trigger also still accepts the older bare-number form (`0.7.4`) for
backward compatibility with historical tags, but new releases should always be
`v`-prefixed so the Tags/Releases lists stay consistent and sort cleanly.

Note: the workflow evaluates the tag trigger **at the moment the tag is
pushed**. If a tag was pushed before a trigger fix landed on `main`, fixing the
trigger does not retroactively run it. Push a new (higher) version tag instead.

## Reporting issues

Use the issue templates. For bugs, include your platform/architecture, the exact `snowglobe` command and flags, and what you expected versus what happened. `-log-level debug` gives more detail.

## Code of conduct

Be decent. We follow the spirit of the [Contributor Covenant](https://www.contributor-covenant.org/): no harassment, assume good faith, keep it about the work.

## License

By contributing, you agree your contributions are licensed under the repository's [Apache-2.0 license](LICENSE).
