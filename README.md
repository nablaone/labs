# AutoApp.AI

An adaptive UI that watches your SQL queries and autonomously builds itself — grids, filters, charts, forms — in real time via an agentic LLM loop.

## Run

```bash
# Prerequisites: Docker, Go 1.22+, Node 20+

# Start Postgres
docker-compose up -d postgres

# Seed schema + data
psql $DATABASE_URL -f scripts/seed.sql

# Copy and fill in env
cp .env.example .env   # set DATABASE_URL and ANTHROPIC_API_KEY

# Build and start
make start
```

Open http://localhost:5173, type a SQL query, and watch the UI build itself.
