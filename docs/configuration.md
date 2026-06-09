# Configuration

Configuration is loaded in three layers, each overriding the previous:

1. Built-in defaults.
2. The YAML file (path from `-config`, or `POPCORN_CONFIG`, default `config.yaml`).
3. `POPCORN_*` environment variables.

The result is validated; invalid configuration aborts startup with a clear error.

Start from [`config.example.yaml`](../config.example.yaml).

## Keys

| YAML key              | Env override                       | Default                     | Description                                              |
| --------------------- | ---------------------------------- | --------------------------- | -------------------------------------------------------- |
| `theaters[].internal_id` | (file only)                     | —                           | allocine.fr theater id (e.g. `C0159`). Required.         |
| `theaters[].name`     | (file only)                        | —                           | Display label shown in the UI. Required.                 |
| `server.host`         | `POPCORN_HOST`               | `0.0.0.0`                   | Listen address.                                          |
| `server.port`         | `POPCORN_PORT`               | `5000`                      | Listen port.                                             |
| `refresh.interval`    | `POPCORN_REFRESH_INTERVAL`   | `3h`                        | How often to refresh. Go duration (`90m`, `6h`, ...).    |
| `refresh.days`        | `POPCORN_REFRESH_DAYS`       | `7`                         | Size of the rolling window (1-31).                       |
| `allocine.base_url`   | `POPCORN_ALLOCINE_BASE_URL`  | `https://www.allocine.fr`   | Upstream base URL (override for testing).                |
| `allocine.timeout`    | (file only)                        | `10s`                       | Per-request HTTP timeout.                                |
| `allocine.max_retries`| (file only)                        | `3`                         | Extra attempts on 5xx / transport errors.                |
| `log_level`           | `POPCORN_LOG_LEVEL`          | `info`                      | `debug`, `info`, `warn`, or `error`.                     |

Theaters are file-only because a list is awkward to express in a single env variable.

## Examples

Run locally with the sample config:

```bash
go run . -config config.example.yaml
```

Override the port and refresh cadence at runtime (e.g. in a container):

```bash
POPCORN_PORT=8080 POPCORN_REFRESH_INTERVAL=30m ./popcorn
```

## Finding a theater's internal id

The `internal_id` is the `csalle=` code in an allocine.fr theater URL, e.g.
`https://www.allocine.fr/seance/salle_gen_csalle=C0159.html` -> `C0159`.
