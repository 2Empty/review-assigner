.PHONY: run stop logs lint loadtest health help

run:
	docker-compose up --build -d

stop:
	docker-compose down

logs:
	docker-compose logs -f app

lint:
	golangci-lint run ./...

loadtest:
	k6 run loadtest/k6-script.js

health:
	curl -f http://localhost:8080/health || echo "Service unavailable"

help:
	@echo "Makefile команды:"
	@echo "  make run       - Запуск через Docker (docker-compose up --build -d)"
	@echo "  make stop      - Остановка контейнеров (docker-compose down)"
	@echo "  make logs      - Просмотр логов приложения (docker-compose logs -f app)"
	@echo "  make lint      - Проверка кода линтером (golangci-lint run ./...)"
	@echo "  make loadtest  - Нагрузочное тестирование (k6 run loadtest/k6-script.js)"
	@echo "  make health    - Проверка здоровья сервиса (curl http://localhost:8080/health)"

.DEFAULT_GOAL := help