package metrics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
)

// InstrumentedDriverName is the registered name of the wrapped driver. Pass
// it to sql.Open() in place of "sqlite" to time every Exec/Query.
const InstrumentedDriverName = "sqlite-instrumented"

func init() {
	sql.Register(InstrumentedDriverName, &instDriver{base: &sqlite.Driver{}})
}

type instDriver struct{ base driver.Driver }

func (d *instDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &instConn{base: c}, nil
}

// instConn wraps a modernc.org/sqlite *conn. Only the context-aware paths
// time; the legacy non-context Exec/Query/Prepare/Begin forward without
// instrumentation since database/sql 1.8+ never calls them when the
// context variants are present.
type instConn struct {
	base driver.Conn
}

func (c *instConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.base.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &instStmt{base: stmt, query: query}, nil
}

func (c *instConn) Close() error { return c.base.Close() }

func (c *instConn) Begin() (driver.Tx, error) {
	if cb, ok := c.base.(driver.ConnBeginTx); ok {
		return cb.BeginTx(context.Background(), driver.TxOptions{})
	}
	return c.base.Begin()
}

func (c *instConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if cb, ok := c.base.(driver.ConnBeginTx); ok {
		return cb.BeginTx(ctx, opts)
	}
	return c.base.Begin()
}

func (c *instConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var stmt driver.Stmt
	var err error
	if cpc, ok := c.base.(driver.ConnPrepareContext); ok {
		stmt, err = cpc.PrepareContext(ctx, query)
	} else {
		stmt, err = c.base.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &instStmt{base: stmt, query: query}, nil
}

func (c *instConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := c.base.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	op, target := fingerprint(query)
	start := time.Now()
	res, err := ec.ExecContext(ctx, query, args)
	observe(op, target, start, err)
	return res, err
}

func (c *instConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.base.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	op, target := fingerprint(query)
	start := time.Now()
	rows, err := qc.QueryContext(ctx, query, args)
	observe(op, target, start, err)
	return rows, err
}

func (c *instConn) Ping(ctx context.Context) error {
	if p, ok := c.base.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

type instStmt struct {
	base  driver.Stmt
	query string
}

func (s *instStmt) Close() error                                    { return s.base.Close() }
func (s *instStmt) NumInput() int                                   { return s.base.NumInput() }
func (s *instStmt) Exec(args []driver.Value) (driver.Result, error) { return s.base.Exec(args) }
func (s *instStmt) Query(args []driver.Value) (driver.Rows, error)  { return s.base.Query(args) }

func (s *instStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	op, target := fingerprint(s.query)
	start := time.Now()
	ec, ok := s.base.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := ec.ExecContext(ctx, args)
	observe(op, target, start, err)
	return res, err
}

func (s *instStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	op, target := fingerprint(s.query)
	start := time.Now()
	qc, ok := s.base.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := qc.QueryContext(ctx, args)
	observe(op, target, start, err)
	return rows, err
}

func observe(op, target string, start time.Time, err error) {
	dbQueryDuration.WithLabelValues(op, target).Observe(time.Since(start).Seconds())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		dbQueryErrors.WithLabelValues(op, target).Inc()
	}
}

// fingerprint reduces a SQL statement to (verb, target) labels so the
// histogram stays low-cardinality. "SELECT * FROM tickets WHERE ..." becomes
// ("select", "tickets"); "INSERT INTO sessions (...) ..." becomes
// ("insert", "sessions"). Verbs we can't pin to a table (PRAGMA, BEGIN, ...)
// get an empty target.
func fingerprint(q string) (op, target string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "unknown", ""
	}
	// Strip leading -- comments and whitespace.
	for strings.HasPrefix(q, "--") {
		if i := strings.IndexByte(q, '\n'); i >= 0 {
			q = strings.TrimSpace(q[i+1:])
		} else {
			return "unknown", ""
		}
	}
	verb, rest := splitWord(q)
	verb = strings.ToLower(verb)
	switch verb {
	case "select":
		// Pull the first identifier after FROM.
		if i := indexFold(rest, " from "); i >= 0 {
			tail, _ := splitWord(strings.TrimSpace(rest[i+len(" from "):]))
			return verb, identTable(tail)
		}
		return verb, ""
	case "insert":
		// "INTO table (...)"
		if i := indexFold(rest, "into "); i >= 0 {
			tail, _ := splitWord(strings.TrimSpace(rest[i+len("into "):]))
			return verb, identTable(tail)
		}
		return verb, ""
	case "update":
		tbl, _ := splitWord(rest)
		return verb, identTable(tbl)
	case "delete":
		if i := indexFold(rest, "from "); i >= 0 {
			tail, _ := splitWord(strings.TrimSpace(rest[i+len("from "):]))
			return verb, identTable(tail)
		}
		return verb, ""
	case "with":
		// Common-table-expression queries: bucket together.
		return "select", "cte"
	case "begin", "commit", "rollback", "savepoint", "release":
		return "tx", verb
	case "pragma":
		tbl, _ := splitWord(rest)
		return "pragma", strings.ToLower(strings.TrimRight(tbl, "("))
	case "create", "drop", "alter":
		return "ddl", verb
	default:
		return verb, ""
	}
}

func splitWord(s string) (head, tail string) {
	s = strings.TrimLeft(s, " \t\r\n(")
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r', '\n', '(', ',', ';':
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// indexFold finds substr in s ignoring ASCII case. substr must be lowercase.
func indexFold(s, substr string) int {
	return strings.Index(strings.ToLower(s), substr)
}

// identTable strips quoting/aliases from an identifier so "tickets" and
// `"tickets"` collapse to the same label.
func identTable(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`+"`")
	if i := strings.IndexAny(s, " \t\r\n("); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
