-- schema.sql — recreated from scratch by `make db` into sqlforth.db
-- Backs the `users` table used in README/CLAUDE.md examples, e.g.:
--   from users col age 30 > where show

DROP TABLE IF EXISTS users;

CREATE TABLE users (
    name    TEXT    NOT NULL,
    age     INTEGER NOT NULL,
    country TEXT    NOT NULL
);

INSERT INTO users (name, age, country) VALUES
    ('Anna',    34, 'PL'),
    ('Bob',     22, 'US'),
    ('Celine',  41, 'PL'),
    ('Dmitri',  29, 'RU');
