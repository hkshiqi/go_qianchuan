package main

import "testing"

func TestNormalizePublicPathPrefix(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		"/":                   "",
		"qianchuan-app":       "/qianchuan-app",
		"/qianchuan-app/":     "/qianchuan-app",
		"  /qianchuan-app/  ": "/qianchuan-app",
	}
	for input, expected := range tests {
		if actual := normalizePublicPathPrefix(input); actual != expected {
			t.Fatalf("normalizePublicPathPrefix(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestOperationReadOnlyPathsAddsPublicPrefix(t *testing.T) {
	input := operationReadOnlyPOSTPaths[0]
	paths := operationReadOnlyPaths("/qianchuan-app")
	if len(paths) != len(operationReadOnlyPOSTPaths) {
		t.Fatalf("unexpected path count: %d", len(paths))
	}
	if paths[0] != "/qianchuan-app"+input {
		t.Fatalf("prefixed path = %q", paths[0])
	}
}
