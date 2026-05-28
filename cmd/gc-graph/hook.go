package main

import (
	"fmt"
	"os"
)

// runHook dispatches `gc graph hook <subcommand>`.
func runHook(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gc graph hook {pre-edit|user-prompt|list|disable|test}")
		return 2
	}
	switch args[0] {
	case "pre-edit":
		return runHookPreEdit(args[1:])
	case "user-prompt":
		return runHookUserPrompt(args[1:])
	case "list":
		return runHookList(args[1:])
	case "disable":
		return runHookDisable(args[1:])
	case "test":
		return runHookTest(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown hook subcommand: %s\n", args[0])
		return 2
	}
}
