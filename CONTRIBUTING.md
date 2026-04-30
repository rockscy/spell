# Contributing

Thanks for your interest. `spell` is small on purpose — please keep changes minimal and focused.

## Development

```sh
git clone https://github.com/rockscy/spell && cd spell
make build         # ./bin/spell
make test
```

You'll need Go 1.22 or newer.

## Guidelines

- **Keep dependencies few.** The binary should stay under ~12 MB. New runtime deps need a clear justification.
- **One feature per PR.** Refactors, formatting changes, and feature work should not be mixed.
- **Match the existing style.** `gofmt -w .` before committing.
- **No telemetry, ever.** `spell` does not phone home. Don't change that.
- **Test what you write.** If you add a parser, write a table test for it.

## Adding a provider preset

Most providers fit through the existing `openai-compatible` adapter — just add an example block to `internal/config/config.go` (`exampleTOML`) and the provider matrix in `README.md`.

If a provider needs a fundamentally different protocol (like Anthropic does), implement a new file in `internal/llm/` and wire it through `config.Build`.

## Filing issues

- **Bug reports:** include the provider, model, and the exact natural-language query that misbehaved.
- **Feature requests:** describe the workflow you want, not the implementation. We'll figure out the implementation together.

## License

By contributing, you agree your contributions are licensed under the [MIT License](LICENSE).
