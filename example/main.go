package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/gosom/gosql2pc"
	_ "github.com/jackc/pgx/stdlib"
)

func main() {
	// Connect to the databases
	userdb, err := sql.Open("pgx", "postgres://user1:secret@localhost:5432/user1?sslmode=disable")
	if err != nil {
		panic(err)
	}
	if err := userdb.Ping(); err != nil {
		panic(err)
	}

	orderdb, err := sql.Open("pgx", "postgres://user2:secret@localhost:5433/user2?sslmode=disable")
	if err != nil {
		panic(err)
	}
	if err := orderdb.Ping(); err != nil {
		panic(err)
	}

	// ------------------------------------------------------------

	// Create some tables for testing

	// One table for users in userdb
	_, err = userdb.Exec("CREATE TABLE IF NOT EXISTS users (id uuid not null primary key, name text)")
	if err != nil {
		panic(err)
	}
	// One table for orders in orderdb
	_, err = orderdb.Exec(`CREATE TABLE IF NOT EXISTS orders(
		id uuid not null primary key, 
		user_id uuid not null, 
		amount int not null)`)
	if err != nil {
		panic(err)
	}

	// truncate the tables in case they existed from previous run
	_, err = userdb.Exec("TRUNCATE TABLE users")
	if err != nil {
		panic(err)
	}
	_, err = orderdb.Exec("TRUNCATE TABLE orders")
	if err != nil {
		panic(err)
	}

	// ------------------------------------------------------------

	// Now we want to create an entry in the users table in userdb and an entry in the orders table in orderdb
	// in a single transaction. We can do this by using the gosql2pc package.

	userID := uuid.New().String()
	name := "John Doe"
	orderID := uuid.New().String()
	amount := 100

	// Prepare the first participant in the transaction
	userParticipant := gosql2pc.NewParticipant(userdb, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, name)
		return err
	})

	// Prepare the second participant in the transaction
	orderParticipant := gosql2pc.NewParticipant(orderdb, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO orders (id, user_id, amount) VALUES ($1, $2, $3)", orderID, userID, amount)
		return err
	})

	// setup the parameters for the transaction
	params := gosql2pc.Params{
		LogFn: func(format string, args ...any) {
			fmt.Println(format, args)
		},
		Participants: []gosql2pc.Participant{userParticipant, orderParticipant},
	}

	// run the transaction
	if err := gosql2pc.Do(context.Background(), params); err != nil {
		panic(err)
	}

	// Now both of the entries should be in the database
	row := userdb.QueryRow("SELECT id, name FROM users WHERE id = $1", userID)
	u := struct {
		ID   string
		Name string
	}{}
	err = row.Scan(&u.ID, &u.Name)
	if err != nil {
		panic(err)
	}
	fmt.Printf("User: %+v\n", u)
	order := struct {
		ID     string
		UserID string
		Amount int
	}{}
	row = orderdb.QueryRow("SELECT id, user_id, amount FROM orders WHERE id = $1", orderID)
	err = row.Scan(&order.ID, &order.UserID, &order.Amount)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Order: %+v\n", order)

	// Now let's try something that will fail
	// We will try to insert a new user but we are going to use an order ID that already exists

	userID = uuid.New().String()
	orderID = order.ID

	userParticipant = gosql2pc.NewParticipant(userdb, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, name)
		return err
	})

	orderParticipant = gosql2pc.NewParticipant(orderdb, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO orders (id, user_id, amount) VALUES ($1, $2, $3)", orderID, userID, amount)
		return err
	})

	params.Participants = []gosql2pc.Participant{userParticipant, orderParticipant}

	// Now we can run the transaction
	err = gosql2pc.Do(context.Background(), params)
	if err == nil {
		panic("expected duplicate error")
	}

	// Now we should see that the user should not have been inserted
	row = userdb.QueryRow("SELECT id, name FROM users WHERE id = $1", userID)
	err = row.Scan(&u.ID, &u.Name)
	if err == nil {
		panic("we should have gotten an error")
	}
	fmt.Println("user not found")
}
