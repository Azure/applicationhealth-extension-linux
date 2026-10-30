# Integration Tests

This directory contains instructions and test files on how to test the extension
binary in an environment that looks like an Azure Linux Virtual Machine.

## Requirements

- Install _Docker Engine on Linux_ or _Docker for Mac_.
- Install [Bats](https://github.com/sstephenson/bats).

## Testing Strategy

The integation tests use `bats` to run the scenarios as bash scripts.

To test the extension handler functionality, we simply:

- build a Docker image using test.Dockerfile
    - copy some files to make it look like a `/var/lib/waagent` dir
    - copy extension binary into the container
- remove the `test` container if it exists
- create a Docker container (name: `test`) from image
    - specify which handler subcommand will be invoked (e.g. `fake-waagent
      install`)
- push .settings file
    - do other things on the container that we need to craft the environment
- start the container
- collect the output from the command execution
- validate using the following:
    - check status code
    - validate output of the command
    - `docker diff test` to validate file changes in the container
    - copy files out of container and validate their contents

## Running Tests

To run the integration tests, run the following commands from the repository
root:

```
make binary
./integration-test/run.sh
```

If you are running on a linux host (a real linux host not WSL), you can use the command `./integration-test/run.sh --all` to run all the tests.

## Localhost fallback regression tests

TCP, HTTP, and HTTPS probes retain normal hostname dialing and supplement a
missing loopback family only when `localhost` resolves exclusively to IPv4 or
IPv6 loopback addresses. TLS and HTTP response failures do not trigger fallback;
rich-state JSON validation and custom metrics are unchanged.

Run the dialer tests and Linux probe scenarios from the repository root:

```sh
CGO_ENABLED=0 go test -count=1 -v ./main -run 'Test.*[Ll]oopback'
```

The Linux scenarios require `unshare`, `ip`, `tc`, and permission to create user,
network, and mount namespaces. They modify hosts mappings and IPv6 settings only
inside isolated namespaces. They skip with `-short` or unavailable prerequisites;
check that the scenario tests execute, rather than skip, when validating a fix.

Coverage includes IPv4-only and IPv6-only listeners, missing-family hosts
mappings, dropped/disabled IPv6, alternate loopback mappings, slow DNS, endpoint
failure, invalid rich-state JSON, and explicit Unhealthy responses. Literal-IP
controls verify endpoint reachability independently of localhost resolution.
