package locker

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	godror "github.com/godror/godror"
)

func TestLocker(t *testing.T) {
	// assert := assert.New(t)
	req := require.New(t)

	connectionString := os.Getenv("DB_URL")
	req.NotEmpty(connectionString, "No DB_URL environment variable defined")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("lock-unlock", func(t *testing.T) {
		sqlDBO, err := sql.Open("godror", connectionString)
		req.NoError(err)
		defer sqlDBO.Close()

		ol := Oracle{
			sqlDBO: sqlDBO,
		}

		_ = ol.Unlock(ctx)

		err = ol.Lock(ctx)
		req.NoError(err)
		req.True(checkSchemaLock(ctx, t, sqlDBO))

		err = ol.Unlock(ctx)
		req.NoError(err)
		req.False(checkSchemaLock(ctx, t, sqlDBO))
	})

	if false {
		godror.Log = func(keyvals ...interface{}) error {
			t.Log(keyvals...)
			return nil
		}
		defer func() { godror.Log = nil }()
	}

	t.Run("Concurrency", func(t *testing.T) {
		var (
			req                = require.New(t)
			numberOfGoRoutines = 25
			err                error
			errorsChan         = make(chan error, numberOfGoRoutines)
			allErrors          = make([]error, 0, cap(errorsChan))
			locker             = make([]*Oracle, 0, cap(errorsChan))
			lockersChan        = make(chan *Oracle)
		)

		for i := 0; i < numberOfGoRoutines; i++ {
			sqlDBO, err := sql.Open("godror", connectionString)
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDBO.Close()

			go func(index int, sqlDBO *sql.DB) {
				om := &Oracle{sqlDBO: sqlDBO}

				if err != nil {
					t.Logf("Go-Routine %v: %+v", index, err)
					errorsChan <- err
					return
				}
				err = om.Lock(ctx)
				if err != nil {
					t.Logf("Go-Routine %v: %+v", index, err)
					errorsChan <- err
					return
				}
				lockersChan <- om
			}(i, sqlDBO)
		}

		for i := 0; i < numberOfGoRoutines; i++ {
			select {
			case err := <-errorsChan:
				allErrors = append(allErrors, err)
			case d := <-lockersChan:
				locker = append(locker, d)
			}
		}

		req.Equal(numberOfGoRoutines-1, len(allErrors), "Expect %v errors but received %v errors", numberOfGoRoutines-1, len(allErrors))
		req.Len(locker, 1)

		for _, err := range allErrors {
			req.True(errors.Is(err, ErrIsLocked), err)
		}

		err = locker[0].Unlock(ctx)
		req.NoError(err)
	})

}

func checkSchemaLock(ctx context.Context, t *testing.T, sqlDBO *sql.DB) bool {
	require := require.New(t)
	checkStmt := "SELECT id FROM schema_lock"

	row := sqlDBO.QueryRowContext(ctx, checkStmt)
	var id int
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(err)
	require.Equal(id, 1)
	return true
}
