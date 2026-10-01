package config

import (
	"net/url"
	"testing"
)

func TestPostgresConnectionBuildEscapesCredentialsAndPreservesOptions(t *testing.T) {
	connection := PostgresConnection{
		Host: "db.example.test", Port: "5433", Database: "filament tracker",
		Username: "app user", Password: "p@ss:/word", Options: url.Values{"sslmode": {"verify-full"}},
	}
	got, err := connection.URL()
	if err != nil {
		t.Fatalf("build PostgreSQL URL: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse generated URL: %v", err)
	}
	if parsed.Hostname() != connection.Host || parsed.Port() != connection.Port || parsed.Path != "/filament tracker" {
		t.Fatalf("URL host/port/database = %q, %q, %q", parsed.Hostname(), parsed.Port(), parsed.Path)
	}
	username := parsed.User.Username()
	password, _ := parsed.User.Password()
	if username != connection.Username || password != connection.Password {
		t.Fatalf("URL credentials = %q / %q", username, password)
	}
	if parsed.Query().Get("sslmode") != "verify-full" {
		t.Fatalf("URL sslmode = %q", parsed.Query().Get("sslmode"))
	}
}

func TestPostgresConnectionFromURLParsesCredentialsAndOptions(t *testing.T) {
	connection, err := PostgresConnectionFromURL("postgres://user:secret@localhost:5434/filatrack?sslmode=require")
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	if connection.Host != "localhost" || connection.Port != "5434" || connection.Database != "filatrack" || connection.Username != "user" || connection.Password != "secret" || connection.Options.Get("sslmode") != "require" {
		t.Fatalf("parsed connection = %#v", connection)
	}
}
