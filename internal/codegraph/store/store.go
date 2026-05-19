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
	_, err := c.Exec("CHECKPOINT;")
	return err
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

// Exec executes a Cypher statement, optionally with named parameters.
// If params are supplied the statement is prepared and executed with
// parameter binding; otherwise it is executed directly.
func (c *Conn) Exec(cypher string, params ...map[string]any) (*lbug.QueryResult, error) {
	if len(params) == 0 {
		return c.inner.Query(cypher)
	}
	prep, err := c.inner.Prepare(cypher)
	if err != nil {
		return nil, err
	}
	defer func() { prep.Close() }()
	return c.inner.Execute(prep, params[0])
}

// Query is an alias for Exec — both return a QueryResult.
func (c *Conn) Query(cypher string, params ...map[string]any) (*lbug.QueryResult, error) {
	return c.Exec(cypher, params...)
}

// Begin starts an explicit transaction on this connection.
// Begin panics if the BEGIN statement fails (programmer error: e.g. a
// transaction is already open on this connection).
func (c *Conn) Begin() *Tx {
	if _, err := c.inner.Query("BEGIN TRANSACTION;"); err != nil {
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

// Exec executes a Cypher statement within the transaction.
func (t *Tx) Exec(cypher string, params ...map[string]any) (*lbug.QueryResult, error) {
	return t.conn.Exec(cypher, params...)
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	_, err := t.conn.inner.Query("COMMIT;")
	return err
}

// Rollback rolls back the transaction.
func (t *Tx) Rollback() error {
	_, err := t.conn.inner.Query("ROLLBACK;")
	return err
}
