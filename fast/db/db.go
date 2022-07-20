package db

import (
	"context"
	"errors"
	"fmt"

	pgx "github.com/jackc/pgx/v4"
)

// DBConnect connect to a Postgres compatible database.
func DBConnect(ctx context.Context, dbUser, dbPass, dbHost, dbName, dbParams string) (*pgx.Conn, error) {
	// https://github.com/jackc/pgx/blob/master/batch_test.go#L32

	dsn := fmt.Sprintf("postgresql://%s:%s@%s/%s?%s", dbUser, dbPass, dbHost, dbName, dbParams)
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, errors.New(fmt.Sprint("failed to connect database", err))
	}

	return conn, nil
}
