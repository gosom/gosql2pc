package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gosom/gosql2pc"
	_ "github.com/jackc/pgx/stdlib"
)

func main() {
	// Connect to the databases
	userdb, orderdb, errdb := setup()
	if errdb != nil {
		fmt.Fprintln(os.Stderr, errdb)
		os.Exit(1)
	}

	var err error
	defer func() {
		userdb.Close()
		orderdb.Close()
		switch err {
		case nil:
			os.Exit(0)
		default:
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	var cmd string
	flag.StringVar(&cmd, "cmd", "basic", "command to run")
	flag.Parse()

	switch cmd {
	case "basic":
		err = basic(userdb, orderdb)
	case "concurrency1":
		err = concurrency1(userdb, orderdb)
	case "concurrency2":
		err = concurrency2(userdb, orderdb)
	default:
		err = fmt.Errorf("unknown command: %s", cmd)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func basic(userdb, orderdb *sql.DB) error {
	// we want to create an entry in the users table in userdb and an entry in the orders table in orderdb
	// in a single transaction. We can do this by using the gosql2pc package.
	userID := uuid.New().String()
	name := "John Doe"
	orderID := uuid.New().String()
	amount := 100

	if err := insertUserAndOrder(userdb, orderdb, userID, name, orderID, amount); err != nil {
		return err
	}

	// Now both of the entries should be in the database
	row := userdb.QueryRow("SELECT id, name FROM users WHERE id = $1", userID)
	u := struct {
		ID   string
		Name string
	}{}
	if err := row.Scan(&u.ID, &u.Name); err != nil {
		return err
	}
	fmt.Printf("User: %+v\n", u)
	order := struct {
		ID     string
		UserID string
		Amount int
	}{}
	row = orderdb.QueryRow("SELECT id, user_id, amount FROM orders WHERE id = $1", orderID)
	if err := row.Scan(&order.ID, &order.UserID, &order.Amount); err != nil {
		return err
	}
	fmt.Printf("Order: %+v\n", order)

	// Now let's try something that will fail
	// We will try to insert a new user but we are going to use an order ID that already exists

	userID = uuid.New().String()
	orderID = order.ID

	if err := insertUserAndOrder(userdb, orderdb, userID, name, orderID, amount); err == nil {
		return fmt.Errorf("expected an error but got none")
	}

	// Now we should see that the user should not have been inserted
	row = userdb.QueryRow("SELECT id, name FROM users WHERE id = $1", userID)
	if err := row.Scan(&u.ID, &u.Name); err != sql.ErrNoRows {
		return fmt.Errorf("expected no rows error but got %s", err)
	}
	fmt.Println("user not found")
	return nil
}

func concurrency1(userdb, orderdb *sql.DB) error {
	// Here we are going to try to insert the same data in parallel
	userID := uuid.New().String()
	name := "John Doe"
	orderID := uuid.New().String()
	amount := 100

	parallelism := 30
	wg := &sync.WaitGroup{}
	wg.Add(parallelism)
	var duplicateErrors int64
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			if err := insertUserAndOrder(userdb, orderdb, userID, name, orderID, amount); err != nil {
				atomic.AddInt64(&duplicateErrors, 1)
			}
		}()
	}
	wg.Wait()

	if duplicateErrors != int64(parallelism-1) {
		return fmt.Errorf("expected %d duplicate errors but got %d", parallelism-1, duplicateErrors)
	}

	// Now both of the entries should be in the database but only once
	row := userdb.QueryRow("SELECT COUNT(id) FROM users")
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	fmt.Printf("User count: %d\n", count)
	row = orderdb.QueryRow("SELECT COUNT(id) FROM orders")
	if err := row.Scan(&count); err != nil {
		return err
	}
	fmt.Printf("Order count: %d\n", count)

	return nil
}

func concurrency2(userdb, orderdb *sql.DB) error {
	// In this example we want to insert a user and an order.
	// the we start 2 goroutines that try to update the users.id at the same time
	// we expect that only one of the updates will succeed because the other will wait for the transaction to finish
	// but we will add on purpose some delay to the update and we will execute the second goroutine with a context that has a timeout
	userID := uuid.New().String()
	name := "John Doe"
	orderID := uuid.New().String()
	amount := 100

	if err := insertUserAndOrder(userdb, orderdb, userID, name, orderID, amount); err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	err1ch := make(chan error, 1)
	err2ch := make(chan error, 1)

	uuid1 := uuid.New().String()
	uuid2 := uuid.New().String()

	fmt.Println("current user id:", userID)
	fmt.Println("uuid1:", uuid1)
	fmt.Println("uuid2:", uuid2)

	go func() {
		defer wg.Done()
		userParticipant := gosql2pc.NewParticipant(userdb, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "update users set id = $1 where id = $2", uuid1, userID)
			time.Sleep(1 * time.Second) // delay the update
			return err
		})
		// Prepare the second participant in the transaction
		orderParticipant := gosql2pc.NewParticipant(orderdb, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "update orders set user_id = $1 where user_id = $2", uuid1, userID)
			time.Sleep(5 * time.Second) // artificial delay here
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
			err1ch <- err
			return
		}
		err1ch <- nil
	}()

	time.Sleep(100 * time.Millisecond) // give some time to the first goroutine to start the transaction

	go func() {
		defer wg.Done()
		userParticipant := gosql2pc.NewParticipant(userdb, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "update users set id = $1 where id = $2", uuid2, userID)
			return err
		})
		// Prepare the second participant in the transaction
		orderParticipant := gosql2pc.NewParticipant(orderdb, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "update orders set user_id = $1 where user_id = $2", uuid2, userID)
			return err
		})
		// setup the parameters for the transaction
		params := gosql2pc.Params{
			LogFn: func(format string, args ...any) {
				fmt.Println(format, args)
			},
			Participants: []gosql2pc.Participant{userParticipant, orderParticipant},
		}

		// setup a context to cancel the operation in 2 seconds
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// run the transaction
		if err := gosql2pc.Do(ctx, params); err != nil {
			err2ch <- err
			return
		}
		err2ch <- nil
	}()

	wg.Wait()

	err1 := <-err1ch
	err2 := <-err2ch
	if err1 != nil {
		return err1
	}
	if err2 == nil {
		return fmt.Errorf("expected error but got nil in the second goroutine")
	}
	fmt.Println("goroutine2 error", err2)
	// the value now of users.id and orders.user_id should be uuid1
	row := userdb.QueryRow("SELECT id FROM users WHERE id = $1", uuid1)
	var id string
	if err := row.Scan(&id); err != nil {
		return err
	}
	fmt.Printf("User id: %s\n", id)
	row = orderdb.QueryRow("SELECT user_id FROM orders WHERE user_id = $1", uuid1)
	if err := row.Scan(&id); err != nil {
		return err
	}
	fmt.Printf("Order user_id: %s\n", id)

	row = userdb.QueryRow("SELECT id FROM users WHERE id = $1", uuid2)
	if err := row.Scan(&id); err == nil {
		return fmt.Errorf("expected error but got nil")
	}
	row = orderdb.QueryRow("SELECT user_id FROM orders WHERE user_id = $1", uuid2)
	if err := row.Scan(&id); err == nil {
		return fmt.Errorf("expected error but got nil")
	}

	return nil
}

func setup() (*sql.DB, *sql.DB, error) {
	// Connect to the databases
	userdb, err := sql.Open("pgx", "postgres://user1:secret@localhost:5432/user1?sslmode=disable")
	if err != nil {
		return nil, nil, err
	}
	if err := userdb.Ping(); err != nil {
		return nil, nil, err
	}

	orderdb, err := sql.Open("pgx", "postgres://user2:secret@localhost:5433/user2?sslmode=disable")
	if err != nil {
		return nil, nil, err
	}
	if err := orderdb.Ping(); err != nil {
		return nil, nil, err
	}

	// ------------------------------------------------------------

	// Create some tables for testing

	// One table for users in userdb
	_, err = userdb.Exec("CREATE TABLE IF NOT EXISTS users (id uuid not null primary key, name text)")
	if err != nil {
		return nil, nil, err
	}
	// One table for orders in orderdb
	_, err = orderdb.Exec(`CREATE TABLE IF NOT EXISTS orders(
		id uuid not null primary key, 
		user_id uuid not null, 
		amount int not null)`)
	if err != nil {
		return nil, nil, err
	}

	// truncate the tables in case they existed from previous run
	_, err = userdb.Exec("TRUNCATE TABLE users")
	if err != nil {
		return nil, nil, err
	}
	_, err = orderdb.Exec("TRUNCATE TABLE orders")
	if err != nil {
		return nil, nil, err
	}
	return userdb, orderdb, nil
}

// insertUserAndOrder inserts a new user and order in a single distributed transaction using the two phase commit protocol
func insertUserAndOrder(userdb, orderdb *sql.DB, userID, name, orderID string, amount int) error {
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
		return err
	}
	return nil
}
