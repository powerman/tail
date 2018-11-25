# tail [![GoDoc](https://godoc.org/github.com/powerman/tail?status.svg)](http://godoc.org/github.com/powerman/tail) [![Go Report Card](https://goreportcard.com/badge/github.com/powerman/tail)](https://goreportcard.com/report/github.com/powerman/tail) [![CircleCI](https://circleci.com/gh/powerman/tail.svg?style=svg)](https://circleci.com/gh/powerman/tail) [![Coverage Status](https://coveralls.io/repos/github/powerman/tail/badge.svg?branch=master)](https://coveralls.io/github/powerman/tail?branch=master)

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
