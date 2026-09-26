package exception

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func ParseGormError(err error, customMessage ...string) *ApplicationError {
	if err == nil {
		return nil
	}

	override := ""
	if len(customMessage) > 0 {
		override = customMessage[0]
	}

	if appErr := parseGormBuiltinError(err, override); appErr != nil {
		return appErr
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return parsePostgresError(pgErr, override)
	}

	return NewApplicationError(
		http.StatusInternalServerError,
		getMessage(override, "Database error occurred"),
	)
}

func parseGormBuiltinError(err error, override string) *ApplicationError {
	type mapping struct {
		match      error
		statusCode int
		defaultMsg string
	}

	maps := []mapping{
		{gorm.ErrRecordNotFound, http.StatusNotFound, "Record not found"},
		{gorm.ErrInvalidData, http.StatusBadRequest, "Invalid data"},
		{gorm.ErrUnsupportedDriver, http.StatusInternalServerError, "Unsupported database driver"},
	}

	for _, mapElement := range maps {
		if errors.Is(err, mapElement.match) {
			return &ApplicationError{
				Message:              getMessage(override, mapElement.defaultMsg),
				HttpStatusCode:       mapElement.statusCode,
				ConventionStatusCode: StatusDatabaseError,
			}
		}
	}

	return nil
}

func parsePostgresError(pgErr *pgconn.PgError, override string) *ApplicationError {
	type mapping struct {
		code       string
		statusCode int
		generator  func(*pgconn.PgError) string
	}

	maps := []mapping{
		{
			code:       "23505",
			statusCode: http.StatusConflict,
			generator: func(e *pgconn.PgError) string {
				return autoMessageFromConstraint(e.ConstraintName, "Duplicate entry")
			},
		},
		{
			code:       "23503",
			statusCode: http.StatusBadRequest,
			generator: func(e *pgconn.PgError) string {
				return autoFKMessage(e)
			},
		},
		{
			code:       "23514",
			statusCode: http.StatusBadRequest,
			generator: func(e *pgconn.PgError) string {
				return autoMessageFromConstraint(e.ConstraintName, "Check constraint failed")
			},
		},
		{
			code:       "23502",
			statusCode: http.StatusBadRequest,
			generator: func(e *pgconn.PgError) string {
				return fmt.Sprintf("%s cannot be null", formatColumnName(e.ColumnName))
			},
		},
		{"22001", http.StatusBadRequest, func(e *pgconn.PgError) string { return "Input data too long" }},
		{"22003", http.StatusBadRequest, func(e *pgconn.PgError) string { return "Numeric value out of range" }},
		{"22P02", http.StatusBadRequest, func(e *pgconn.PgError) string { return "Invalid input format" }},
		{"40P01", http.StatusConflict, func(e *pgconn.PgError) string { return "Database deadlock detected" }},
		{"42601", http.StatusBadRequest, func(e *pgconn.PgError) string { return "SQL syntax error" }},
		{"42501", http.StatusForbidden, func(e *pgconn.PgError) string { return "Permission denied" }},
	}

	for _, m := range maps {
		if pgErr.Code == m.code {
			return &ApplicationError{
				HttpStatusCode:       m.statusCode,
				ConventionStatusCode: StatusDatabaseError,
				Message:              getMessage(override, m.generator(pgErr)),
				Details:              nil,
			}
		}
	}

	return &ApplicationError{
		Message:              getMessage(override, "Database error occurred"),
		HttpStatusCode:       http.StatusInternalServerError,
		ConventionStatusCode: StatusDatabaseError,
	}
}

func getMessage(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func autoMessageFromConstraint(constraint, fallback string) string {
	if constraint == "" {
		return fallback
	}

	r := regexp.MustCompile(`(?:.*_)?(.+?)_(?:key|fkey|unique|idx)$`)
	match := r.FindStringSubmatch(constraint)

	if len(match) < 2 {
		return fallback
	}

	column := formatColumnName(match[1])
	return fmt.Sprintf("%s already exists", column)
}

func autoFKMessage(pgErr *pgconn.PgError) string {
	c := pgErr.ConstraintName
	if c == "" {
		return "Foreign key constraint failed"
	}

	r := regexp.MustCompile(`(.+?)_fkey$`)
	match := r.FindStringSubmatch(c)

	if len(match) < 2 {
		return "Foreign key constraint failed"
	}

	col := strings.TrimSuffix(match[1], "_id")
	return fmt.Sprintf("%s is not valid", formatColumnName(col))
}

func formatColumnName(col string) string {
	if col == "" {
		return ""
	}

	col = strings.TrimSuffix(col, "_id")

	parts := strings.Split(col, "_")
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}

	return strings.Join(parts, " ")
}
