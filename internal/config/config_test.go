package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DATABASE", "")

	cfg := Load()

	if cfg.Server.Port != "8080" {
		t.Fatalf("expected default server port 8080, got %q", cfg.Server.Port)
	}
	if cfg.Mongo.URI != "mongodb://localhost:27017" {
		t.Fatalf("expected default mongo uri, got %q", cfg.Mongo.URI)
	}
	if cfg.Mongo.Database != "nano_indexer" {
		t.Fatalf("expected default mongo database nano_indexer, got %q", cfg.Mongo.Database)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("MONGO_URI", "mongodb://example:27017")
	t.Setenv("MONGO_DATABASE", "custom_indexer")

	cfg := Load()

	if cfg.Server.Port != "9090" {
		t.Fatalf("expected server port 9090, got %q", cfg.Server.Port)
	}
	if cfg.Mongo.URI != "mongodb://example:27017" {
		t.Fatalf("expected mongo uri override, got %q", cfg.Mongo.URI)
	}
	if cfg.Mongo.Database != "custom_indexer" {
		t.Fatalf("expected mongo database override, got %q", cfg.Mongo.Database)
	}
}
