package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExample(t *testing.T) {
	cfg, err := Load("../../mock.yml")
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Functions) != 2 {
		t.Fatalf("functions = %d", len(cfg.Functions))
	}
}

func TestRejectInvalidConfig(t *testing.T) {
	tests := map[string]string{
		"duplicate":        "      - {name: TEXT, type: STRING, direction: import}\n      - {name: TEXT, type: STRING, direction: export}",
		"missing source":   "      - {name: TEXT, type: STRING, direction: export, from: MISSING}",
		"invalid value":    "      - {name: TEXT, type: INT, direction: export, value: 2147483648}",
		"truncation":       "      - {name: TEXT, type: CHAR, length: 1, direction: export, value: '😀'}",
		"unsupported type": "      - {name: TEXT, type: TABLE, direction: export}",
		"input output":     "      - {name: TEXT, type: STRING, direction: import, value: test}",
		"ambiguous output": "      - {name: TEXT, type: STRING, direction: changing, value: test, from: TEXT}",
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mock.yml")
			data := "port: 8088\nfunctions:\n  - name: TEST\n    parameters:\n" + params + "\n"
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}

			if _, err := Load(path); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestStrictYAML(t *testing.T) {
	for _, data := range []string{"port: 8088\nunknown: true", "port: 8088\n---\nport: 8089"} {
		path := filepath.Join(t.TempDir(), "mock.yml")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}

		if _, err := Load(path); err == nil {
			t.Fatal("expected YAML error")
		}
	}
}

func TestEchoCompatibility(t *testing.T) {
	cfg := Config{Port: 8088, Functions: []Function{{Name: "TEST", Parameters: []Parameter{
		{Name: "INPUT", Type: "CHAR", Length: 10, Direction: "import"},
		{Name: "OUTPUT", Type: "CHAR", Length: 5, Direction: "export", From: "INPUT"},
	}}}}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "same type and length") {
		t.Fatalf("unexpected error: %v", err)
	}
}
