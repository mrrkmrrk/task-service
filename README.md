# Task Service

REST API для команд и задач.

## Стек

Go 1.26, MySQL 8.4, Redis 7.4, Docker Compose, JWT, OpenAPI 3.

## Запуск

```sh
cp .env.example .env
docker compose up -d --build --wait
```

API: http://localhost:8080/api/v1

Swagger: http://localhost:8080/swagger/

Проверка готовности: http://localhost:8080/readyz

Swagger UI и спецификация включены в бинарник, CDN не нужен.
Настройки находятся в `.env`. Пример содержит локальные пароли;
для другого окружения задайте свои значения, JWT_SECRET не короче 32 байт.
`JWT_TTL` задается в формате Go duration, например `24h`.
Приложение слушает `0.0.0.0`, Compose публикует порт только на localhost.
MySQL хранит данные в volume `mysql_data`. Порты MySQL и Redis не публикуются.

```sh
docker compose logs -f app
docker compose down
```

## Миграции

SQL находится в `migrations/` и применяется автоматически до запуска HTTP.
Выполненные миграции и их контрольные суммы сохраняются в `schema_migrations`.
Именованная блокировка MySQL защищает одновременный запуск экземпляров.
Выполненные файлы не редактируют: изменения схемы добавляют новой миграцией.
Начальная миграция допускает повтор после сбоя DDL.

## Эндпоинты

Все пути ниже начинаются с `/api/v1`. Кроме register/login нужен Bearer JWT.

| Метод | Путь | Доступ |
| --- | --- | --- |
| POST | `/register`, `/login` | Без JWT |
| POST, GET | `/teams` | Текущий пользователь |
| POST | `/teams/{id}/invite` | owner/admin |
| PATCH | `/teams/{id}/members/{userID}` | owner |
| POST, GET | `/tasks` | Участники команды |
| PUT | `/tasks/{id}` | По правилам редактирования |
| GET | `/tasks/{id}/history` | Участники команды |
| POST, GET | `/tasks/{id}/comments` | Участники команды |
| GET | `/teams/{team_id}/stats` | owner/admin |

Приглашение принимает `user_id` существующего пользователя и роль `admin/member`.
Повторное приглашение возвращает 409. Роль меняет только owner через PATCH.
Назначить или понизить owner через API нельзя. Удаление участников не предусмотрено.

Любой участник создает задачи, читает историю и оставляет комментарии.
Owner/admin редактируют любые задачи команды. Создатель задачи меняет все
редактируемые поля; исполнитель, который не является создателем, меняет только status.
Исполнитель обязательно состоит в той же команде; `assignee_id: null` снимает назначение.
Чужие задачи, их история и комментарии возвращают 404; чужая команда возвращает 403.

## Примеры запросов

Примеры для Bash. В PowerShell используйте `curl.exe` с подходящим экранированием JSON.

```sh
curl -s localhost:8080/api/v1/register -H 'Content-Type: application/json' -d '{"email":"owner@example.com","password":"password123","name":"Owner"}'
curl -s localhost:8080/api/v1/login -H 'Content-Type: application/json' -d '{"email":"owner@example.com","password":"password123"}'
TOKEN='<access_token из login>'
curl -s localhost:8080/api/v1/teams -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"name":"Backend"}'
```

В следующих примерах команда имеет id=1, второй зарегистрированный пользователь id=2.

```sh
curl -s localhost:8080/api/v1/teams/1/invite -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"user_id":2,"role":"member"}'
curl -s localhost:8080/api/v1/tasks -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"team_id":1,"title":"Add tests","description":"SQL report","status":"todo","assignee_id":2}'
curl -s 'localhost:8080/api/v1/tasks?team_id=1&status=todo&assignee_id=2&limit=20&offset=0' -H "Authorization: Bearer $TOKEN"
curl -s -X PUT localhost:8080/api/v1/tasks/1 -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"version":1,"status":"done"}'
curl -s localhost:8080/api/v1/tasks/1/history -H "Authorization: Bearer $TOKEN"
curl -s localhost:8080/api/v1/tasks/1/comments -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"content":"Ready"}'
curl -s localhost:8080/api/v1/teams/1/stats -H "Authorization: Bearer $TOKEN"
```

Ошибки: `{"error":{"code":"forbidden","message":"access denied"}}`.
Списки возвращаются JSON-массивами. GET tasks требует team_id, limit по умолчанию 20,
максимум 100, offset по умолчанию 0. Порядок: created_at DESC, id DESC.

## Обновления и история

PUT принимает `version` и хотя бы одно из полей title/description/status/assignee_id.
Неуказанные поля сохраняются. Начальная версия 1; успешное изменение увеличивает ее.
UPDATE содержит `WHERE id = ? AND version = ?`; устаревшая версия дает 409
с кодом `version_conflict`. Сначала перечитайте список и разрешите конфликт.
Задача, история old/new и версия кеша записываются одной транзакцией.
Пустое изменение не увеличивает version и не создает историю. Создание записывается в историю.
Статусы: todo/in_progress/done. Закрытие заполняет closed_at, повторное открытие очищает его.

## Кеш и отчет

GET tasks кешируется на 5 минут. Ключ включает команду, версию кеша, status,
assignee_id, limit и offset. Проверка членства выполняется перед чтением кеша.
`teams.cache_version` увеличивается внутри транзакции создания/обновления задачи.
Старые ключи становятся недоступны и истекают по TTL; KEYS/SCAN не используются.
Это также исключает возврат старого кеша после сбоя Redis или поздней записи старого списка.
При ошибке Redis список читается из MySQL. Readiness зависит от MySQL.

Отчет использует один CTE-запрос в `internal/repository/stats.sql`.
Статусы и комментарии считаются по всей команде. Топ-3: закрытые задачи за 30 дней UTC,
при равенстве первым идет меньший user_id. Задачи без исполнителя в топ не входят.
Среднее время: от created_at до closed_at по всем закрытым задачам; без задач возвращается 0.

## Тесты

```sh
go test ./...
go vet ./...
docker compose -p task-service-tests -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests
docker compose -p task-service-tests -f docker-compose.test.yml down -v
```

Compose запускает отдельные MySQL/Redis и `go test -race` + `go vet`.
Без `TEST_MYSQL_DSN` интеграционные тесты пропускаются. Для запуска вне Compose задайте
DSN отдельной тестовой БД и `TEST_REDIS_ADDR`. Тесты создают данные и временный SQL trigger.
Для trigger тестовый MySQL запускается с `log_bin_trust_function_creators=1`.
Покрыты отчет, граница команд, роли, JWT, кеш/TTL, фильтры, конкурентные записи
и откат задачи/истории/версии кеша при ошибке истории, отказ Redis и поздняя запись кеша.
