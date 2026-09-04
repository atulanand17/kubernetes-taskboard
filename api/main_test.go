package main

import "testing"

func TestEnvUsesFallback(t *testing.T) {
	const key = "TASKBOARD_TEST_VALUE"
	t.Setenv(key, "")
	if got := env(key, "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestEnvUsesEnvironmentValue(t *testing.T) {
	const key = "TASKBOARD_TEST_VALUE"
	t.Setenv(key, "configured")
	if got := env(key, "fallback"); got != "configured" {
		t.Fatalf("expected configured, got %q", got)
	}
}
