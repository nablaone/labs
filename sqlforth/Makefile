BINARY := sqlforth
DB      := sqlforth.db
SCHEMA  := schema.sql

.PHONY: all build run db sqlite vet clean

all: build

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

# (re)create the sqlite database from scratch from schema.sql
db:
	rm -f $(DB)
	sqlite3 $(DB) < $(SCHEMA)

# open an interactive sqlite3 shell against the database
sqlite:
	sqlite3 $(DB)

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(DB) people.csv
