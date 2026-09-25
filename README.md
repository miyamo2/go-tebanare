# go-tebanare

go-tebanare hides Go code from the "Files changed" tab of GitHub pull requests when your team has agreed that the code needs no line-by-line review. You enable built-in presets in `.gotebanare.yml` in your repository:

- `getter` for methods that only return a field of the receiver
- `noop` for methods that take nothing, return nothing, and do nothing
- `iferr` for `if err != nil` blocks that return the error unchanged

go-tebanare parses the changed Go files with the standard `go/parser` and leaves code visible whenever it cannot be sure, for example when a file does not parse or a getter gained logic in the pull request.

## Status

The Chrome extension is in development.

## Configuration

A minimal `.gotebanare.yml` at the root of the repository:

```yaml
version: 1
presets:
  - getter
  - noop
  - iferr
```

The extension reads the configuration from the base branch of the pull request, so a pull request cannot change the configuration that applies to it.

## Documentation

- [Configuration reference](docs/configuration.md)
- [Presets](docs/presets.md)
- [Design](docs/design.md) and [architecture decision records](docs/adr/README.md)

## License

MIT. See [LICENSE](LICENSE).
