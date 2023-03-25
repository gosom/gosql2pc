# go-sql-2pc

`go-sql-2pc` is a Golang library for implementing 2 phase commit transactions in PostgreSQL, ensuring atomicity and consistency across distributed systems.

## Features

- Provides a simple and easy-to-use API for managing 2 phase commit transactions in PostgreSQL.
- Supports distributed transaction management across multiple databases.
- Ensures atomicity and consistency of transactions using the 2 phase commit protocol.
- Compatible with PostgreSQL 9.6 and higher.

## Installation

To install `go-sql-2pc`, run the following command:

```sh
go get github.com/gosom/go-sql-2pc
```

## Usage

```go
package main

import (
	"context"
	"database/sql"

	twophase "github.com/gosom/go-sql-2pc"
	_ "github.com/jackc/pgx/stdlib"
)

func main() {
    // Connect to the database
    db1, err := sql.Open("pgx", "postgres://user:password@host1/mydb1?sslmode=disable")
    if err != nil {
        panic(err)
    }
    defer db1.Close()
    db2, err := sql.Open("pgx", "postgres://user:password@host2/mydb2?sslmode=disable")
    if err != nil {
        panic(err)
    }
    defer db2.Close()

    // Create some tables for testing
    // One table for users in db1
    _, err = db1.Exec("CREATE TABLE IF NOT EXISTS users (id uuid not null primary key, name text)")
    if err != nil {
        return nil, nil, err
    }
    // One table for orders in db2
    _, err = db2.Exec(`CREATE TABLE IF NOT EXISTS orders(
        id uuid not null primary key, 
        user_id uuid not null, 
        amount int not null)`)
    if err != nil {
        return nil, nil, err
    }

    userID := "47f89b20-cb3a-11ed-8475-aba23b81b15d"
    name := "John Doe"
    amount := 10

    // Create the participants for the 2 phase commit
    p1 := twophase.NewParticipant(db1, func(ctx context.Context, tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, name)
        return err
    })

    p2 := twophase.NewParticipant(orderdb, func(ctx context.Context, tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, "INSERT INTO orders (id, user_id, amount) VALUES ($1, $2, $3)", orderID, userID, amount)
        return err
    })

    // setup the parameters for the transaction
    params := twophase.Params{
        Participants: []gosql2pc.Participant{p1, p2},
    }

    // run the transaction
    if err := twophase.Do(context.Background(), params); err != nil {
        panic(err)
    }
}
```

## Contributing
Contributions to go-sql-2pc are always welcome. If you find a bug or want to suggest a new feature, please open an issue or submit a pull request.


## LICENCE

go-sql-2pc is licensed under the MIT License. See the LICENSE file for more information.


