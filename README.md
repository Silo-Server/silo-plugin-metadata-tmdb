# TMDB Metadata Plugin for Silo

First-party [Silo](https://github.com/Silo-Server/silo-server) metadata plugin
backed by The Movie Database. It provides movie, series, season, and episode
metadata and resolves `tmdb://` artwork references.

## Setup

TMDB Metadata is installed as a default Silo plugin. Add or enable **TMDB** in a
movie or television library's metadata provider chain. No configuration is
required for direct TMDB access.

### Silo metadata proxy

The plugin's **Configure** tab has a *Silo Metadata Proxy* section. Turning it on
routes every TMDB request through the shared proxy at
`https://metadata.siloserver.org` (or a self-hosted proxy URL you supply). The
proxy holds the project credential and caches responses for all Silo
installations, so scans reach TMDB far less often and no API key leaves your
server. Saving the setting reloads the plugin; no server restart is needed.

## Dependency Model

This repository consumes `github.com/Silo-Server/silo-plugin-sdk` as a normal Go module dependency. CI and release builds run with `GOWORK=off` and expect the SDK version in `go.mod` to resolve from a published semver tag.

For local multi-repository development, use a `go.work` file that points at a
sibling SDK checkout. Do not commit machine-local filesystem replacements.

## Development

```sh
go test ./...
go build .
```

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. Matching,
mapping, or capability changes should start as an issue.

## Attribution

This product uses the TMDB API but is not endorsed or certified by TMDB. All metadata and images sourced from this plugin are provided by [The Movie Database (TMDB)](https://www.themoviedb.org/).

<a href="https://www.themoviedb.org/">
  <img src="https://www.themoviedb.org/assets/2/v4/logos/v2/blue_short-8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c.svg" alt="TMDB Logo" width="200">
</a>

## License

`silo-plugin-metadata-tmdb` is licensed under `AGPL-3.0-or-later`. See [LICENSE](LICENSE).
