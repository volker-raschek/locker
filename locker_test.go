package locker

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/godror/godror"
)

func TestLocker(t *testing.T) {
	// assert := assert.New(t)
	req := require.New(t)

	connectionString := os.Getenv("DB_URL")
	req.NotEmpty(connectionString, "No DB_URL environment variable defined")

	sqlDBO, err := sql.Open("godror", connectionString)
	req.NoError(err)

	ol := Oracle{
		sqlDBO: sqlDBO,
	}

	err = ol.Lock()
	req.NoError(err)
	req.True(checkSchemaLock(t, sqlDBO))

	err = ol.Unlock()
	req.NoError(err)
	req.False(checkSchemaLock(t, sqlDBO))

	t.Run("Concurrency", func(t *testing.T) {
		var (
			req                = require.New(t)
			numberOfGoRoutines = 25
			err                error
			errorsChan         = make(chan error, numberOfGoRoutines)
			allErrors          = make([]error, 0)
			locker             = make([]*Oracle, 0)
			lockersChan        = make(chan *Oracle)
		)

		for i := 0; i < numberOfGoRoutines; i++ {
			go func(index int) {
				sqlDBO, err := sql.Open("godror", connectionString)
				if err != nil {
					sqlDBO.Close()
					errorsChan <- err
					return
				}

				om := &Oracle{
					sqlDBO: sqlDBO,
				}

				if err != nil {
					t.Logf("Go-Routine %v: %v", index, err)
					sqlDBO.Close()
					errorsChan <- err
					return
				}
				err = om.Lock()
				if err != nil {
					t.Logf("Go-Routine %v: %v", index, err)
					sqlDBO.Close()
					errorsChan <- err
					return
				}
				lockersChan <- om
			}(i)
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

		err = locker[0].Unlock()
		req.NoError(err)

		err = locker[0].Close()
		req.NoError(err)
	})

}

func checkSchemaLock(t *testing.T, sqlDBO *sql.DB) bool {
	require := require.New(t)
	checkStmt := "SELECT id FROM schema_lock"

	row := sqlDBO.QueryRow(checkStmt)
	var id int
	err := row.Scan(&id)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(err)
	require.Equal(id, 1)
	return true
}
