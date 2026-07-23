module github.com/larsartmann/go-workflow-auditlog

go 1.26.4

require (
	github.com/Azure/go-workflow v0.1.13
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/larsartmann/go-error-family v0.7.0
)

require github.com/benbjohnson/clock v1.3.5 // indirect

replace github.com/larsartmann/auditlog-core => ../auditlog-core
