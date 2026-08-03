# Apache

[English](README.md) | [中文](README-zh.md)

This project is for assisting with my personal work at the Apache Software Foundation.

## CI

### [incubator-seata.yml](.github/workflows/incubator-seata.yml)

> Used to verify the release voting of [incubator-seata](https://github.com/apache/incubator-seata)

> Since downloading files from https://dist.apache.org/repos/dist/ is slow on my personal network, but GitHub pages are
> accessible normally.

> To reduce repetitive work for each vote, this CI was created.

#### download

Download release artifacts (binary, source, signatures, checksums, KEYS) and upload as shared artifacts for subsequent
jobs.

#### gpg

Import KEYS, verify GPG signatures of the binary and source packages.

#### sha512sum

Verify SHA-512 checksums of the binary and source packages using `sha512sum -c`.

#### check-license

Extract the binary tarball and inspect the LICENSE file for server / namingserver modules.

#### check-notice

Extract the binary tarball and validate the NOTICE file for server / namingserver modules:

- Line 1: `Apache Seata (Incubating)`
- Line 2: `Copyright 2023-{current year} The Apache Software Foundation`

#### check-compile-license

Obtain source code from different sources (zip / tag / branch), compile on JDK 8 / 17 / 21 / 25 and export the
dependency tree. The Go program runs only on JDK 25, checking whether the LICENSE file contains every dependency from
the dependency tree.

#### check-eyes-license

Obtain source code from different sources (zip / tag / branch), use Apache SkyWalking-Eyes to check license headers and
dependency license compliance.

#### check-sha

Checkout tag / branch and verify that the commit SHA matches the expected value.

## [main.go](main.go)

Parses the dependency tree output from `mvn dependency:tree`, resolves each dependency's Maven coordinates, reads the
corresponding POM file from the local Maven repository to extract license information, then checks whether each
dependency is declared in the LICENSE whitelist file.

### Usage

```shell
go run main.go --file=tree.txt --license-file=LICENSE --exclude-group=org.apache.seata
```

### Parameters

| Parameter            | Short | Default    | Description                                                              |
|----------------------|-------|------------|--------------------------------------------------------------------------|
| `--file`             | `-f`  | `tree.txt` | Path to the `mvn dependency:tree` output file                            |
| `--license-file`     | `-lf` | `LICENSE`  | Path to the license whitelist file                                       |
| `--exclude-group`    | `-eg` | —          | Exclude dependencies matching this Maven groupId (repeatable)            |
| `--exclude-artifact` | `-ea` | —          | Exclude dependencies matching this Maven groupId:artifactId (repeatable) |
| `--check-version`    | `-cv` | `true`     | Include version in license key matching                                  |
| `--skip-test`        | `-st` | `false`    | Skip dependencies with test scope                                        |

### Flow

1. Parse dependency lines (prefixed with `+- ` or `\- `) from the `mvn dependency:tree` output
2. Parse Maven coordinates (groupId:artifactId:type:version:scope)
3. Read the corresponding POM file from the local Maven repository, extract license information (recursively look up
   parent POM if not declared)
4. Deduplicate, exclude specified groupIds and groupId:artifactIds, and perform substring matching against the whitelist file
5. Output all unmatched dependencies and return an error
