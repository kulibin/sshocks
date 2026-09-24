# sshocks

Локальный SOCKS5-прокси поверх SSH-туннеля. Консольное приложение на Go,
одна горуoutine-модель, один процесс. Клиенты (браузеры, мессенгеры)
обращаются к локальному SOCKS5-листенеру; трафик прокидывается к целевому
`host:port` через SSH-туннель на удалённый сервер. Все настройки — в YAML,
важные события — в файл лога.

Имя целевого хоста из SOCKS5-запроса разрешается **на удалённой стороне**
(через `ssh.Client.Dial`), что обходит локальные DNS-ограничения. Опционально
через `dns.servers` имя можно разрешать **локально** против указанных DNS-серверов
(напр. `8.8.8.8`, `1.1.1.1`), а полученный IP прокидывать по туннелю — это
обходит сломанный/заблокированный локальный резолвер.

## Возможности (v1)

- Транспорт: `golang.org/x/crypto/ssh` + минимальный SOCKS5-сервер (RFC 1928)
  и опциональный HTTP/1.1 `CONNECT`-прокси.
- ТОЛЬКО TCP-сессии, метод `CONNECT` (в обоих прокси). Без UDP-ассоциации, без
  SOCKS4. Без аутентификации на стороне SOCKS5/HTTP (локальный инструмент).
- Два слушателя поверх одной SSH-сессии: SOCKS5 (`socks.listen`) и HTTP
  CONNECT (`http.listen`, опц., пусто = выкл). Оба туннелируют трафик через
  одну и ту же SSH-сессию через `Connector.DialTarget`.
- Аутентификация SSH: пароль и приватный ключ. **Приоритет — пароль**: методы
  аутентификации предлагаются SSH-серверу в порядке пароль → ключ.
- Проверка host key **выключена** (`ssh.InsecureIgnoreHostKey()`).
- Здоровье SSH держится проактивно: «watcher» раз в `keepAliveInterval` (1s)
  гоняет `SendRequest("keepalive@sshocks.local")` по сессии; по ошибке транспорта
  сессия считается потерянной и запускается авто-реконнект.
- Авто-реконнект SSH: экспоненциальный backoff 1s → 30s (без jitter). Слушатели
  SOCKS5/HTTP продолжают работать при обрыве SSH; новые сессии отклоняются до
  реконнекта.
- Graceful shutdown: SIGINT/SIGTERM → отмена контекста → drain обоих прокси до
  `drain_timeout` → форс-закрытие → `Connector.Close()` → flush+close логгера →
  выход 0.
- Локальное разрешение DNS (опц.): блок `dns.servers` (см. ниже).
- Версия сборки, выводимая в лог при старте: `v1.0.0` (константа `app.Version`).

## Сборка

```sh
go build ./...
go vet ./...
go test ./...                    # юниты config, tunnel, socks, httpproxy
```

Модуль `sshocks`, Go 1.27.1. Внешние зависимости: `golang.org/x/crypto/ssh`,
`gopkg.in/yaml.v3` (косвенно `golang.org/x/sys`).

## Запуск

```sh
cp config.example.yaml config.yaml   # заполнить, chmod 600
go run ./cmd/sshocks --config config.yaml
# или собрать бинарь
go build -o sshocks ./cmd/sshocks
./sshocks --config config.yaml
```

Флаг `--config` указывает путь к YAML-конфигу (дефолт `config.yaml`).

## Конфиг

См. `config.example.yaml`. Обязательны `ssh.host` и `ssh.user`. Если и
`ssh.password`, и `ssh.key` пусты — ошибка (выход ≠ 0). `socks.listen`
без `:` — ошибка. Таймауты парсятся как `time.Duration`; невалидный — выход ≠ 0.

```yaml
ssh:
  host: 1.2.3.4        # обязателен
  port: 22             # дефолт
  user: alice          # обязателен
  password: ""         # приоритет аутентификации
  key: ~/.ssh/id_ed25519
  key_passphrase: ""
  timeout: 30s
socks:
  listen: 127.0.0.1:1080     # только локально
http:
  listen: 127.0.0.1:8080     # HTTP CONNECT-прокси (опц., пусто = выкл)
dns:
  servers:                    # явные DNS-серверы для локального резолвинга
     - 8.8.8.8                 # пусто = имя разрешается удалённой стороной
     - 8.8.4.4                 # (дефолт); задайте, если локальный резолвер
     - 1.1.1.1                 #  сломан/заблокирован — IP идёт по туннелю
log:
  file: ./sshocks.log
  level: info
drain_timeout: 5s
```

`dns.servers` (опц.): список DNS-серверов (IP, напр. `8.8.8.8`, `8.8.4.4`,
`1.1.1.1`). Если задан — имя целевого хоста из SOCKS5/HTTP-запроса разрешается
на этой машине против этих серверов (обходя сломанный/заблокированный локальный
резолвер), а полученный IP прокидывается через SSH-туннель. Если список пуст —
имя разрешается на удалённой стороне (поведение по умолчанию). Недействительный
IP в списке — ошибка валидации (выход ≠ 0). Разрешение работает одинаково для
SOCKS5 и HTTP-прокси (общий `Connector.DialTarget`): IP-литерал прокидывается
напрямую, имя — против `dns.servers` или удалённой стороной.

## HTTP CONNECT-прокси

Помимо SOCKS5, приложение поднимает **опциональный** HTTP/1.1 `CONNECT`-прокси на
`http.listen` (дефолт `127.0.0.1:8080`; пустое значение или отсутствие блока =
выкл). Он принимает только команду `CONNECT host:port HTTP/1.1`, отбрасывает
оставшиеся заголовки и прокидывает трафик через ту же SSH-сессию, что и SOCKS5.

```sh
curl -x 127.0.0.1:8080 https://example.com/
curl -x 127.0.0.1:8080 https://example.com:8443/
```

Отклики дублируются HTTP-статусами в ответе на `CONNECT`: `400` — некорректная
строка запроса или адрес, `502` — целевой dial завершился ошибкой. В отличие от
SOCKS5, HTTP-прокси поддерживает только `CONNECT` (без `GET`/`POST`, без UDP).

## Логирование

Человекочитаемый текст, формат строки:
`2026-09-23T08:12:57Z ERROR ssh: connect failed: ...`. Уровень
`debug|info|warn|error` из конфига (дефолт `info`). Файл — `log.file`
(дефолт `./sshocks.log`). **Ротация в v1 не предусмотрена** (follow-up).

## Риски и митигации

- **Приоритет пароля над ключом** — нетипично для безопасности; осознанный
  выбор пользователя. Митигация: документация, `key_passphrase` шифрует ключ.
- **Отсутствие host-key verification** — риск MITM. Митигация: документация;
  в v2 — `ssh.HostKeyCallback`/known_hosts.
- **Отсутствие ротации лог** — файл растёт. Митигация: документация + follow-up.
- **Backoff без jitter** — в v2 добавить.
- **Пароль/ключ в конфиге в открытом виде** — митигация: `chmod 600 config.yaml`.

## Smoke-тест через docker

```sh
# 1. поднять локальный sshd на 127.0.0.1:2222 + эхо-сервер на 8080
docker compose -f smoke/docker-compose.yml up -d

# 2. запуск и проверка через туннель
./smoke/smoke.sh
```

Ожидаемый результат: `curl` получает ответ эхо-сервера через SSH-туннель;
лог содержит все важные события. `kill -INT` → drain + выход 0.

Автоматических интеграционных тестов в v1 нет; обязательны юниты:
`internal/config` (парсер/валидатор), `internal/socks` (хэндшейк),
`internal/httpproxy` (CONNECT-хэндшейк), `internal/tunnel` (выбор аутентификации,
DNS-resolver).

## Организация кода

Структура `golang-standards` + Clean-Архитектурное разделение слоёв (без
фейкового репозитория — у приложения нет хранилища данных).

```
internal/config      структура, загрузка, валидация конфига (+ юнит, duration.go — YAML-таймауты)
internal/tunnel      SSH-коннектор (auth-выбор, keepalive+autoreconnect) + dialer + DNS-resolver (+ юниты)
internal/socks       SOCKS5-сервер (RFC 1928): handshake, server, handler, ошибки (+ юнит)
internal/httpproxy   HTTP/1.1 CONNECT-прокси: handshake, server, handler, ошибки (+ юнит)
internal/log         level-aware file-логгер (Flush/Close, Discard)
internal/app         wiring config+logger+tunnel+socks+httpproxy, New/Run/Shutdown(drain)
cmd/sshocks          точка входа (флаги, конфиг, сигналы, drain)
```

## Открытые вопросы (v2)

- `ssh.HostKeyCallback` по known_hosts.
- Ротация логов (lumberjack / size-based).
- Jitter в backoff.
- systemd/launchd-юниты.
