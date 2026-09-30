# API веб-сервиса — справочник эндпоинтов

JSON API бэкенда «Расчёт количества теплоты в реакции горения» для SPA (ЛР3).
Роутинг — `registerAPI` в [server.go](server.go), обработчики — `internal/app/handler/*_api_handler.go`.

## Запуск и проверка

```bash
docker compose up -d        # PostgreSQL :5434, Adminer :8081, Minio :9000
go run ./cmd/migrate        # таблицы + стартовые данные
go run ./cmd/app            # API на http://localhost:3030/api
```

Коллекция Postman со всеми запросами: `рип\лаба3\postman\heat_fuels_lr3.postman_collection.json`.

## Общие правила

- **Base URL:** `http://localhost:3030`, все пути начинаются с `/api`.
- **Формат:** тело запроса и ответа — JSON (кроме `POST /api/fuels`: `multipart/form-data`).
- **Ошибка:** всегда `{"error": "текст"}` с кодом 400 / 401 / 403 / 404 / 409 / 413 / 500.
- **Пользователь:** авторизации нет до ЛР4. Все методы работают от имени `user_id = 1` (ivanov),
  его отдаёт функция-singleton `auth.GetCurrentUser()`. С клиента пользователь не передаётся.
- **Системные поля запрещены:** `fuel_id`, `fuel_status`, `creator_id`, `user_id`, `created_at`,
  `formed_at` вычисляет бэкенд. Любое поле, которого нет в описании запроса, → `400`.
- **Удалённые записи** (`fuel_status = "удален"`) не отдаются ни одним методом → `404`.
- **Статусы услуги:** `черновик` → `опубликован` → `удален`; `черновик` → `удален`.
  Вернуть в черновик и восстановить удалённую нельзя → `409`.
  Менять статус может только создатель карточки → `403`.
- **Черновик:** не более одного на пользователя.

## Эндпоинты

| # | Метод | URL | Тело | Успех |
|---|---|---|---|---|
| 1 | GET | `/api/fuels?min_heat={int}` | — | 200 `FuelListItem[]` |
| 2 | GET | `/api/fuels/feed`, `/api/fuels/feed/{fuel_id}[?next=true]` | — | 200 `FuelDetail` |
| 3 | GET | `/api/fuels/draft` | — | 200 `FuelDetail` |
| 4 | POST | `/api/fuels` | form-data | 201 `FuelDetail` |
| 5 | PUT | `/api/fuels/{fuel_id}/publish` | JSON | 200 `FuelDetail` |
| 6 | DELETE | `/api/fuels/{fuel_id}` | — | 200 `{fuel_id, message}` |
| 7 | POST | `/api/fuels/{fuel_id}/like` | JSON | 200 `LikeResponse` |
| 8 | POST | `/api/users/register` | JSON | 201 `UserShort` |
| 9 | POST | `/api/users/login` | JSON | 200 — заглушка до ЛР4 |
| 10 | POST | `/api/users/logout` | — | 200 — заглушка до ЛР4 |

### 1. `GET /api/fuels` — список услуг с фильтром

- Только опубликованные, по возрастанию `fuel_id`.
- `min_heat` — необязательный: нижняя граница `heat_of_combustion_kj`, кДж/м³.
- Ошибка: `min_heat` не целое или < 0 → `400`.

### 2. `GET /api/fuels/feed` — лента

- `/feed` — первая опубликованная карточка.
- `/feed/{fuel_id}` — эта карточка; `/feed/{fuel_id}?next=true` — следующая за ней (по кругу).
- Ошибка: нет такой опубликованной → `404`.

### 3. `GET /api/fuels/draft` — черновик текущего пользователя

- Ид не передаётся. Нет черновика → `404`.

### 4. `POST /api/fuels` — добавление услуги с изображением и видео

```text
Content-Type: multipart/form-data
fuel_name  text  обязательно, 1–128 символов
image      file  обязательно, jpeg/png/gif/webp/svg, ≤ 5 МБ
video      file  обязательно, mp4/webm, ≤ 30 МБ
```

- Создаёт карточку в статусе `черновик`, `creator_id` = текущий пользователь.
- Тип файла определяется по содержимому, не по расширению.
- Файлы сохраняются в Minio (бакет `heat-fuel-media`) под именем, которое генерирует бэкенд
  латиницей: `fuel-image-20260927-122214-7294bea3.jpg`. Url объекта пишется в `image_url` / `video_url`.
- Ответ содержит заголовок `Location: /api/fuels/draft`.
- Ошибки: нет поля или файла, неверный тип, лишнее поле → `400`; черновик уже есть → `409`;
  тело > 36 МБ → `413`.

### 5. `PUT /api/fuels/{fuel_id}/publish` — публикация

```json
{
  "combustion_note": "Этилен сгорает коптящим светящимся пламенем.",
  "heat_of_combustion_kj": 59000,
  "ignition_temp_c": 450
}
```

- Все поля необязательные: непереданное берётся из черновика. К публикации все три должны быть
  заполнены: описание не пустое, числа > 0.
- `черновик` → `опубликован`, `formed_at` = текущее время.
- Ошибки: незаполненное поле или лишнее поле → `400`; чужая карточка → `403`;
  удалена → `404`; уже опубликована → `409`.

### 6. `DELETE /api/fuels/{fuel_id}` — удаление

- Только soft delete: `fuel_status = "удален"`, строка и лайки остаются в БД.
- Ответ: `{"fuel_id": 7, "message": "Карточка переведена в статус «удален»"}`.
- Ошибки: чужая карточка → `403`; нет или уже удалена → `404`.

### 7. `POST /api/fuels/{fuel_id}/like` — лайк

```json
{ "like": 1 }
```

- `1` ставит лайк текущего пользователя, `0` отменяет. Повтор с тем же значением ничего не меняет:
  пара `(user_id, fuel_id)` уникальна, вставка идёт с `ON CONFLICT DO NOTHING`.
- Только опубликованные карточки.
- Ошибки: `like` нет или не 0/1 → `400`; не опубликована → `404`.

### 8. `POST /api/users/register` — регистрация

```json
{ "login": "sidorov", "full_name": "Сидоров Алексей", "password": "heat54321" }
```

- `login` — 3–64 символа: латиница, цифры, `_ . -`; `full_name` — 1–128; `password` — 6–72.
- В БД хранится bcrypt-хэш пароля, в ответе пароля нет.
- Ошибки: неверный формат или лишнее поле → `400`; логин занят → `409`.

### 9. `POST /api/users/login` — аутентификация (заглушка)

```json
{ "login": "sidorov", "password": "heat54321" }
```

- Проверяет логин и пароль, но сессию и токен не создаёт: пользователь по-прежнему из singleton.
- Ответ: `{"message": "...", "user": UserShort}`. Неверные данные → `401`.

### 10. `POST /api/users/logout` — деавторизация (заглушка)

- Тела нет. Ответ: `{"message": "..."}`.

## Схемы ответов

```text
FuelListItem  { fuel_id: uint, fuel_name: string, heat_of_combustion_kj: int, ignition_temp_c: int,
                image_url: string, likes_count: int, liked: bool }

FuelDetail    { fuel_id: uint, fuel_name: string, combustion_note: string,
                fuel_status: "черновик" | "опубликован",
                heat_of_combustion_kj: int, ignition_temp_c: int,
                image_url: string, video_url: string,
                created_at: RFC3339, formed_at: RFC3339 | null,
                creator: UserShort, likes_count: int, liked: bool }

LikeResponse  { fuel_id: uint, liked: bool, likes_count: int }

UserShort     { user_id: uint, login: string, full_name: string }
```

- `liked` — поставил ли лайк текущий пользователь; `likes_count` — `count(*)` по `fuel_likes`.
- Медиа по умолчанию (`/resources/media/...`) отдаются полным url этого сервера.
