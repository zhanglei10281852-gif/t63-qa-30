package sqlite

import (
	"database/sql"
	"strings"

	"sanitation-operations/internal/apperror"
)

func notFound(err error) error {
	if err == sql.ErrNoRows {
		return apperror.NotFound(apperror.ErrNotFound)
	}
	return err
}

func databaseError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "constraint failed") {
		return apperror.Conflict(err)
	}
	return err
}
