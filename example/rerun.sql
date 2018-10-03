-- create user test with password 'test';
-- create database test owner test;

\i test_drop.sql
\i test_create.sql


insert into client values(1, 'Foo Bar', 'foobar@example.com');
insert into client values(2, 'Quux', 'quux@example.com');
insert into client values(3, 'Foo Bar', 'foobar@example.com');
