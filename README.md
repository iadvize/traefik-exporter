# Traefik Exporter [![Build Status](https://travis-ci.org/iadvize/traefik_exporter.svg)][travis]

Export Traefik service health to Prometheus.

## Examples

To run it:

```bash
make
./traefik_exporter [flags]
```

## Documentation

### Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| traefik_up | Was the health check Traefik successful ? | |
| traefik_uptime | What is the Traefik uptime ? | |
| traefik_request_count_current | How many request is Traefik currently managing ? | 100, 101, 102, 200, 201... |
| traefik_request_count_total | How many request Traefik managed until now ? | 100, 101, 102, 200, 201... |
| traefik_request_response_time_total | Cummulated Traefik response time | |
| traefik_request_response_time_avg | Average Traefik response time | |

### Flags

```bash
./traefik_exporter --help
```

* __`-log.format`:__ If set use a syslog logger or JSON logging. Example: `logger:syslog?appname=bob&local=7` or `logger:stdout?json=true`. Defaults to stderr`.
* __`-log.level`:__ Only log messages with the given severity or above. Valid levels: `[debug, info, warn, error, fatal]`. (default `info`)
* __`-timeout`:__ Timeout for trying to get stats from Traefik. (in seconds, default 5s)
* __`-traefik.address`:__ HTTP API address of a Traefik or agent. (default `http://localhost:8080/health`)
* __`-version`:__ Print version information.
* __`-web.listen-address`:__ Address to listen on for web interface and telemetry. (default `:9000`)
* __`-web.telemetry-path`:__ Path under which to expose metrics. (default `/metrics`)

### Useful Queries

__Are every Traefik instances up ?__

     avg(traefik_up)

Value of 1 mean that all nodes for the service are passing. Value of 0 mean no node is running. Value between 0 and 1 means at least one instance of traefik is not passing.

__How many request Traefik is currently handling ?__

    sum by (statusCode)(traefik_request_count_current)

## Install

### Using Docker

You can deploy this exporter using the [iadvize/traefik-exporter](https://registry.hub.docker.com/u/iadvize/traefik-exporter/) Docker image.

For example:

```bash
docker pull traefik
docker pull iadvize/traefik-exporter

docker run -d -v /var/run/docker.sock:/var/run/docker.sock --name traefik traefik --docker --web --web.address :8080
docker run -d -p 9000:9000 iadvize/traefik-exporter -traefik=http://traefik:8080/health
```

## Contribute

Look at contribution guidelines here : [CONTRIBUTING.md](CONTRIBUTING.md)
