-- ── Core CRM ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS customers (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT,
    company    TEXT,
    country    TEXT DEFAULT 'PL',
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS products (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    category    TEXT CHECK (category IN ('software','hardware','service','addon')),
    price       NUMERIC(10,2) NOT NULL,
    status      TEXT CHECK (status IN ('active','discontinued','draft')) DEFAULT 'active',
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id          SERIAL PRIMARY KEY,
    customer_id INT REFERENCES customers(id),
    status      TEXT CHECK (status IN ('pending','processing','shipped','cancelled')) DEFAULT 'pending',
    amount      NUMERIC(10,2),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    id         SERIAL PRIMARY KEY,
    order_id   INT REFERENCES orders(id) ON DELETE CASCADE,
    product_id INT REFERENCES products(id),
    quantity   INT NOT NULL DEFAULT 1,
    unit_price NUMERIC(10,2) NOT NULL
);

-- ── SaaS ──────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS user_accounts (
    id            SERIAL PRIMARY KEY,
    customer_id   INT REFERENCES customers(id),
    email         TEXT NOT NULL UNIQUE,
    role          TEXT CHECK (role IN ('owner','admin','member','viewer')) DEFAULT 'member',
    status        TEXT CHECK (status IN ('active','suspended','pending','invited')) DEFAULT 'pending',
    created_at    TIMESTAMPTZ DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id          SERIAL PRIMARY KEY,
    customer_id INT REFERENCES customers(id),
    product_id  INT REFERENCES products(id),
    plan        TEXT CHECK (plan IN ('trial','basic','pro','enterprise')) DEFAULT 'trial',
    status      TEXT CHECK (status IN ('active','cancelled','expired','past_due')) DEFAULT 'active',
    mrr         NUMERIC(10,2),
    started_at  TIMESTAMPTZ DEFAULT now(),
    expires_at  TIMESTAMPTZ
);

-- ── Audit ─────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_log (
    id              SERIAL PRIMARY KEY,
    user_account_id INT REFERENCES user_accounts(id),
    action          TEXT CHECK (action IN ('create','update','delete','login','logout','export')) NOT NULL,
    table_name      TEXT,
    record_id       INT,
    summary         TEXT,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- ── Seed data ─────────────────────────────────────────────────────────────────

INSERT INTO customers (name, email, company, country, created_at) VALUES
    ('Alice Kowalski',   'alice@example.com',   'Kowalski sp. z o.o.',  'PL', now() - interval '120 days'),
    ('Bob Nowak',        'bob@example.com',     'Nowak & Partners',     'PL', now() - interval '90 days'),
    ('Carol Wiśniewska', 'carol@example.com',   'Wiśniewska Tech',      'PL', now() - interval '60 days'),
    ('David Mazur',      'david@example.com',   'Mazur Digital',        'PL', now() - interval '45 days'),
    ('Eva Wójcik',       'eva@example.com',     'Wójcik Consulting',    'PL', now() - interval '30 days'),
    ('Frank Lis',        'frank@example.com',   'Lis Software House',   'DE', now() - interval '20 days'),
    ('Grace Jabłońska',  'grace@example.com',   'Jabłońska SaaS',       'PL', now() - interval '10 days')
ON CONFLICT DO NOTHING;

INSERT INTO products (name, description, category, price, status, created_at) VALUES
    ('AutoApp Core',        'Base adaptive UI platform',           'software', 299.00, 'active',       now() - interval '200 days'),
    ('AutoApp Pro',         'Pro tier with advanced analytics',    'software', 599.00, 'active',       now() - interval '200 days'),
    ('AutoApp Enterprise',  'Unlimited seats + SLA',               'software', 1999.00,'active',       now() - interval '200 days'),
    ('Onboarding Pack',     'Guided setup sessions (5h)',          'service',  499.00, 'active',       now() - interval '100 days'),
    ('Data Connector',      'PostgreSQL + BigQuery connector',     'addon',    149.00, 'active',       now() - interval '60 days'),
    ('Legacy Bridge',       'Import tool for legacy systems',      'addon',     99.00, 'discontinued', now() - interval '300 days'),
    ('Hardware Token',      '2FA hardware token (FIDO2)',          'hardware',  49.00, 'active',       now() - interval '80 days')
ON CONFLICT DO NOTHING;

INSERT INTO orders (customer_id, status, amount, created_at) VALUES
    (1, 'shipped',    448.00, now() - interval '110 days'),
    (1, 'pending',     49.00, now() - interval '2 days'),
    (2, 'processing', 748.00, now() - interval '5 days'),
    (2, 'cancelled',   99.00, now() - interval '8 days'),
    (3, 'shipped',   2498.00, now() - interval '55 days'),
    (3, 'shipped',    299.00, now() - interval '3 days'),
    (4, 'pending',    349.00, now() - interval '1 day'),
    (5, 'processing', 748.00, now() - interval '25 days'),
    (5, 'shipped',    599.00, now() - interval '12 days'),
    (6, 'shipped',   2498.00, now() - interval '18 days'),
    (7, 'pending',    299.00, now())
ON CONFLICT DO NOTHING;

INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES
    (1, 1, 1, 299.00),
    (1, 5, 1, 149.00),
    (2, 7, 1,  49.00),
    (3, 2, 1, 599.00),
    (3, 5, 1, 149.00),
    (5, 3, 1, 1999.00),
    (5, 4, 1, 499.00),
    (6, 1, 1, 299.00),
    (7, 1, 1, 299.00),
    (7, 7, 1,  49.00),
    (8, 2, 1, 599.00),
    (8, 5, 1, 149.00),
    (9, 2, 1, 599.00),
    (10,3, 1, 1999.00),
    (10,4, 1, 499.00),
    (11,1, 1, 299.00)
ON CONFLICT DO NOTHING;

INSERT INTO user_accounts (customer_id, email, role, status, created_at, last_login_at) VALUES
    (1, 'alice@example.com',      'owner',  'active',    now() - interval '120 days', now() - interval '1 day'),
    (1, 'alice.dev@example.com',  'admin',  'active',    now() - interval '100 days', now() - interval '3 days'),
    (2, 'bob@example.com',        'owner',  'active',    now() - interval '90 days',  now() - interval '5 days'),
    (3, 'carol@example.com',      'owner',  'active',    now() - interval '60 days',  now() - interval '2 days'),
    (3, 'carol.pm@example.com',   'member', 'active',    now() - interval '50 days',  now() - interval '10 days'),
    (4, 'david@example.com',      'owner',  'active',    now() - interval '45 days',  now() - interval '7 days'),
    (5, 'eva@example.com',        'owner',  'suspended', now() - interval '30 days',  now() - interval '20 days'),
    (6, 'frank@example.com',      'owner',  'active',    now() - interval '20 days',  now() - interval '1 day'),
    (7, 'grace@example.com',      'owner',  'pending',   now() - interval '10 days',  NULL)
ON CONFLICT DO NOTHING;

INSERT INTO subscriptions (customer_id, product_id, plan, status, mrr, started_at, expires_at) VALUES
    (1, 2, 'pro',        'active',    599.00, now() - interval '110 days', now() + interval '250 days'),
    (2, 2, 'pro',        'active',    599.00, now() - interval '85 days',  now() + interval '275 days'),
    (3, 3, 'enterprise', 'active',   1999.00, now() - interval '55 days',  now() + interval '305 days'),
    (4, 1, 'basic',      'active',    299.00, now() - interval '40 days',  now() + interval '320 days'),
    (5, 1, 'basic',      'cancelled', 299.00, now() - interval '25 days',  now() - interval '5 days'),
    (6, 3, 'enterprise', 'active',   1999.00, now() - interval '18 days',  now() + interval '342 days'),
    (7, 1, 'trial',      'active',      0.00, now() - interval '10 days',  now() + interval '20 days')
ON CONFLICT DO NOTHING;

INSERT INTO audit_log (user_account_id, action, table_name, record_id, summary, ip_address, created_at) VALUES
    (1, 'login',  NULL,          NULL, 'Successful login',                       '185.12.0.1',  now() - interval '1 day'),
    (1, 'update', 'orders',      2,    'Changed status: pending → processing',   '185.12.0.1',  now() - interval '23 hours'),
    (1, 'create', 'order_items', 2,    'Added product: AutoApp Core',            '185.12.0.1',  now() - interval '22 hours'),
    (3, 'login',  NULL,          NULL, 'Successful login',                       '91.54.20.3',  now() - interval '2 days'),
    (3, 'export', 'orders',      NULL, 'Exported orders report (CSV)',           '91.54.20.3',  now() - interval '2 days'),
    (6, 'login',  NULL,          NULL, 'Successful login',                       '78.90.1.44',  now() - interval '1 day'),
    (6, 'create', 'subscriptions',7,   'Created enterprise subscription',        '78.90.1.44',  now() - interval '18 days'),
    (4, 'update', 'user_accounts',7,   'Set status: active → suspended',         '185.12.0.1',  now() - interval '20 days'),
    (2, 'delete', 'orders',      4,    'Deleted cancelled order #4',             '185.12.0.2',  now() - interval '7 days'),
    (8, 'login',  NULL,          NULL, 'Successful login',                       '193.20.5.7',  now() - interval '1 day'),
    (1, 'logout', NULL,          NULL, 'Session ended',                          '185.12.0.1',  now() - interval '20 hours')
ON CONFLICT DO NOTHING;
