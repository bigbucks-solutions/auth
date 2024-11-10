package models

import (
	"errors"

	"github.com/jackc/pgconn"
)

var (
	// ErrDuplicateKey Raise custom Unique key violation error
	ErrDuplicateKey = errors.New("duplicate key error")
)

// ParseError Parse GORM Error to mask custom type
func ParseError(err error) error {
	var pqerr *pgconn.PgError
	if errors.As(err, &pqerr) {
		if pqerr.Code == "23505" {
			return ErrDuplicateKey
		}
	}
	return err
}
