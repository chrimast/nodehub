package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

var ErrNoRows = sql.ErrNoRows

type DB struct {
	sql *sql.DB
}

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type CommandTag interface {
	RowsAffected() int64
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type Row interface {
	Scan(dest ...any) error
}

type commandTag int64

func (c commandTag) RowsAffected() int64 { return int64(c) }

type sqlTx struct {
	tx *sql.Tx
}

func (d *DB) Close() { _ = d.sql.Close() }

func (d *DB) Ping(ctx context.Context) error { return d.sql.PingContext(ctx) }

func (d *DB) Exec(ctx context.Context, query string, args ...any) (CommandTag, error) {
	rewritten := rewriteSQL(query)
	res, err := d.sql.ExecContext(ctx, rewritten, adaptArgs(args)...)
	if err != nil {
		err = fmt.Errorf("%w [sql=%s]", err, rewritten)
	}
	return tagOf(res, err)
}

func (d *DB) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	rewritten := rewriteSQL(query)
	rows, err := d.sql.QueryContext(ctx, rewritten, adaptArgs(args)...)
	if err != nil {
		return nil, constraint(fmt.Errorf("%w [sql=%s]", err, rewritten))
	}
	return sqlRows{rows: rows}, nil
}

func (d *DB) QueryRow(ctx context.Context, query string, args ...any) Row {
	rewritten := rewriteSQL(query)
	return sqlRow{row: d.sql.QueryRowContext(ctx, rewritten, adaptArgs(args)...), sql: rewritten}
}

func (d *DB) Begin(ctx context.Context) (Tx, error) {
	tx, err := d.sql.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx}, nil
}

func (t *sqlTx) Exec(ctx context.Context, query string, args ...any) (CommandTag, error) {
	rewritten := rewriteSQL(query)
	res, err := t.tx.ExecContext(ctx, rewritten, adaptArgs(args)...)
	if err != nil {
		err = fmt.Errorf("%w [sql=%s]", err, rewritten)
	}
	return tagOf(res, err)
}

func (t *sqlTx) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	rewritten := rewriteSQL(query)
	rows, err := t.tx.QueryContext(ctx, rewritten, adaptArgs(args)...)
	if err != nil {
		return nil, constraint(fmt.Errorf("%w [sql=%s]", err, rewritten))
	}
	return sqlRows{rows: rows}, nil
}

func (t *sqlTx) QueryRow(ctx context.Context, query string, args ...any) Row {
	rewritten := rewriteSQL(query)
	return sqlRow{row: t.tx.QueryRowContext(ctx, rewritten, adaptArgs(args)...), sql: rewritten}
}

func (t *sqlTx) Commit(ctx context.Context) error   { return t.tx.Commit() }
func (t *sqlTx) Rollback(ctx context.Context) error { return t.tx.Rollback() }

type sqlRows struct{ rows *sql.Rows }

func (r sqlRows) Next() bool              { return r.rows.Next() }
func (r sqlRows) Scan(dest ...any) error { return r.rows.Scan(wrapScan(dest)...) }
func (r sqlRows) Close()                  { _ = r.rows.Close() }
func (r sqlRows) Err() error              { return r.rows.Err() }

type sqlRow struct {
	row *sql.Row
	sql string
}

func (r sqlRow) Scan(dest ...any) error {
	err := r.row.Scan(wrapScan(dest)...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoRows
	}
	if err != nil && r.sql != "" {
		err = fmt.Errorf("%w [sql=%s]", err, r.sql)
	}
	return constraint(err)
}

func wrapScan(dest []any) []any {
	out := make([]any, len(dest))
	for i, d := range dest {
		switch d.(type) {
		case *time.Time, **time.Time:
			out[i] = &timeScanner{dest: d}
		case *[]string:
			out[i] = &stringSliceScanner{dest: d.(*[]string)}
		default:
			if isJSONDest(d) {
				out[i] = &jsonScanner{dest: d}
			} else {
				out[i] = d
			}
		}
	}
	return out
}

type stringSliceScanner struct{ dest *[]string }

func (s *stringSliceScanner) Scan(src any) error {
	if src == nil {
		*s.dest = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into []string", src)
	}
	if len(b) == 0 || string(b) == "null" {
		*s.dest = nil
		return nil
	}
	return json.Unmarshal(b, s.dest)
}

type timeScanner struct{ dest any }

func (t *timeScanner) Scan(src any) error {
	if src == nil {
		switch d := t.dest.(type) {
		case *time.Time:
			*d = time.Time{}
		case **time.Time:
			*d = nil
		}
		return nil
	}
	var parsed time.Time
	switch v := src.(type) {
	case time.Time:
		parsed = v
	case string:
		parsed = parseTime(v)
	case []byte:
		parsed = parseTime(string(v))
	default:
		return fmt.Errorf("cannot scan %T into time.Time", src)
	}
	switch d := t.dest.(type) {
	case *time.Time:
		*d = parsed
	case **time.Time:
		if parsed.IsZero() {
			*d = nil
		} else {
			p := parsed
			*d = &p
		}
	}
	return nil
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	} {
		if tm, err := time.Parse(layout, s); err == nil {
			return tm.UTC()
		}
	}
	if tm, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
		return tm
	}
	return time.Time{}
}

type jsonScanner struct{ dest any }

func (j *jsonScanner) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		b = []byte("null")
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into json", src)
	}
	if raw, ok := j.dest.(*json.RawMessage); ok {
		*raw = append(json.RawMessage(nil), b...)
		return nil
	}
	if reflect.TypeOf(j.dest).Elem().Kind() == reflect.Slice && reflect.TypeOf(j.dest).Elem().Elem().Kind() == reflect.Uint8 {
		reflect.ValueOf(j.dest).Elem().SetBytes(append([]byte(nil), b...))
		return nil
	}
	return json.Unmarshal(b, j.dest)
}

func isJSONDest(d any) bool {
	if _, ok := d.(*json.RawMessage); ok {
		return true
	}
	t := reflect.TypeOf(d)
	if t == nil || t.Kind() != reflect.Ptr {
		return false
	}
	name := t.Elem().String()
	return strings.Contains(name, "jsontext.Value") || strings.Contains(name, "json.RawMessage")
}

func tagOf(res sql.Result, err error) (CommandTag, error) {
	if err != nil {
		return commandTag(0), constraint(err)
	}
	n, _ := res.RowsAffected()
	return commandTag(n), nil
}

type ConstraintError struct {
	Code string
	err  error
}

func (e *ConstraintError) Error() string { return e.err.Error() }
func (e *ConstraintError) Unwrap() error { return e.err }

func IsConstraint(err error) bool {
	var c *ConstraintError
	return errors.As(err, &c)
}

func constraint(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unique constraint"):
		return &ConstraintError{Code: "23505", err: err}
	case strings.Contains(msg, "foreign key constraint"):
		return &ConstraintError{Code: "23503", err: err}
	case strings.Contains(msg, "check constraint"):
		return &ConstraintError{Code: "23514", err: err}
	case strings.Contains(msg, "constraint failed"):
		return &ConstraintError{Code: "23505", err: err}
	default:
		return err
	}
}

func sqlitePath(url string) (string, error) {
	switch {
	case strings.HasPrefix(url, "sqlite://"):
		return strings.TrimPrefix(url, "sqlite://"), nil
	case strings.HasPrefix(url, "file:"):
		return url, nil
	case url == "" || url == "sqlite":
		return "file:nodehub.db?_pragma=foreign_keys(1)", nil
	default:
		return "", fmt.Errorf("unsupported DATABASE_URL (sqlite://path required): %s", redactURL(url))
	}
}

func redactURL(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return url[:i+3] + "…"
	}
	return "…"
}
