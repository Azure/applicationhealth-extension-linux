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
bats integration-test/test
```

## Loopback probe regression scenarios

The Go tests also exercise TCP, HTTP, and HTTPS against IPv4-only and IPv6-only
listeners, single-family `localhost` entries, disabled IPv6, dropped IPv6
packets, alternate loopback mappings, and dual-stack listeners:

```
CGO_ENABLED=0 go test ./main -run '^Test_loopbackProbe(LinuxScenarios|PreservesIPv4Mapping)$' -count=1 -v
```

These Linux-only tests require `unshare`, `ip`, `tc`, and permission to create
unprivileged user, network, and mount namespaces. Each scenario runs in a child
namespace. Hosts-file mounts, IPv6 sysctls, and packet-drop rules cannot affect
the parent network or hosts file. The suite skips when namespaces are unavailable
or `-short` is specified.

Use the same Go compiler for baseline and candidate comparisons. With both
addresses returned for `localhost`, modern Go already handles dropped IPv6
packets using its default fallback. Single-family hosts entries distinguish the
explicit loopback fallback from that existing behavior. The TLS 1.3 scenario
does not simulate FIPS policy or certify a hardened customer image.

The mapping tests also provide a healthy IPv4 HTTPS listener and an
unadvertised IPv6 listener that accepts TCP but stalls TLS. They cover both a
hosts-file mapping and delayed DNS responses. The primary dial must preserve
the configured addresses, perform only one lookup, and start the supplemental
fallback delay after its own resolution completes, not during DNS resolution.
