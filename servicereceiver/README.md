# System Service Receiver

The system service receiver reports the state of the services running on the local host, using systemd on Linux and the Service Control Manager on Windows. Both platforms produce the same metric with the same value range, so a single dashboard or alert works everywhere.

| Status                |                            |
| --------------------- | -------------------------- |
| Stability             | [alpha]: metrics           |
| Unsupported Platforms | darwin                     |
| Distributions         | none (build with [ocb])    |
| Issues                | [Open issues](https://github.com/pierre-prevoteau/otel-receiver-service/issues) |
| Code Owners           | [@pierre-prevoteau](https://www.github.com/pierre-prevoteau) |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[ocb]: https://opentelemetry.io/docs/collector/custom-collector/

On every collection interval the receiver asks the host's service manager for the state of each service and emits one data point per service:

```
system.service.state{system.service.name="nginx"} = 4
system.service.state{system.service.name="cron"}  = 1
```

The value is a platform independent code:

| Value | State    | Meaning                                                     |
| ----- | -------- | ----------------------------------------------------------- |
| `0`   | unknown  | The service manager reported a state the receiver does not recognize. |
| `1`   | stopped  | Not running, and stopped cleanly.                           |
| `2`   | starting | Transitioning to running.                                   |
| `3`   | stopping | Transitioning to stopped.                                   |
| `4`   | running  | Running (including while reloading its configuration).      |
| `5`   | paused   | Suspended.                                                  |
| `6`   | failed   | Not running, after terminating abnormally.                  |

See [documentation.md](./documentation.md) for the generated metric reference.

## How the state is determined

On Linux the receiver connects to systemd over D-Bus and calls `ListUnitsByPatterns` for `*.service`, then maps each unit's `ActiveState`:

| `ActiveState`                       | Value            |
| ----------------------------------- | ---------------- |
| `active`, `reloading`, `refreshing` | `4` running      |
| `activating`                        | `2` starting     |
| `deactivating`                      | `3` stopping     |
| `inactive`                          | `1` stopped      |
| `maintenance`                       | `5` paused       |
| `failed`                            | `6` failed       |

A unit that is reloading is still serving requests, so it is reported as running rather than as a transitional state. systemd only knows about loaded units, so a service that has never been loaded since boot is not reported at all.

On Windows the receiver opens the Service Control Manager, enumerates the services and maps each `SERVICE_STATUS`:

| `dwCurrentState`                             | Value        |
| -------------------------------------------- | ------------ |
| `SERVICE_RUNNING`                            | `4` running  |
| `SERVICE_START_PENDING`, `SERVICE_CONTINUE_PENDING` | `2` starting |
| `SERVICE_STOP_PENDING`                       | `3` stopping |
| `SERVICE_PAUSE_PENDING`, `SERVICE_PAUSED`    | `5` paused   |
| `SERVICE_STOPPED`                            | `1` stopped, or `6` failed |

Windows has no dedicated failed state: a service whose process terminated abnormally is reported as stopped with a non-zero `dwWin32ExitCode`. That case is reported as `6` failed so the metric behaves the same way as on Linux. The exit code `1077` (`ERROR_SERVICE_NEVER_STARTED`), which the Service Control Manager returns for a service that has not been started since the last boot, is not treated as a failure.

If the collector lacks the rights to open the Service Control Manager, the receiver logs a warning and reports nothing instead of failing the collector.

## Service names

`system.service.name` holds the service name without any platform decoration: on Linux the `.service` suffix of the unit name is stripped, so the `nginx.service` unit and the Windows `nginx` service are both reported as `nginx`.

## Configuration

| Field                 | Default  | Description                                                                       |
| --------------------- | -------- | -------------------------------------------------------------------------------- |
| `collection_interval` | `60s`    | How often the service states are collected.                                      |
| `initial_delay`       | `1s`     | Time to wait before the first collection.                                        |
| `include_services`    | `[]`     | Glob patterns of service names to collect. Empty collects every service.          |
| `exclude_services`    | `[]`     | Glob patterns of service names to skip. Applied after `include_services`.         |
| `scope`               | `system` | The systemd service manager to collect from, either `system` or `user`. No effect on Windows. |

Patterns use [`path.Match`](https://pkg.go.dev/path#Match) syntax (`*`, `?`, `[...]`) and are matched against the service name as reported in `system.service.name`, as well as against the full unit name on Linux. Both `nginx` and `nginx.service` therefore select the `nginx.service` unit.

## Example

### Basic configuration

In its default configuration the receiver reports every service on the host once per minute:

```yaml
receivers:
  system_service:
```

### Advanced configuration

```yaml
receivers:
  system_service:
    collection_interval: 30s
    include_services:
      - nginx
      - "postgresql*"
    exclude_services:
      - "systemd-*"
```

Collecting from the calling user's systemd manager instead of the system one:

```yaml
receivers:
  system_service:
    scope: user
```
