<!-- SPDX-License-Identifier: MIT -->
# Kosmoo

[![Build Status](https://github.com/mercedes-benz/kosmoo/workflows/.github%2Fworkflows%2Fci.yml/badge.svg)](https://github.com/mercedes-benz/kosmoo/actions?query=workflow%3A.github%2Fworkflows%2Fci.yml)
[![Release Status](https://github.com/mercedes-benz/kosmoo/workflows/release/badge.svg)](https://github.com/mercedes-benz/kosmoo/actions?query=workflow%3Arelease)

*Kosmoo* exposes metrics about:
* [Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) by queries to the Kubernetes API
* [Cinder](https://docs.openstack.org/cinder/latest/) and its Disks by queries to the OpenStack API combined with data from the Kubernetes API
* [Neutron Floating IPs](https://docs.openstack.org/api-ref/network/v2/index.html#floating-ips-floatingips)
* [Load balancers](https://docs.openstack.org/api-ref/load-balancer/)

## Installation

### Building from source

To build the exporter from the source code yourself you need to have a working Go environment with [version 1.12 or greater installed](https://golang.org/doc/install).

```
$ mkdir -p $GOPATH/src/github.com/mercedes-benz
$ cd $GOPATH/src/github.com/mercedes-benz
$ git clone https://github.com/mercedes-benz/kosmoo.git
$ cd kosmoo
$ make build
```

The Makefile provides several targets:

* *build:*  build the `kosmoo` binary
* *docker:* build a docker container for the current `HEAD`
* *fmt:* format the source code
* *test:* runs the `vet`, `lint` and `fmtcheck` targets
* *vet:* check the source code for common errors
* *lint:* does source code linting
* *fmtcheck:* check the source code for format findings
* *version:* prints the version tag

## Usage

```
$ ./kosmoo -h
Usage of ./kosmoo:
  -addr string
        Address to listen on (default ":9183")
  -alsologtostderr
        log to standard error as well as files
  -cloud-conf string
        path to the cloud.conf file. If this path is not set the scraper will use the usual OpenStack environment variables.
  -kubeconfig string
        Path to the kubeconfig file to use for CLI requests. (uses in-cluster config if empty)
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory
  -log_file string
        If non-empty, use this log file
  -log_file_max_size uint
        Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
  -logtostderr
        log to standard error instead of files (default true)
  -refresh-interval int
        Interval between scrapes to OpenStack API (default 120s) (default 120)
  -skip_headers
        If true, avoid header prefixes in the log messages
  -skip_log_headers
        If true, avoid headers when openning log files
  -stderrthreshold value
        logs at or above this threshold go to stderr (default 2)
  -v value
        number for the log level verbosity
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```

## Deployment to Kubernetes

*kosmoo* can get deployed as a deployment. See the [instructions](kubernetes/) how to get started.
You can also use the docker images under [packages](https://github.com/mercedes-benz/kosmoo/packages), 
see also [authenticating-to-github-package-registry](https://help.github.com/en/articles/configuring-docker-for-use-with-github-package-registry#authenticating-to-github-package-registry).



## Metrics

Metrics will be made available on port 9183 by default, or you can pass the commandline flag `-addr` to override the port.
An overview and example output of the metrics can be found in [metrics.md](docs/metrics.md).

## Alert Rules

In combination with [Prometheus](https://prometheus.io/) it is possible to create alerts from the metrics exposed by the `kosmoo`.
An example for some alerts can be found in [alerts.md](docs/alerts.md).

## Contributing

We welcome any contributions.
If you want to contribute to this project, please read the [contributing guide](CONTRIBUTING.md).

## License

Full information on the license for this software is available in the [LICENSE](LICENSE) file.

# Provider Information

Please visit https://www.mercedes-benz-techinnovation.com/en/imprint/ for information on the provider.

Notice: Before you use the program in productive use, please take all necessary precautions, e.g. testing and verifying the program with regard to your specific use. The program was tested solely for our own use cases, which might differ from yours.
