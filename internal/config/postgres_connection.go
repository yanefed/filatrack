package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// PostgresConnection contains the editable parts of a PostgreSQL connection URL.
type PostgresConnection struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	Options  url.Values
}

func PostgresConnectionFromURL(raw string) (PostgresConnection, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return PostgresConnection{}, fmt.Errorf("parsing PostgreSQL URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return PostgresConnection{}, fmt.Errorf("unsupported PostgreSQL URL scheme %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return PostgresConnection{}, fmt.Errorf("PostgreSQL host is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	username, password := "", ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	return PostgresConnection{
		Host: parsed.Hostname(), Port: port,
		Database: strings.TrimPrefix(parsed.Path, "/"),
		Username: username, Password: password, Options: parsed.Query(),
	}, nil
}

func (c PostgresConnection) URL() (string, error) {
	host := strings.TrimSpace(c.Host)
	database := strings.TrimSpace(c.Database)
	username := strings.TrimSpace(c.Username)
	if host == "" || database == "" || username == "" {
		return "", fmt.Errorf("PostgreSQL host, database, and username are required")
	}
	port, err := strconv.Atoi(strings.TrimSpace(c.Port))
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PostgreSQL port must be between 1 and 65535")
	}
	parsed := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/" + database,
		User:   url.UserPassword(username, c.Password),
	}
	if len(c.Options) > 0 {
		parsed.RawQuery = c.Options.Encode()
	}
	return parsed.String(), nil
}
