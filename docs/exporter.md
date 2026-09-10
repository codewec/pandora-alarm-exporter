# Pandora → Prometheus → Grafana

Самостоятельный Go-экспортер получает данные напрямую из API Pandora. Реализация использует только стандартную библиотеку Go, отдаёт Prometheus text format 0.0.4 на /metrics и проверку работоспособности HTTP-сервера на /healthz.

## Анализ API

Описание основано на анализе клиентского протокола и примеров ответов, с дополнительной проверкой входа, получения списка устройств и телеметрии на живом аккаунте 10 сентября 2026 года. API неофициальный; состав и доступность полей зависят от устройства, подключения CAN и прошивки. Наличие поля в примере не гарантирует его наличие у конкретной машины.

| Запрос к https://pro.p-on.ru | Данные / поведение |
|---|---|
| POST /api/users/login | Form login, password, lang=ru; возвращает session_id и устанавливает cookies |
| GET /api/devices | Список машин: id, name, model, firmware, fuel_tank, features, permissions, SIM, сведения об автомобиле |
| GET /api/updates?ts=-1 | Полный запрос состояния |
| GET /api/updates?ts=предыдущий_ts-1 | Частичное обновление; stats и time объединяются с кэшем |
| POST /api/devices/command | Управление сигнализацией; экспортер этот endpoint не использует |

В updates: ts — курсор сервера; stats — телеметрия по ID; time — отметки online/onlined/command/setting; lenta — события с координатами и снимком телеметрии; ucr — предположительно ответы на команды. Точная семантика ucr не подтверждена.

Запросы выполняются последовательно, с cookie jar, браузерным User-Agent, Origin/Referer, X-Requested-With и Sec-Fetch-* заголовками. Без браузерных Fetch Metadata сервер может вернуть HTTP 400: Request must be done via browser or use access_token. При HTTP 401/403 или статусах Session is expired, Invalid session, sid-expired выполняются один повторный вход и один повтор запроса. Прочие ошибки учитываются без вывода тела ответа или секретов в лог.

## Метрики

Все метрики машины имеют label device_id. Название, модель и прошивка находятся только в pandora_device_info{device_id,name,model,firmware}=1. Моточасы сопоставлены с моделью CurrentState из pandora-cas 0.0.16, используемой HA-интеграцией версии 2025.2.4. Вложенные can.motohours / can.motohours_CAN имеют приоритет над одноимёнными полями stats. Отсутствующие значения не заменяются нулями.

Все перечисленные измерения — gauge; GPS/CAN-пробег также gauge, поскольку может корректироваться или сбрасываться.

| Поле API | Метрика (префикс pandora_) | Единицы |
|---|---|---|
| online / move | online / moving | 0/1 |
| voltage | battery_voltage_volts | В |
| engine_temp / cabin_temp / out_temp | engine_temperature_celsius / cabin_temperature_celsius / ambient_temperature_celsius | °C |
| fuel | fuel_ratio | доля 0–1 |
| fuel × fuel_tank / 100 | fuel_liters | расчётные литры; только при заданном положительном объёме бака |
| speed | speed_meters_per_second | м/с, API км/ч делятся на 3.6 |
| mileage / mileage_CAN | gps_mileage_meters / can_mileage_meters | м, API км умножаются на 1000 |
| engine_rpm | engine_rpm | об/мин |
| motohours / motohours_CAN (или engine_hours / can_engine_hours) | engine_hours / can_engine_hours | часы, без пересчёта; вложенные can имеют приоритет |
| gsm_level | gsm_level | исходная шкала API, не dBm |
| balance / balance1 | sim_balance{sim="0 или 1",currency} | валюта из cur |
| active_sim | active_sim | индекс |
| x / y | latitude_degrees / longitude_degrees | градусы, только с --collect-coordinates |
| dtime_rec | data_timestamp_seconds | Unix seconds, время записи stats по комментарию исходного кода |
| dtime | device_timestamp_seconds | исходное время устройства |
| time[id].online | last_online_timestamp_seconds | Unix seconds |
| bit_state_1 | state{state="…"} | флаг 0/1 |

Флаги state декодируются целочисленно, включая биты 60–61 без потери точности:

| Бит | Значение label state |
|---|---|
| 0–4 | armed, alarm, engine_running, ignition, autostart_active |
| 5–10 | handsfree_lock, handsfree_unlock, gsm_enabled, gps_enabled, tracking_enabled, immobilizer |
| 11–14 | extra_sensor_warning_disabled, extra_sensor_main_disabled, shock_sensor_warning_disabled, shock_sensor_main_disabled |
| 15–20 | autostart_scheduled, sms_enabled, calls_enabled, lights, siren_warning_disabled, siren_disabled |
| 21–26 | front_left_door_open, front_right_door_open, rear_left_door_open, rear_right_door_open, trunk_open, hood_open |
| 27–31 | parking_brake, brake, coolant_heater, active_security, heater_scheduled |
| 33–35 | evacuation_mode, service_mode, stay_home |
| 60–61 | tag_polling_disabled, disarm_without_tag_disabled |

Сенсор HA parking соответствует pandora_state{state="parking_brake"} (бит 27). На дашборде он подписан «Нейтраль (parking)» по обозначению пользователя в HA. В протоколе бит описан как ручной тормоз, поэтому это не отдельный универсальный датчик положения коробки передач.

armed=1 означает «под охраной»: значение флага используется без инверсии. Для *_disabled единица означает отключение соответствующей функции.

Другие доступные поля пока не экспортируются:

- features и permissions — возможности устройства и права аккаунта, не текущая телеметрия.
- phone, phone1, sims.phoneNumber, owner_id, photo и сведения о владельце — не нужны для мониторинга.
- sims содержит дополнительные сведения SIM; текущая реализация использует balance/balance1, не дублирует sims.balance.
- tanks, props — в образце пустые массивы, схема неизвестна.
- rot, evaq, metka, brelok, relay, smeter, tconsum, land, bunker, ex_status, engine_remains — присутствуют в примере, но единицы или точная семантика не подтверждены.
- lenta — события; без подтверждённой схемы кодов и механизма дедупликации нельзя достоверно считать тревоги. Экспортер отдаёт текущее состояние тревоги, а не счётчик событий.
- onlined, command, setting и ucr не используются.

## Кэш, ошибки и свежесть

Интервал настраивается в .env: PANDORA_POLL_INTERVAL=5m. Поддерживаются длительности с единицами, например 1m или 300s; минимум 10s. Приоритет: --poll-interval → переменная окружения / .env → 5m. Изменение .env требует перезапуска экспортера.

Опрос сразу при запуске, затем через 5 минут после завершения предыдущего. Полное обновление и перечитывание списка устройств — каждые 5 минут при следующем опросе. Удалённые устройства и отсутствующие поля удаляются при полном обновлении. Частичные обновления сохраняют предыдущие значения; явный null прекращает экспорт соответствующего измерения. Отсутствующие и нечисловые значения не заменяются нулём.

При ошибке сохраняется последний успешный снимок. Поэтому HTTP 200 и Prometheus up=1 не означают, что Pandora доступна:

- pandora_scrape_success — успешность последнего фонового опроса, при запуске 0.
- pandora_poll_errors_total — число неудачных опросов (counter).
- pandora_poll_duration_seconds — продолжительность последнего опроса.
- pandora_last_success_timestamp_seconds — время последнего успеха, до первого успеха 0.
- pandora_devices — количество обнаруженных устройств.
- pandora_data_age_seconds{device_id} — возраст dtime_rec относительно часов экспортера.
- pandora_data_stale{device_id} — возраст больше --max-data-age (по умолчанию 10m).

online — отдельное состояние связи машины. Сервер API может успешно отвечать, когда машина офлайн. Возраст данных вычисляется при каждом scrape, но доступен только при наличии dtime_rec. Это возраст снимка, а не гарантия свежести каждого поля. Часы сервера и устройства могут различаться; отрицательный возраст ограничивается нулём.

## Запуск

Требуется Go >= 1.25. Docker и CI используют Go 1.26. Все команды ниже выполняются из корня проекта.

Создайте .env командой cp .env.example .env (если файл ещё не создан) и заполните PANDORA_USERNAME и PANDORA_PASSWORD учётными данными p-on.ru. Файл исключён из Git и контекста Docker-сборки.

Рекомендуемый запуск — через Compose, который автоматически читает .env из корня проекта:

    docker compose -f examples/compose.yaml up -d --build

Бинарник автоматически читает .env из текущей рабочей папки. Уже заданные переменные окружения имеют приоритет, даже если они пустые. Поддерживаются KEY=value, одинарные/двойные кавычки и комментарии. Подстановка переменных и выполнение shell-команд не производятся. Значения со спецсимволами удобно заключать в одинарные кавычки.

    go build -o /tmp/pandora-exporter ./cmd/pandora-exporter
    /tmp/pandora-exporter

Вместо PANDORA_PASSWORD можно указать PANDORA_PASSWORD_FILE с путём к файлу секрета, доступному UID 65532 в контейнере. Одновременно задавать оба нельзя. Завершающий перевод строки файла удаляется.

Флаги:

| Флаг | По умолчанию |
|---|---|
| --web.listen-address | :9349 |
| --pandora.base-url | https://pro.p-on.ru |
| --poll-interval | 5m, минимум 10s; переопределяет PANDORA_POLL_INTERVAL |
| --request-timeout | 20s на каждый HTTP-запрос |
| --max-data-age | 10m |
| --collect-coordinates | true |

    docker build -t pandora-exporter .
    docker run --rm -p 127.0.0.1:9349:9349 \
      -e PANDORA_USERNAME -e PANDORA_PASSWORD pandora-exporter

Или через Compose с файлом .env:

    docker compose -f examples/compose.yaml up -d --build

Prometheus доступен на localhost:9090, метрики — localhost:9349/metrics. HTTP endpoint не имеет аутентификации; примеры публикуют порт только на loopback. Для Grafana, установленной на том же хосте, добавьте источник Prometheus http://localhost:9090; из контейнера Grafana в сети Compose используйте http://prometheus:9090.

## Запросы Grafana / PromQL

Готовый [dashboard](../examples/grafana-dashboard.json): импортируйте JSON через Dashboards → Import и выберите источник Prometheus. Пробег GPS/CAN и три температуры используют отдельные запросы и легенды. Добавлены панели GSM, открытия дверей/багажника/капота и состояний автомобиля. Панель баланса показывает валюту в легенде; она не предполагает, что все SIM используют рубли.

    pandora_battery_voltage_volts
    pandora_fuel_ratio * 100
    pandora_speed_meters_per_second * 3.6
    pandora_gps_mileage_meters / 1000
    pandora_state{state="armed"}
    pandora_state{state="alarm"}
    pandora_sim_balance
    pandora_data_age_seconds

Для названий машин:

    pandora_battery_voltage_volts
      * on(device_id) group_left(name) pandora_device_info

Условия для alert rules (добавьте for: 5m по требованиям мониторинга):

    up{job="pandora"} == 0
    pandora_scrape_success == 0
    time() - pandora_last_success_timestamp_seconds > 660
    pandora_data_stale == 1
    pandora_online == 0

Для графиков только со свежими данными используйте:

    pandora_battery_voltage_volts
      and on(device_id) (pandora_data_stale == 0)

Координаты включены по умолчанию (отключить можно флагом `--collect-coordinates=false`). Готовый dashboard содержит Geomap и карточки для бинарных состояний: в Geomap запросы `pandora_latitude_degrees` и `pandora_longitude_degrees` объединяются по `device_id`, затем поля переименовываются в `latitude` и `longitude`. Координаты являются значениями метрик, не labels. Если у машины нет валидной пары координат, точка не выводится; последняя корректная позиция сохраняется при частичном ответе без координат.

## GitHub Actions / GHCR

[Workflow](../.github/workflows/exporter.yaml) запускает gofmt, vet, race-тесты и сборку Go, затем собирает образ linux/amd64 и linux/arm64. PR проверяет сборку без публикации; push в master/main, тег v* или ручной запуск публикуют ghcr.io/<owner>/pandora-alarm-exporter. Имя автоматически приводится к нижнему регистру.

Теги: latest для default branch, имя ветки, sha-…, версия из semver-тега (v1.2.3 → 1.2.3). Для первого релиза отправьте изменения в свой GitHub-репозиторий и при необходимости тег v1.0.0. Используется встроенный GITHUB_TOKEN с packages:write; секреты Pandora для сборки не требуются. Видимость пакета GHCR настраивается в GitHub; для скачивания приватного пакета нужна авторизация.

Для запуска GitHub Actions содержимое этой папки должно находиться в корне отдельного репозитория, включая .github/workflows. Публикация произойдёт при запуске workflow в GitHub; локальное добавление файлов её не запускает.

Формат метрик: [официальная документация Prometheus](https://prometheus.io/docs/instrumenting/exposition_formats/).
Сборка контейнеров: [Docker GitHub Actions](https://docs.docker.com/build/ci/github-actions/).
