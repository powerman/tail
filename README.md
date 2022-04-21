# Go package tail implements behaviour of `tail` tool

[![Go Reference](https://pkg.go.dev/badge/github.com/powerman/tail.svg)](https://pkg.go.dev/github.com/powerman/tail)
[![CI/CD](https://github.com/powerman/tail/workflows/CI/CD/badge.svg?event=push)](https://github.com/powerman/tail/actions?query=workflow%3ACI%2FCD)
[![Coverage Status](https://coveralls.io/repos/github/powerman/tail/badge.svg?branch=master)](https://coveralls.io/github/powerman/tail?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/powerman/tail)](https://goreportcard.com/report/github.com/powerman/tail)
[![Release](https://img.shields.io/github/v/release/powerman/tail)](https://github.com/powerman/tail/releases/latest)

Go package tail implements behaviour of `tail -n 0 -F path` to follow
rotated log files using polling.

Most existing solutions for Go have race condition issues and occasionally
may lose lines from tracked file - such bugs are hard to fix without
massive changes in their architecture, so it turns out to be easier to
reimplement this functionality from scratch to make it work reliable and
don't lose data.

This package tries to log messages in same way as `tail`.

Unlike `tail` tool it does track renamed/removed file contents up to the
moment new file will be created with original name - this ensure no data
will be lost in case log rotation is done by external tool (i.e. not the
one which write to log file) and thus original log file may be appended
between rename/removal and reopening.

Unlike `tail` it does not support file truncation. While this can't work
reliable, truncate support may be added in the future.

## Installation

```
go get github.com/powerman/tail
```
