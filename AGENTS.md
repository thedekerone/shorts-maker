# AGENTS
1. Repo targets Go 1.23.3; keep modules tidy and commit go.sum changes.
2. Build locally via `make build` (CGO enabled) to produce `video-processor`.
3. Run the binary with `make run`; Docker builds use `docker build -t shorts-maker .`.
4. Full tests: `make test` which runs `CGO_ENABLED=1 go test ./...`.
5. Single test example: `CGO_ENABLED=1 go test ./engine -run TestTextEngine`.
6. Lint/verify using `go vet ./...` plus optional `staticcheck ./...` if installed.
7. Format with `gofmt -w` and `goimports -w`; do not mix tabs/spaces manually.
8. External deps (ffmpeg, fonts) must exist locally for video/image engines; Kling clips come from `kwaivgi/kling-v2.1` with 5s/10s durations and reuse the Replicate-generated start image (portrait -> standard, landscape -> pro).
9. Layout: `cmd/server` entrypoint, `handlers` HTTP, `services` integrations, `engine` media—add code in the matching layer.
10. Organize imports into stdlib / third-party / internal groups separated by blank lines and alphabetized.
11. Exported identifiers use PascalCase, unexported camelCase; avoid stutter like `video.VideoService`.
12. Prefer concrete structs, introduce interfaces only when multiple implementations or tests need them.
13. Pass `context.Context` as the first parameter on request-scoped helpers and honor cancellation/timeouts.
14. Validate inputs early (see handlers/video.go) and respond with `http.StatusBadRequest` plus helpful messages.
15. Handle errors with `%w` wrapping, log via `log.Printf`, and avoid panics except during process startup.
16. Keep bucket names, transition types, and magic numbers as typed consts in their owning package.
17. When spawning commands (ffmpeg) build argument slices explicitly and always check `cmd.Run` errors.
18. Tests favor table-driven layouts; share setup with helpers (e.g., pkg/ass_test.go) and keep fixtures nearby.
19. There are no Cursor or Copilot rule files; update this document when conventions change.
20. Document new workflows (env vars, services, scripts) as they appear so future agents stay aligned.
