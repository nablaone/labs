-- create user test with password 'test';
-- create database test owner test;
create user test with password 'test';
create database test owner test;

\c test test


\i test_drop.sql
\i test_create.sql


insert into client values(1, 'Foo Bar', 'foobar@example.com');
insert into client values(2, 'Quux', 'quux@example.com');
insert into client values(3, 'Foo Bar', 'foobar@example.com');

insert into product_category values(1, 'Main', null);

insert into product 
 select x as id,
        1 as product_category_id, 
        'SKU' || x as sku,
        'Product #' || x as name,
         x as price,
         'Description' as description,
         ''::bytea as image
         from generate_series(1,10) as x;

insert into purchase values(1, '0001', 1);

insert into purchase_item values(1,1,1,1);
insert into purchase_item values(2,1,2,2);
