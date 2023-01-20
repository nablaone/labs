#! /bin/bash

set -x
set -e

psql test -U test < rerun.sql 

go install github.com/nablaone/vertabelo2gorm/cmd/vertabelo2gorm
vertabelo2gorm test.xml main example.go
go build 
./example