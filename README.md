# Silo TMDB Plugin

First-party Silo metadata plugin backed by TMDB.

## Dependency Model

This repository consumes `github.com/Silo-Server/silo-plugin-sdk` as a normal Go module dependency. CI and release builds run with `GOWORK=off` and expect the SDK version in `go.mod` to resolve from a published semver tag.

For local multi-repo development, use a temporary `replace` or a local `go.work` that points at `dev/github/silo-plugin-sdk`. Do not commit machine-local filesystem replaces as the supported release path.

## Development

```sh
go test ./...
go build .
```

## Attribution

This product uses the TMDB API but is not endorsed or certified by TMDB. All metadata and images sourced from this plugin are provided by [The Movie Database (TMDB)](https://www.themoviedb.org/).

<a href="https://www.themoviedb.org/">
  <img src="https://www.themoviedb.org/assets/2/v4/logos/v2/blue_short-8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c.svg" alt="TMDB Logo" width="200">
</a>

## License

`silo-plugin-tmdb` is licensed under `AGPL-3.0-or-later`. See [LICENSE](LICENSE).
