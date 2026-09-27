# fuel-backend — Лабораторная работа №3

Заявочная система по курсу «Разработка Интернет Приложений» (РИП).

**Тема:** Расчёт количества теплоты в реакции горения.

| Сущность курса | Сущность предметной области |
|---|---|
| `услуга` | вид топлива (метан, пропан-бутан, ацетилен, водород) |
| `заявка` | расчёт количества теплоты в кДж, выделившейся при полном сгорании заданного объёма при н.у. |

---

## 1. Стек

| Слой | Технология |
|---|---|
| Язык | Go 1.26 |
| Веб-фреймворк | Gin (`github.com/gin-gonic/gin`) |
| Шаблонизатор | `html/template` (штатный для Gin) |
| База данных | PostgreSQL 17, развёрнут в Docker |
| ORM | GORM (`gorm.io/gorm` + `gorm.io/driver/postgres`) |
| Панель администратора БД | Adminer, развёрнут в Docker |
| Логирование | logrus (`github.com/sirupsen/logrus`) |
| Хранилище файлов | Minio (S3), развёрнут в Docker; клиент `github.com/minio/minio-go/v7` |
| Веб-сервис | JSON API под `/api` для будущего SPA (ЛР3), тестируется в Postman |
| JavaScript | **не используется** — по заданию |

---

## 2. Запуск

### 2.1. Поднять инфраструктуру

```bash
docker compose up -d
```

Поднимаются четыре сервиса:

| Сервис | Адрес | Назначение |
|---|---|---|
| `heat-postgres` | `localhost:5434` | база данных `heat_fuels` |
| `heat-adminer` | <http://localhost:8081> | панель администратора БД |
| `heat-minio` | <http://localhost:9000>, консоль <http://localhost:9001> | изображения и видео карточек |
| `heat-minio-init` | — | разовое создание бакета `heat-fuel-media` |

Вход в Adminer: система **PostgreSQL**, сервер `heat-postgres`,
пользователь `heat`, пароль `heat`, база `heat_fuels`.

> Порты 5432 и 5433 на машине заняты другими установками PostgreSQL,
> поэтому контейнер курса отдаёт базу наружу через **5434**.

### 2.2. Создать таблицы и наполнить их данными

```bash
go run ./cmd/migrate
```

Команда выполняет `AutoMigrate` трёх моделей, создаёт индекс «не более одного
черновика на пользователя» и наполняет таблицы стартовыми данными.
Повторный запуск данные не дублирует.

### 2.3. Запустить приложение

```bash
go run ./cmd/app
```

Приложение поднимается на <http://localhost:3030> и редиректит `/` на плитку.
Веб-сервис — на том же порту под префиксом `/api` (раздел 4.1).

Переменные окружения (все необязательные):

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `APP_PORT` | `3030` | порт приложения |
| `DB_HOST` | `localhost` | хост PostgreSQL |
| `DB_PORT` | `5434` | порт PostgreSQL |
| `DB_USER` / `DB_PASSWORD` | `heat` / `heat` | учётные данные |
| `DB_NAME` | `heat_fuels` | имя базы |
| `MINIO_ENDPOINT` | `localhost:9000` | адрес S3 API Minio для загрузки файлов |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | `minioadmin` / `minioadmin` | учётные данные Minio |
| `MINIO_PUBLIC_ENDPOINT` | `http://localhost:9000` | адрес Minio для сборки url медиа |
| `MINIO_BUCKET` | `heat-fuel-media` | имя бакета |

---

## 3. Структура проекта

Раскладка по `golang-standards/project-layout`, как в методических указаниях курса.

```
cmd/app/main.go                       точка входа приложения
cmd/migrate/main.go                   миграции таблиц и наполнение данными
internal/api/server.go                настройка роутера, шаблонов, статики
internal/app/dsn/dsn.go               строка подключения к PostgreSQL из окружения
internal/app/auth/current_user.go     функция-singleton текущего пользователя (до ЛР4 — константа)
internal/app/models/fuel.go           модели User, Fuel, FuelLike, переходы статусов + HeatCardView
internal/app/serializers/             JSON запросов и ответов веб-сервиса
internal/app/repository/              слой доступа к данным (GORM и «сырой» SQL для SSR-удаления)
internal/app/handler/fuel_handler.go  SSR-страницы (ЛР2)
internal/app/handler/*_api_handler.go веб-сервис: домены FuelDomain и UserDomain (ЛР3)
internal/app/storage/media.go         подстановка медиа по умолчанию, сборка url Minio
internal/app/storage/minio.go         загрузка файлов в Minio, генерация латинских имён
templates/                            шаблоны трёх страниц + общие блоки
resources/css/style.css               стили приложения, вынесены в отдельный файл
resources/media/                      изображение и видео по умолчанию
docker-compose.yml                    PostgreSQL, Adminer, Minio
```

Зависимости однонаправленные: **обработчики обращаются к репозиторию,
репозиторий к обработчикам — никогда**.

---

## 4. HTTP-методы

### 4.1. Веб-сервис `/api` (ЛР3)

Десять методов в двух доменах, все обращения к БД — через ORM, ответы — JSON.
Роутинг — `registerAPI` в [internal/api/server.go](internal/api/server.go),
интерфейсы доменов — `FuelDomain` и `UserDomain` в
[fuel_api_handler.go](internal/app/handler/fuel_api_handler.go) и
[user_api_handler.go](internal/app/handler/user_api_handler.go).

| # | Метод и URL | Обработчик | Что делает | Коды |
|---|---|---|---|---|
| 1 | `GET /api/fuels?min_heat=50000` | `GetFuels` | список опубликованных с фильтром по теплоте сгорания | 200, 400 |
| 2 | `GET /api/fuels/feed`<br>`GET /api/fuels/feed/:fuel_id`<br>`GET /api/fuels/feed/:fuel_id?next=true` | `GetFuelFeed` | лента: первая / эта / следующая опубликованная карточка | 200, 404 |
| 3 | `GET /api/fuels/draft` | `GetFuelDraft` | черновик текущего пользователя, ид не указывается | 200, 404 |
| 4 | `POST /api/fuels` (form-data: `fuel_name`, `image`, `video`) | `CreateFuel` | новый черновик, файлы — в Minio | 201, 400, 409, 413 |
| 5 | `PUT /api/fuels/:fuel_id/publish` (JSON: `combustion_note`, `heat_of_combustion_kj`, `ignition_temp_c`) | `PublishFuel` | черновик → опубликован, ставит `formed_at` | 200, 400, 403, 404, 409 |
| 6 | `DELETE /api/fuels/:fuel_id` | `DeleteFuel` | soft delete: статус → удален | 200, 403, 404 |
| 7 | `POST /api/fuels/:fuel_id/like` (JSON: `{"like": 1}` / `{"like": 0}`) | `LikeFuel` | поставить / отменить лайк текущего пользователя | 200, 400, 404 |
| 8 | `POST /api/users/register` (JSON: `login`, `full_name`, `password`) | `Register` | регистрация, пароль — bcrypt-хэш | 201, 400, 409 |
| 9 | `POST /api/users/login` | `Login` | **заглушка до ЛР4**: проверяет пароль, сессию не создаёт | 200, 401 |
| 10 | `POST /api/users/logout` | `Logout` | **заглушка до ЛР4** | 200 |

Ошибки — всегда `{"error": "текст"}`.

**Бизнес-правила:**

* **Пользователь-создатель зафиксирован** функцией-singleton `auth.GetCurrentUser()`
  ([internal/app/auth/current_user.go](internal/app/auth/current_user.go)): объект
  создаётся один раз через `sync.Once`, `user_id` задан константой `creatorUserID = 1`.
  Все методы услуги берут пользователя только оттуда.
* **Переходы статусов** — `FuelStatus.CanChangeTo`: у создателя два метода —
  `черновик → опубликован` (PUT publish) и `черновик/опубликован → удален` (DELETE).
  Вернуть в черновик и восстановить удалённую нельзя (409). Публиковать и удалять
  может только создатель карточки (403).
* **Системные поля с клиента не принимаются.** В сериализаторах запросов их нет, JSON
  разбирается с `DisallowUnknownFields`, поля формы проверяются по списку: попытка
  передать `fuel_status`, `creator_id`, `formed_at`, `user_id`… даёт 400.
* **Удалённые записи на клиент не передаются:** список и лента — только
  опубликованные, остальные методы ищут карточку среди неудалённых (`404`).
* **Не более одного черновика** — проверка в обработчике (409) и частичный уникальный индекс в БД.
* **Файлы:** тип определяется по содержимому (первые 512 байт), изображение — jpeg/png/gif/webp/svg
  до 5 МБ, видео — mp4/webm до 30 МБ. Имя генерирует бэкенд латиницей:
  `fuel-image-20260926-162305-8cb80a19.jpg` (исходное, возможно кириллическое, имя
  не используется). Файл кладётся в бакет `heat-fuel-media`, url объекта — в поля
  `image_url` / `video_url`. Если запись в БД не удалась, загруженные файлы удаляются.

**Сериализаторы** ([internal/app/serializers/](internal/app/serializers/)): модели GORM
в JSON напрямую не отдаются. Ответы — `FuelListItem` (плитка), `FuelDetail` (лента,
черновик, результат создания/публикации, с вложенным `creator`), `LikeResponse`,
`UserShort` (без пароля). Запросы — `PublishFuelRequest`, `LikeRequest`,
`RegisterRequest`, `LoginRequest`.

Коллекция Postman и файлы для добавления услуги — в папке материалов ЛР3
(`рип\лаба3\postman\`).

### 4.2. SSR-страницы (ЛР2)

| # | Метод и URL | Контроллер | Доступ к данным |
|---|---|---|---|
| 1 | `GET /fuel_feed`<br>`GET /fuel_feed/:fuel_id`<br>`GET /fuel_feed/:fuel_id?next=true` | `GetFuelFeed` | ORM |
| 2 | `GET /fuel_draft` | `GetFuelDraft` | ORM |
| 3 | `GET /fuel_grid`<br>`GET /fuel_grid?min_heat=50000` | `GetFuelGrid` | ORM |
| 4 | `POST /fuel_draft/create` | `CreateFuelDraft` | ORM (`db.Create`) |
| 5 | `POST /fuel_draft/publish` | `PublishFuelDraft` | ORM (`db.Model().Updates`) |
| 6 | `POST /fuel_grid/delete/:fuel_id` | `DeleteFuel` | **`SQL UPDATE` без ORM** |

Роутинг объявлен в [internal/api/server.go](internal/api/server.go),
контроллеры — в [internal/app/handler/fuel_handler.go](internal/app/handler/fuel_handler.go).

### Логическое удаление без ORM

`FuelRepository.SoftDeleteFuel` берёт у GORM низкоуровневое соединение
`database/sql` и выполняет запрос напрямую; результат читается курсором
`sql.Rows` через `RETURNING`:

```sql
UPDATE fuels
   SET fuel_status = $1
 WHERE fuel_id = $2
   AND fuel_status <> $1
RETURNING fuel_id
```

Физически строка не удаляется — меняется только статус, поэтому лайки в
таблице связей остаются нетронутыми.

### Разбор url для вкладки Network

* `http://localhost:3030/fuel_feed/2` — `2` это `fuel_id` услуги в таблице
  `fuels`; параметр пути читается через `ctx.Param("fuel_id")`.
* `http://localhost:3030/fuel_feed/2?next=true` — `next=true` переключает
  обработчик с «показать карточку 2» на «показать следующую за 2»; список
  закольцован.
* `http://localhost:3030/fuel_grid?min_heat=50000` — `min_heat` это параметр
  фильтрации: нижняя граница **теплоты сгорания в кДж/м³ при н.у.**
  Фильтрация выполняется на сервере запросом ORM
  `Where("heat_of_combustion_kj >= ?", minHeatKJ)`. Значение возвращается в
  шаблон и остаётся в положении ползунка после запроса.

Фильтрация сделана по **числовому** полю по теме, а не по текстовому.

---

## 5. База данных

Три таблицы, **каскадное удаление запрещено**: все внешние ключи объявлены с
`ON DELETE RESTRICT` / `ON UPDATE RESTRICT`.

### `users` — пользователи

| Столбец | Тип | Ключ |
|---|---|---|
| `user_id` | `bigserial` | **PK** |
| `login` | `varchar(64) NOT NULL` | уникальный |
| `full_name` | `varchar(128) NOT NULL` | |
| `password` | `varchar(255) NOT NULL` | хэш пароля (bcrypt), открытый пароль не хранится |

### `fuels` — услуги (виды топлива)

| Столбец | Тип | Ключ / смысл |
|---|---|---|
| `fuel_id` | `bigserial` | **PK** |
| `fuel_name` | `varchar(128) NOT NULL` | наименование |
| `combustion_note` | `text` | краткое описание реакции горения |
| `fuel_status` | `varchar(16) NOT NULL` | черновик / опубликован / удален |
| `image_url` | `varchar(512) NOT NULL` | url изображения, обязательный |
| `video_url` | `varchar(512) NOT NULL` | url видео, обязательный |
| `heat_of_combustion_kj` | `bigint` | **поле по теме:** теплота сгорания, кДж/м³ при н.у. |
| `ignition_temp_c` | `bigint` | **поле по теме:** температура воспламенения, °C |
| `created_at` | `timestamptz NOT NULL` | дата создания |
| `formed_at` | `timestamptz` | дата формирования (проставляется при публикации) |
| `creator_id` | `bigint NOT NULL` | **FK → `users.user_id`**, создатель |

Ограничение «у каждого пользователя не более одной услуги в статусе черновик»
реализовано частичным уникальным индексом:

```sql
CREATE UNIQUE INDEX idx_fuels_single_draft
    ON fuels (creator_id)
 WHERE fuel_status = 'черновик';
```

### `fuel_likes` — лайки, связь м-м «пользователь — топливо»

| Столбец | Тип | Ключ |
|---|---|---|
| `like_id` | `bigserial` | **PK** |
| `user_id` | `bigint NOT NULL` | **FK → `users.user_id`** |
| `fuel_id` | `bigint NOT NULL` | **FK → `fuels.fuel_id`** |

Пара (`user_id`, `fuel_id`) уникальна — один пользователь ставит карточке не
более одного лайка. Количество лайков вычисляется в обработчике запросом
`count(*)` с группировкой по `fuel_id`, во второй лабораторной лайки только
отображаются.

### Связи для ER-диаграммы (StarUML)

```
users (1) ──< (N) fuels        по fuels.creator_id
users (1) ──< (N) fuel_likes   по fuel_likes.user_id
fuels (1) ──< (N) fuel_likes   по fuel_likes.fuel_id
```

`fuel_likes` — ассоциативная таблица связи «многие ко многим» между `users` и
`fuels`.

### Стартовые данные

| ID | Топливо | Теплота сгорания, кДж/м³ | Статус |
|---|---|---|---|
| 1 | Метан | 35 800 | опубликован |
| 2 | Пропан-бутан | 108 000 | опубликован |
| 3 | Ацетилен | 56 000 | опубликован |
| 4 | Водород | 10 800 | опубликован |
| 5 | Метано-водородная смесь | — | **черновик** пользователя `petrova` — без своих медиа (в url записаны файлы по умолчанию) |
| 6 | Коксовый газ | 16 600 | **удален** — в интерфейсе не отображается |

Черновик №5 принадлежит `petrova`, а не пользователю из singleton (`ivanov`), поэтому
через API сразу можно добавить новую услугу — второй черновик у одного пользователя
запрещён.

Проверка, что удалённая карточка недоступна: `GET /fuel_feed/6` возвращает
`404` и страницу «Такого вида топлива нет в справочнике».

---

## 6. Медиафайлы

### Minio

Адреса изображения и видео хранятся в таблице `fuels` **двумя отдельными
полями** (`image_url`, `video_url`) и содержат полный url объекта в бакете:

```
http://localhost:9000/heat-fuel-media/propane_butane.mp4
```

Содержимое бакета `heat-fuel-media`: `metan.jpg`, `propan-bytan.jpg`,
`acetilen.png`, `vodorod.jpg` и `methane.mp4`, `propane_butane.mp4`,
`acetylene.mp4`, `hydrogen.mp4`.

Файлы, добавленные через `POST /api/fuels`, ложатся туда же под именами, которые
генерирует бэкенд: `fuel-image-<дата>-<время>-<8 hex>.<расш>` и
`fuel-video-…` ([internal/app/storage/minio.go](internal/app/storage/minio.go)).
В ответах API относительные пути медиа по умолчанию превращаются в полный адрес
на этом сервере (`http://localhost:3030/resources/media/…`), чтобы SPA могло их загрузить.

### Медиа по умолчанию

Если поле `image_url` или `video_url` пустое **или файл по ссылке недоступен**,
шаблон получает файл по умолчанию с самого SSR-сервера:

| Файл | Путь |
|---|---|
| изображение | `/resources/media/fuel_default.svg` |
| видео | `/resources/media/fuel_default.mp4` |

Доступность проверяется запросом `HEAD` в
[internal/app/storage/media.go](internal/app/storage/media.go), результат
кэшируется на 30 секунд. Поля url обязательные (`NOT NULL`), а файлы во второй
лабораторной на сервер не передаются, поэтому новой карточке при создании
в `image_url` и `video_url` записываются пути к медиа по умолчанию — они
показываются на всех трёх страницах.

Использование url во всех трёх шаблонах:

| Шаблон | Изображение | Видео |
|---|---|---|
| `templates/fuel_feed.html` | атрибут `poster` у `<video>` | `src` у `<video>` |
| `templates/fuel_grid.html` | `src` у `<img class="heat-card__image">` | — |
| `templates/fuel_draft.html` | `src` у `<img class="heat-form__preview">` | `src` у `<video>` |

---

## 7. Дизайн

### Сайт-источник

<https://antonio-merloni.ru/teplotvornaja-sposobnost-razlichnyh-vidov-topliva>
— раздел «Теплотворная способность различных видов топлива», по теме работы.

### Что именно скопировано

**Три основных цвета с точными кодами:**

| Код | Роль на сайте-источнике | Роль в приложении |
|---|---|---|
| `#FD2B06` | фирменный оранжево-красный акцент: ссылки при наведении, плашки «новинка», активные пункты меню | акцент: кнопки, активная вкладка, значения полей |
| `#212121` | цвет заголовков и основного текста | цвет заголовков и основного текста |
| `#FFFFFF` | фон страниц | фон страниц |

Дополнительно перенесены `#D93518` (числовые значения), `#EEEEEE` (рамки и
разделители), `#555555` (вторичный текст).

**Типографика:** Arial, заголовки жирные, в верхнем регистре, с увеличенным
межбуквенным интервалом — как в `#center_column h1` исходной темы.

**Форма карточек:** прямые углы без скруглений, рамка `1px solid #EEEEEE`,
белая подложка.

**Hover:** цвет текста и рамки уходит в `#FD2B06` за `0.15s` — как
`a.product_link:hover{color:#fd2b06}` в исходном CSS. Реализовано для карточек
плитки (`.heat-card:hover`), кнопок (`.heat-button:hover`) и вкладок
(`.heat-tabbar__item:hover`).

### Макет

Портретный режим под мобильный телефон. На широком экране «телефон»
шириной 420px центрируется, на узком занимает всю ширину.
Снизу на всех трёх страницах — панель вкладок: **Лента / Добавление / Плитка**.

### Страница «Лента» (`/fuel_feed`)

Короткое видео горения в вертикальной развёртке, проигрывается автоматически
(`autoplay muted loop playsinline`) и зациклено. Вся информация и кнопки
лежат **поверх видео**: снизу слева название и краткое описание с
переключателем **Больше / Меньше** (чистый CSS через скрытый чекбокс и
селектор `:checked ~`, без JavaScript), **справа** — параметры и иконки:
два поля по теме, лайк с количеством и кнопка **Следующий**
→ `/fuel_feed/<id>?next=true`.

### Страница «Добавление» (`/fuel_draft`)

Два состояния:

* **черновика нет** — форма создания: поля для изображения и видео, поле
  названия и кнопка **Далее** (`POST /fuel_draft/create`). Превью показывают
  медиа по умолчанию;
* **черновик есть** — открывается с заполненными полями, ниже краткое
  описание и оба поля по теме, кнопка **Опубликовать**
  (`POST /fuel_draft/publish`). Пустое описание или нечисловые поля по теме
  не публикуются: страница перерисовывается с сообщением об ошибке и
  сохраняет введённое.

### Страница «Плитка» (`/fuel_grid`)

Список всех опубликованных карточек в **два столбца**. В каждой карточке:
изображение, название, теплота сгорания, количество лайков из таблицы связей
и кнопка **Удалить** (логическое удаление, `POST /fuel_grid/delete/<id>`).
Сверху — фильтрация ползунком по теплоте сгорания. Клик по карточке открывает
ленту начиная с этой карточки (`/fuel_feed/<id>`).

---

## 8. Материалы вне репозитория (`рип\лаба3\`)

* `class_diagram.mdj` — диаграмма классов StarUML: 4 страницы фронтенда
  («page»), домены `FuelDomain` / `UserDomain` («interface») со всеми методами и url,
  singleton `CurrentUser`, `MinioStorage`, модели, таблицы БД; зависимости
  страницы → домены → модели → таблицы. `class_diagram.png` — её экспорт.
* `postman/heat_fuels_lr3.postman_collection.json` — коллекция запросов,
  `postman/media/` — изображение и видео для добавления услуги «Этилен».
* `docs/pokaz_lr3.md` — сценарий показа и ответы на контрольные вопросы.
* ER-диаграмма не менялась: схема БД в ЛР3 та же.

## 9. Порядок показа ЛР3 — где что смотреть

| Скриншоты | Что показать | Где |
|---|---|---|
| 1–10 | коллекция Postman; выполнить: список с фильтром, добавление с картинкой и видео, черновик, публикация, лента без ид, лента `?next=true`, лайк, удаление, регистрация | Postman, коллекция из `рип\лаба3\postman\` |
| 11–13 | изменённые данные через `select` | Adminer, запросы в `docs/pokaz_lr3.md` |
| 14–16 | модели, сериализаторы, функция-singleton и её использование | `internal/app/models/fuel.go`, `internal/app/serializers/`, `internal/app/auth/current_user.go` |
