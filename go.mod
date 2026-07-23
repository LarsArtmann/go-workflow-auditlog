module github.com/larsartmann/go-workflow-auditlog

go 1.26.4

require (
	github.com/Azure/go-workflow v0.1.13
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/larsartmann/go-atomic-write v0.0.0-00010101000000-000000000000
	github.com/larsartmann/go-error-family v0.7.0
	github.com/larsartmann/go-ndjson v0.0.0-00010101000000-000000000000
)

require (
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/larsartmann/auditlog-core => ../auditlog-core

replace github.com/larsartmann/go-ndjson => ../go-ndjson

replace github.com/larsartmann/go-atomic-write => ../go-atomic-write
