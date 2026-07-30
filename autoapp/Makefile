.PHONY: start stop reset-data logs build build-backend build-frontend

BACKEND_PID_FILE := .backend.pid
FRONTEND_PID_FILE := .frontend.pid
BIN := bin/server

start: stop build
	@mkdir -p logs data
	@echo "Starting backend..."
	@./$(BIN) > logs/server.log 2>&1 & echo $$! > $(BACKEND_PID_FILE)
	@echo "Starting frontend..."
	@cd frontend && npm run dev > ../logs/frontend.log 2>&1 & echo $$! > ../$(FRONTEND_PID_FILE)
	@sleep 3
	@echo ""
	@tail -3 logs/server.log
	@echo ""
	@grep "Local:" logs/frontend.log | tail -1

build: build-backend build-frontend

build-backend:
	@echo "Building backend..."
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/server

build-frontend:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install --silent

stop:
	@if [ -f $(BACKEND_PID_FILE) ]; then \
		kill $$(cat $(BACKEND_PID_FILE)) 2>/dev/null && echo "Backend stopped" || true; \
		rm -f $(BACKEND_PID_FILE); \
	fi
	@if [ -f $(FRONTEND_PID_FILE) ]; then \
		kill $$(cat $(FRONTEND_PID_FILE)) 2>/dev/null && echo "Frontend stopped" || true; \
		rm -f $(FRONTEND_PID_FILE); \
	fi
	@lsof -ti :8080 | xargs kill -9 2>/dev/null || true
	@lsof -ti :5173 | xargs kill -9 2>/dev/null || true

reset-data:
	@echo "Resetting data..."
	@rm -f data/ui-state.json data/action-log.json data/discovery.json
	@echo "Done — restart to apply"

logs:
	@tail -f logs/server.log
