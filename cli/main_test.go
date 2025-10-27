package main

import "testing"

func TestResolveListenerPortPrefersStandardPortEnv(t *testing.T) {
	t.Setenv(standardPortKey, "9001")
	t.Setenv(portEnvKey, "7777")

	got := resolveListenerPort("")
	if got != "9001" {
		t.Fatalf("expected resolved port to prefer %s, got %s", standardPortKey, got)
	}
}
