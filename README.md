# Discovery
[![Documentation](https://godoc.org/gitlab.com/tblyler/discovery?status.svg)](https://godoc.org/gitlab.com/tblyler/discovery)
`discovery` is a pure [Go](https://golang.org) library to provide simple abstractions to discover other hosts on networks.

## Documentation
The [`discovery` documentation for source code is available on GoDoc](https://godoc.org/gitlab.com/tblyler/discovery) or you can run:

```bash
godoc gitlab.com/tblyler/discovery
```

## Testing
Most tests do not require an active network. However, for the ones that do, they only utilize the loopback device and bind to an automatically assigned port via the `0` port.

```bash
go test -cover gitlab.com/tblyler/discovery
```

## Examples
Usable examples are [located here](./examples) or run:

```bash
go get gitlab.com/tblyler/discovery/examples/...
```
