package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	const fromFile = "JROCK_TEST_DOTENV_VALUE"
	const preexisting = "JROCK_TEST_DOTENV_ENV_WINS"
	oldFile, hadFile := os.LookupEnv(fromFile)
	oldExisting, hadExisting := os.LookupEnv(preexisting)
	t.Cleanup(func() {
		if hadFile {
			_ = os.Setenv(fromFile, oldFile)
		} else {
			_ = os.Unsetenv(fromFile)
		}
		if hadExisting {
			_ = os.Setenv(preexisting, oldExisting)
		} else {
			_ = os.Unsetenv(preexisting)
		}
	})
	_ = os.Unsetenv(fromFile)
	_ = os.Setenv(preexisting, "process-value")
	contents := "# comment\n\n" + fromFile + " = \"file value\"\n" + preexisting + "=file-value\nINVALID_LINE\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	loadDotEnv(path)
	if got := os.Getenv(fromFile); got != "file value" {
		t.Fatalf("file value = %q, want quoted value without quotes", got)
	}
	if got := os.Getenv(preexisting); got != "process-value" {
		t.Fatalf("existing env = %q, want process-value", got)
	}
}

func TestGetUsesFallbackOnlyWhenUnsetOrEmpty(t *testing.T) {
	t.Setenv("JROCK_TEST_CONFIG_GET", "configured")
	if got := get("JROCK_TEST_CONFIG_GET", "fallback"); got != "configured" {
		t.Fatalf("get() = %q, want configured", got)
	}
	t.Setenv("JROCK_TEST_CONFIG_GET", "")
	if got := get("JROCK_TEST_CONFIG_GET", "fallback"); got != "fallback" {
		t.Fatalf("get() = %q, want fallback", got)
	}
}
