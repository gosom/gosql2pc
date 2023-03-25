module github.com/gosom/go-sql-2pc/example2pc

go 1.20

require (
	github.com/google/uuid v1.3.0 // indirect
	github.com/gosom/gosql2pc v0.0.0-20230325120000-000000000000
	github.com/jackc/pgx v3.6.2+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/text v0.8.0 // indirect
)

replace github.com/gosom/gosql2pc => ../
