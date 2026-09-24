# CLI

FARM accepts a subcommand and a configuration path:

```sh
farm validate -config PATH
farm run -config PATH
farm version
```

`validate` checks the YAML structure and values. It does not read token or
certificate files or connect to Forgejo or Incus. `run` starts the controller
and checks file access and external connections. `version` prints the build
version, commit, and date.

The repository build places the executable at `dist/farm`.
