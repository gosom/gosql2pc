package gosql2pc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// Participant is a participant in a 2PC transaction
type Participant struct {
	do func(ctx context.Context, tx *sql.Tx) error

	db         *sql.DB
	txid       string
	prepared   bool
	committed  bool
	rollbacked bool
}

// NewParticipant creates a new participant
// The do function is called when the participant is prepared. It should contain all the
// database operations that should be performed in the transaction.
func NewParticipant(db *sql.DB, do func(ctx context.Context, tx *sql.Tx) error) Participant {
	return Participant{
		db: db,
		do: do,
	}
}

func (o *Participant) prepare(ctx context.Context) error {
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := o.do(ctx, tx); err != nil {
		return err
	}
	o.txid = getPreparedGid()
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PREPARE TRANSACTION '%s'", o.txid)); err != nil {
		return err
	}
	o.prepared = true
	return nil
}

func (o *Participant) rollback() error {
	if o.txid == "" || o.rollbacked || o.committed {
		return nil
	}
	if _, err := o.db.Exec(fmt.Sprintf("ROLLBACK PREPARED '%s'", o.txid)); err != nil {
		return err
	}
	o.rollbacked = true
	return nil
}

func (o *Participant) commit() error {
	if _, err := o.db.Exec(fmt.Sprintf("COMMIT PREPARED '%s'", o.txid)); err != nil {
		return err
	}
	o.committed = true
	return nil
}

func getPreparedGid() string {
	return defaultPrefix + uuid.New().String()
}
