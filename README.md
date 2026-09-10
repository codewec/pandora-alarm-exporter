# Pandora Exporter

Экспортер метрик Pandora для Prometheus и Grafana на Go.

[Анализ API, метрики и инструкция запуска](docs/exporter.md).

## Запуск через Docker Compose

Скопируйте шаблон, если .env ещё не создан:

    cp .env.example .env

Укажите в .env логин и пароль аккаунта p-on.ru:

    PANDORA_USERNAME=ваш_логин
    PANDORA_PASSWORD=ваш_пароль
    PANDORA_POLL_INTERVAL=5m

Запустите из корня проекта:

    docker compose -f examples/compose.yaml up -d --build

Первый опрос выполняется сразу, следующие — через 5 минут после завершения предыдущего. PANDORA_POLL_INTERVAL задаёт интервал (например 5m, 1m, 300s; минимум 10s). Флаг --poll-interval имеет приоритет.

Compose читает .env из корня проекта. Файл исключён из Git и Docker-образа.
Если пароль содержит символ $ или #, заключите значение в одинарные кавычки
по синтаксису Compose: PANDORA_PASSWORD='пароль$с#символами'.

## Запуск без Docker

Заполните .env и выполните из корня проекта:

    go run ./cmd/pandora-exporter/main.go

Приложение автоматически читает .env из текущей папки. Переменные,
уже заданные в окружении, имеют приоритет. Значения из файла не выполняются
как shell-команды и не используют подстановку переменных.

## Структура

- cmd/pandora-exporter — приложение.
- internal/pandora — API-клиент, метрики и тесты.
- Dockerfile — сборка контейнера.
- examples — Compose, Prometheus и dashboard Grafana.
- .github/workflows/exporter.yaml — проверки и публикация образа в GHCR.

Все команды выполняются из корня этого проекта:

    go build -o pandora-exporter ./cmd/pandora-exporter
    go test -race ./...
    docker build -t pandora-exporter .
