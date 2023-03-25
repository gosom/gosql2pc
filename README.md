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

	"github.com/gosom/gosql2pc"
	twophase "github.com/gosom/gosql2pc"
	_ "github.com/jackc/pgx/stdlib"
)

func main() {
	// Connect to the database
	db1, err := sql.Open("pgx", "postgres://user1:secret@localhost:5432/user1?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db1.Close()
	db2, err := sql.Open("pgx", "postgres://user2:secret@localhost:5433/user2?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db2.Close()

	// Create some tables for testing
	// One table for users in db1
	_, err = db1.Exec("CREATE TABLE IF NOT EXISTS users (id uuid not null primary key, name text)")
	if err != nil {
		panic(err)
	}
	// One table for orders in db2
	_, err = db2.Exec(`CREATE TABLE IF NOT EXISTS orders(
        id uuid not null primary key, 
        user_id uuid not null, 
        amount int not null)`)
	if err != nil {
		panic(err)
	}

	userID := "47f89b20-cb3a-11ed-8475-aba23b81b15d"
	name := "John Doe"
	orderID := "a444eeaa-cb3b-11ed-a5c2-3b46183795fa"
	amount := 10

	// Create the participants for the 2 phase commit
	p1 := twophase.NewParticipant(db1, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", userID, name)
		return err
	})

	p2 := twophase.NewParticipant(db2, func(ctx context.Context, tx *sql.Tx) error {
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

### Examples

In the `examples` directory there is an executable with some examples.

```
cd examples
docker-compose up -d
```

then you can run the 3 examples. Please read the source code and it's comments.

```
go run main.go -cmd=basic
go run main.go -cmd=concurrency1
go run main.go -cmd=concurrency2
```

## Caution

PostgreSQL has by default disabled the `prepared_transactions`. There is a good reason for that.
You may even locked out of the database of permanently lock a table.

You need a mechanism that monitors that monitors any orphaned prepared transactions and takes action.

Please read the [documentation](https://www.postgresql.org/docs/current/sql-prepare-transaction.html)
and [this blog post](https://www.cybertec-postgresql.com/en/prepared-transactions/)  
and [this blog post](https://www.highgo.ca/2020/01/28/understanding-prepared-transactions-and-handling-the-orphans/).

Don't be afraid but you should know with what you are dealing with.

In order to enable prepared transactions set  in `postgresql.conf`
`max_prepared_transactions` to something larger that zero. Better to set it to the number of `max_connections`

Alternatively, you can set it when you start the postgreSQL server by using the `-c` flag. 
(see the docker-compose.yaml in the `examples` folder).


The library gives you some level of consistency BUT when the process that coordinates the distributed transactions crashes you may leave orphan prepared transactions or having data inconsistency since only some of the
participants may have finished the commits.

Additionally, if one participant manages to commit and the others don't (because of a disk failure for example) then again you may have data incosistency. I recommend to have some monitoring for these cases.


Consider if you actually need to phase commit or you can use maybe Sagas. Both patterns are useful, but
I believe that Sagas are more generic since in order to use the 2 Phase Commit Protocol all the 
platforms need to implement the protocol. In any case distributed transactions are not trivial and you 
should be careful.


## Contributing
Contributions to go-sql-2pc are always welcome. If you find a bug or want to suggest a new feature, please open an issue or submit a pull request.


## LICENCE

go-sql-2pc is licensed under the MIT License. See the LICENSE file for more information.


