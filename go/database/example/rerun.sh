#! /bin/bash

set -x
set -e
docker run -e POSTGRES_PASSWORD= -p 5432:5432 --name pgtest --rm  -d postgres

sleep 20
psql  -U postgres -h localhost < rerun.sql 

go install github.com/nablaone/vertabelo2sqlx/cmd/vertabelo2sqlx
vertabelo2sqlx test.xml main example.go
go build 
./example

docker stop pgtest