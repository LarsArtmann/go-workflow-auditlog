package main

import (
	"fmt"

	auditlog "github.com/larsartmann/go-workflow-auditlog"
)

func runSchema(_ []string) error {
	fmt.Println(auditlog.JSONSchema())
	return nil
}
