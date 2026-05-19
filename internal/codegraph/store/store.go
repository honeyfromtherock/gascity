// Package store is a thin idiomatic-Go wrapper over the LadybugDB binding.
// It enforces the single-writer/many-reader contract by exposing Open with
// an explicit Mode.
package store

import (
	"fmt"

	lbug "github.com/ladybugdb/go-ladybug"
)

// Mode controls whether the database is opened for reading and writing or
// reading only.
type Mode int

const (
	// ModeReadWrite opens the database for both reads and writes.
	ModeReadWrite Mode = iota
	// ModeReadOnly opens the database for reads only.
	ModeReadOnly
)

// DB is a handle to an open LadybugDB database. Obtain one via Open.
type DB struct {
	inner *lbug.Database
	path  string
	mode  Mode
}

// Open opens the LadybugDB database at path with the given mode.
// The caller is responsible for calling Close when done.
func Open(path string, mode Mode) (*DB, error) {
	cfg := lbug.DefaultSystemConfig()
	if mode == ModeReadOnly {
		cfg.ReadOnly = true
	}
	inner, err := lbug.OpenDatabase(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &DB{inner: inner, path: path, mode: mode}, nil
}

// Connect opens a new connection to the database.
// Connect panics if the database is healthy but a connection cannot be
// established — this represents a programmer error (e.g. DB already closed).
func (d *DB) Connect() *Conn {
	c, err := lbug.OpenConnection(d.inner)
	if err != nil {
		panic(fmt.Errorf("connect %s: %w", d.path, err))
	}
	return &Conn{inner: c}
}

// Checkpoint flushes all buffered writes to disk.
func (d *DB) Checkpoint() error {
	c := d.Connect()
	defer func() { _ = c.Close() }()
	return c.Exec("CHECKPOINT;")
}

// Close closes the database. It is safe to call Close multiple times.
func (d *DB) Close() error {
	// inner.Close is void in lbug; we return nil to match the io.Closer convention.
	d.inner.Close()
	return nil
}

// Path returns the filesystem path of the database.
func (d *DB) Path() string { return d.path }

// Conn is a single connection to the database. Connections are not
// goroutine-safe; use one connection per goroutine.
type Conn struct{ inner *lbug.Connection }

// Exec executes a Cypher statement, optionally with named parameters, and
// discards the result. Use Query when you need to iterate over returned rows.
//
// Exec always closes the underlying QueryResult before returning. This
// prevents a GC finalizer race where lbug_query_result_destroy fires after
// the database has already been explicitly closed, causing a SIGSEGV.
func (c *Conn) Exec(cypher string, params ...map[string]any) error {
	res, err := c.query(cypher, params...)
	if res != nil {
		res.Close()
	}
	return err
}

// Query executes a Cypher statement and returns the QueryResult for iteration.
// The caller is responsible for calling Close() on the returned result.
func (c *Conn) Query(cypher string, params ...map[string]any) (*lbug.QueryResult, error) {
	return c.query(cypher, params...)
}

// query is the shared implementation for Exec and Query.
func (c *Conn) query(cypher string, params ...map[string]any) (*lbug.QueryResult, error) {
	if len(params) == 0 {
		return c.inner.Query(cypher)
	}
	prep, err := c.inner.Prepare(cypher)
	if err != nil {
		return nil, err
	}
	res, err := c.inner.Execute(prep, params[0])
	prep.Close()
	return res, err
}

// Begin starts an explicit transaction on this connection.
// Begin panics if the BEGIN statement fails (programmer error: e.g. a
// transaction is already open on this connection).
func (c *Conn) Begin() *Tx {
	if err := c.Exec("BEGIN TRANSACTION;"); err != nil {
		panic(err)
	}
	return &Tx{conn: c}
}

// Inner returns the underlying *lbug.Connection. Prefer using Exec/Query; this
// escape hatch exists for packages (e.g. schema) that accept the raw type.
func (c *Conn) Inner() *lbug.Connection { return c.inner }

// Close closes the connection. It is safe to call Close multiple times.
func (c *Conn) Close() error {
	// inner.Close is void in lbug; we return nil to match the io.Closer convention.
	c.inner.Close()
	return nil
}

// Tx is an in-flight explicit transaction. Use Commit or Rollback to end it.
type Tx struct{ conn *Conn }

// Exec executes a Cypher statement within the transaction and discards the result.
func (t *Tx) Exec(cypher string, params ...map[string]any) error {
	return t.conn.Exec(cypher, params...)
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	return t.conn.Exec("COMMIT;")
}

// Rollback rolls back the transaction.
func (t *Tx) Rollback() error {
	return t.conn.Exec("ROLLBACK;")
}
