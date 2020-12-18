package locker

import (
	"database/sql"
	"errors"
	"fmt"
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
	dbIsLocked bool
	sqlDBO     *sql.DB
}

func (om *Oracle) Close() error {
	return om.sqlDBO.Close()
}

func (om *Oracle) Lock() error {

	if om.dbIsLocked {
		return ErrIsLocked
	}

	tx, err := om.sqlDBO.Begin()
	if err != nil {
		return fmt.Errorf("Unable to begin a transaction: %w", err)
	}

	var cd interface{ Code() int }

	_, err = tx.Exec("CREATE TABLE schema_lock (id NUMBER PRIMARY KEY)")
	switch {
	case errors.As(err, &cd) && cd.Code() == ORA00955:
		break
	case err != nil:
		tx.Rollback()
		return fmt.Errorf("Failed to create table schema_lock: %w", err)
	default:
		break
	}

	_, err = tx.Exec("LOCK TABLE schema_lock IN EXCLUSIVE MODE NOWAIT")
	switch {
	case errors.As(err, &cd) && cd.Code() == ORA00054:
		tx.Rollback()
		return ErrIsLocked
	case err != nil:
		tx.Rollback()
		return fmt.Errorf("Failed to lock table schema_lock: %w", err)
	}

	_, err = tx.Exec("INSERT INTO schema_lock (id) VALUES (1)")
	switch {
	case errors.As(err, &cd) && cd.Code() == ORA00001:
		tx.Rollback()
		return ErrIsLocked
	case errors.As(err, &cd) && cd.Code() == ORA00054:
		tx.Rollback()
		return ErrIsLocked
	case err != nil:
		tx.Rollback()
		return fmt.Errorf("Failed to insert row into table schema_lock: %w", err)
	}

	om.dbIsLocked = true

	return tx.Commit()
}

func (om *Oracle) Unlock() error {

	tx, err := om.sqlDBO.Begin()
	if err != nil {
		return fmt.Errorf("Unable to begin a transaction: %w", err)
	}

	var cd interface{ Code() int }
	_, err = tx.Exec("LOCK TABLE schema_lock IN EXCLUSIVE MODE NOWAIT")
	switch {
	case errors.As(err, &cd) && cd.Code() == ORA00054:
		tx.Rollback()
		return ErrIsLocked
	case err != nil:
		tx.Rollback()
		return fmt.Errorf("Failed to lock table schema_lock: %w", err)
	}

	_, err = tx.Exec("DELETE FROM schema_lock WHERE id = 1")
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
