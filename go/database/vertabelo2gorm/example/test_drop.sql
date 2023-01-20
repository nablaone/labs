-- Created by Vertabelo (http://vertabelo.com)
-- Last modification date: 2018-10-02 18:10:57.004

-- foreign keys
ALTER TABLE purchase
    DROP CONSTRAINT client_purchase;

ALTER TABLE product
    DROP CONSTRAINT product_category_product;

ALTER TABLE product_category
    DROP CONSTRAINT product_category_product_category;

ALTER TABLE purchase_item
    DROP CONSTRAINT product_purchase_item;

ALTER TABLE purchase_item
    DROP CONSTRAINT purchase_purchase_item;

-- tables
DROP TABLE client;

DROP TABLE product;

DROP TABLE product_category;

DROP TABLE purchase;

DROP TABLE purchase_item;

-- End of file.

