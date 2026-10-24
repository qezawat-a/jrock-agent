package config

import "testing"

func TestLoadMemoryPathDefaultAndOverride(t *testing.T) {
	t.Setenv("MEMORY_PATH", "")
	cfg := Load()
	if cfg.MemoryPath != "data/memory.md" {
		t.Fatalf("memory path default=%q", cfg.MemoryPath)
	}
	t.Setenv("MEMORY_PATH", "/app/data/memory.md")
	cfg = Load()
	if cfg.MemoryPath != "/app/data/memory.md" {
		t.Fatalf("memory path override=%q", cfg.MemoryPath)
	}
}
