# Сенсоры HA Pandora CAS 2025.2.4

Инвентаризация определений sensor.py и binary_sensor.py из hass-pandora-cas. Зависимость API: pandora-cas==0.0.16. Это список объявленных сенсоров; фактические значения зависят от устройства. Экспортер не зависит от этой интеграции.

## Обычные сенсоры (36)

| ID без префикса устройства | Название | Источник модели | По умолчанию |
|---|---|---|---|
| mileage | Пробег системы | CurrentState.mileage | включён |
| can_mileage | Пробег по CAN | CurrentState.can_mileage | включён |
| can_mileage_to_empty | Пробег по CAN на топливе | CurrentState.can_mileage_to_empty | отключён |
| can_mileage_by_battery | CAN Mileage batt | CurrentState.can_mileage_by_battery | включён |
| engine_hours | Моточасы | CurrentState.engine_hours | включён |
| can_engine_hours | Моточасы по CAN | CurrentState.can_engine_hours | включён |
| fuel | Топливный бак | CurrentState.fuel | включён |
| fuel_consumption | Расход топлива | CurrentState.can_consumption | отключён |
| soc | State Of Charge | CurrentState.ev_state_of_charge | включён |
| interior_temperature | Температура в салоне | CurrentState.interior_temperature | включён |
| engine_temperature | Температура двигателя | CurrentState.engine_temperature | включён |
| exterior_temperature | Температура за бортом | CurrentState.exterior_temperature | включён |
| battery_temperature | Температура аккумулятора | CurrentState.battery_temperature | отключён |
| heater_temperature | Heater Temperature | CurrentState.heater_temperature | отключён |
| balance | Баланс SIM | CurrentState.balance | включён |
| balance_secondary | Баланс второй SIM-карты | CurrentState.balance_other | отключён |
| speed | Скорость | CurrentState.speed | включён |
| tachometer | Обороты | CurrentState.engine_rpm | включён |
| gsm_level | Сотовый сигнал | CurrentState.gsm_level | включён |
| battery_voltage | Напряжение аккумулятора | CurrentState.voltage | включён |
| internal_voltage | Internal voltage | CurrentState.internal_voltage | включён |
| heater_voltage | Heater voltage | CurrentState.heater_voltage | включён |
| left_front_tire_pressure | Давление передней левой шины | CurrentState.can_tpms_front_left | отключён |
| right_front_tire_pressure | Давление передней правой шины | CurrentState.can_tpms_front_right | отключён |
| left_back_tire_pressure | Давление задней левой шины | CurrentState.can_tpms_back_left | отключён |
| right_back_tire_pressure | Давление задней правой шины | CurrentState.can_tpms_back_right | отключён |
| reserve_tire_pressure | Давление запасной шины | CurrentState.can_tpms_reserve | отключён |
| track_distance | Дистанция трека | TrackingPoint.length | отключён |
| key_number | Номер ключа | CurrentState.key_number | отключён |
| tag_number | Номер метки | CurrentState.tag_number | отключён |
| last_online | Последняя связь | CurrentState.online_timestamp_utc | включён |
| last_state_update | Последнее обновление состояния | CurrentState.state_timestamp_utc | включён |
| last_settings_change | Последнее изменение настроек | CurrentState.settings_timestamp_utc | включён |
| last_command_execution | Последнее выполнение команды | CurrentState.command_timestamp_utc | включён |
| days_to_maintenance | Дней до технического обслуживания | CurrentState.can_days_to_maintenance | включён |
| remaining_engine_runtime | До остановки двигателя | CurrentState.engine_remains | отключён |

## Бинарные сенсоры (28)

| ID без префикса устройства | Название | Источник модели | По умолчанию |
|---|---|---|---|
| connection_state | Соединение | 'is_online' | включён |
| moving | В движении | CurrentState.is_moving | включён |
| driver_door | Дверь водителя | BitStatus.DOOR_DRIVER_OPEN | включён |
| passenger_door | Дверь пассажира | BitStatus.DOOR_PASSENGER_OPEN | включён |
| left_back_door | Левая задняя дверь | BitStatus.DOOR_BACK_LEFT_OPEN | включён |
| right_back_door | Правая задняя дверь | BitStatus.DOOR_BACK_RIGHT_OPEN | включён |
| driver_glass | Стеклоподъёмник водителя | CurrentState.can_glass_driver | отключён |
| passenger_glass | Стеклоподъёмник пассажира | CurrentState.can_glass_passenger | отключён |
| left_back_glass | Задний левый стеклоподъёмник | CurrentState.can_glass_back_left | отключён |
| right_back_glass | Задний правый стеклоподъёмник | CurrentState.can_glass_back_right | отключён |
| driver_safety_belt | Ремень безопасности водителя | CurrentState.can_belt_driver | отключён |
| passenger_safety_belt | Ремень безопасности пассажира | CurrentState.can_belt_passenger | отключён |
| left_back_safety_belt | Задний левый ремень безопасности | CurrentState.can_belt_back_left | отключён |
| right_back_safety_belt | Задний правый ремень безопасности | CurrentState.can_belt_back_right | отключён |
| center_back_safety_belt | Задний центральный ремень безопасности | CurrentState.can_belt_back_center | отключён |
| seat_taken | Кресло занято | CurrentState.can_seat_taken | отключён |
| trunk | Багажник | BitStatus.TRUNK_OPEN | включён |
| hood | Капот | BitStatus.HOOD_OPEN | включён |
| parking | Нейтраль | BitStatus.HANDBRAKE_ENGAGED | включён |
| brakes | Тормоз | BitStatus.BRAKES_ENGAGED | включён |
| ignition | Зажигание | BitStatus.IGNITION | включён |
| exterior_lights | Световые сигналы | BitStatus.EXTERIOR_LIGHTS_ACTIVE | включён |
| evacuation_mode | Режим эвакуации | BitStatus.EVACUATION_MODE_ACTIVE | включён |
| ev_charging_connected | Зарядка подключена | CurrentState.ev_charging_connected | отключён |
| can_low_liquid | Низкий уровень жидкости | CurrentState.can_low_liquid | отключён |
| engine_locked | Блокировка двигателя | BitStatus.ENGINE_LOCKED | включён |
| heater_errors | Ошибки подогревателя | CurrentState.heater_errors | отключён |
| obd_error_codes | Diagnostic Trouble Codes | CurrentState.obd_error_codes | включён |

## Моточасы и parking

- engine_hours → CurrentState.engine_hours → motohours, часы.
- can_engine_hours → CurrentState.can_engine_hours → motohours_CAN, часы.
- Для HTTP-ответов библиотека проверяет также вложенные can-поля с приоритетом над полями stats.
- parking → BitStatus.HANDBRAKE_ENGAGED → бит 27 bit_state_1.
- В экспортере: pandora_engine_hours, pandora_can_engine_hours и pandora_state{state="parking_brake"}.
