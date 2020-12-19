package locker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

const (
	// ORA00001 unique constraint violated
	ORA00001 int = 1

	// ORA00054 resource busy and acquire with NOWAIT specified or timeout expired
	ORA00054 int = 54

	// ORA00955 name is already being used by existing object
	ORA00955 int = 955
)

var ErrIsLocked = errors.New("Is locked")

type Oracle struct {
	mu         sync.Mutex
	dbIsLocked bool
	sqlDBO     *sql.DB
}

func (om *Oracle) Close() error {
	return om.sqlDBO.Close()
}

func (om *Oracle) Lock(ctx context.Context) error {
	om.mu.Lock()
	defer om.mu.Unlock()
	if om.dbIsLocked {
		return ErrIsLocked
	}

	tx, err := om.sqlDBO.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Unable to begin a transaction: %w", err)
	}
	defer tx.Rollback()

	var cd interface{ Code() int }

	if _, err = tx.ExecContext(ctx, "CREATE TABLE schema_lock (id NUMBER PRIMARY KEY)"); err != nil {
		if !(errors.As(err, &cd) && cd.Code() == ORA00955) {
			return fmt.Errorf("Failed to create table schema_lock: %w", err)
		}
	}

	if _, err = tx.ExecContext(ctx, "LOCK TABLE schema_lock IN EXCLUSIVE MODE NOWAIT"); err != nil {
		if errors.As(err, &cd) && cd.Code() == ORA00054 {
			return ErrIsLocked
		}
		return fmt.Errorf("Failed to lock table schema_lock: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_lock (id) VALUES (1)"); err != nil {
		if errors.As(err, &cd) {
			switch cd.Code() {
			case ORA00001, ORA00054:
				return ErrIsLocked
			}
		}
		return fmt.Errorf("Failed to insert row into table schema_lock: %w", err)
	}

	om.dbIsLocked = true

	return tx.Commit()
}

func (om *Oracle) Unlock(ctx context.Context) error {
	om.mu.Lock()
	defer om.mu.Unlock()
	tx, err := om.sqlDBO.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Unable to begin a transaction: %w", err)
	}
	defer tx.Rollback()

	var cd interface{ Code() int }
	_, err = tx.ExecContext(ctx, "LOCK TABLE schema_lock IN EXCLUSIVE MODE NOWAIT")
	switch {
	case errors.As(err, &cd) && cd.Code() == ORA00054:
		return ErrIsLocked
	case err != nil:
		return fmt.Errorf("Failed to unlock table schema_lock: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM schema_lock WHERE id = 1")
	if err != nil {
		return err
	}

	return tx.Commit()
}
