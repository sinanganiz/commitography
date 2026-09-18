package main

import "testing"

func TestServeFlagsDefaultToLoopbackWithoutBrowser(t *testing.T) {
	flags := newServeCommand(testEnvironment()).Flags()
	for name, want := range map[string]string{
		"listen":       "127.0.0.1:8080",
		"open":         "false",
		"allowed-root": "[]",
	} {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Fatalf("serve has no --%s flag", name)
		}
		if flag.DefValue != want {
			t.Errorf("--%s defaults to %q, want %q", name, flag.DefValue, want)
		}
	}
}
