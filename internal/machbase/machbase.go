package machbase

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-client/machgo"
	"github.com/machbase/neo-water/internal/importer"
)

func OpenAppender(ctx context.Context, cfg importer.Config) (importer.Appender, func(), error) {
	host, portText, err := net.SplitHostPort(cfg.DB)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid -db %q: %w", cfg.DB, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid -db port %q: %w", portText, err)
	}

	database, err := machgo.NewDatabase(&machgo.Config{
		Host:         host,
		Port:         port,
		MaxOpenConn:  -1,
		MaxOpenQuery: -1,
	})
	if err != nil {
		return nil, nil, err
	}

	conn, err := database.Connect(ctx, api.WithPassword(cfg.User, cfg.Password))
	if err != nil {
		database.Close()
		return nil, nil, err
	}

	appender, err := conn.Appender(ctx, cfg.Table)
	if err != nil {
		conn.Close()
		database.Close()
		return nil, nil, err
	}

	closeFn := func() {
		conn.Close()
		database.Close()
	}
	return appender, closeFn, nil
}
