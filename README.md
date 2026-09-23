# sshocks

Локальный SOCKS5-прокси поверх SSH-туннеля. Консольное приложение на Go,
одна гораoutine-модель, один процесс. Клиенты (браузеры, мессенгеры)
обращаются к локальному SOCKS5-листенеру; трафик прокидывается к целевому
`host:port` через SSH-туннель на удалённый сервер. Все настройки — в YAML,
важные события — в файл лога.

Имя целевого хоста из SOCKS5-запроса разрешается **на удалённой стороне**
(через `ssh.Client.Dial`), что обходит локальные DNS-ограничения.

## Возможности (v1)

- Транспорт: `golang.org/x/crypto/ssh` + минимальный SOCKS5-сервер (RFC 1928).
- ТОЛЬКО TCP-сессии, метод `CONNECT`. Без UDP-ассоциации, без SOCKS4.
- Аутентификация SSH: пароль и приватный ключ. **Приоритет — пароль**.
- Проверка host key **выключена** (`ssh.InsecureIgnoreHostKey()`).
- Авто-реконнект SSH: экспоненциальный backoff 1s → 30s. Слушатель SOCKS5
  продолжает работать при обрыве SSH; новые сессии отклоняются до реконнекта.
- Graceful shutdown: SIGINT/SIGTERM → отмена контекста → drain до `drain_timeout`
  → форс-закрытие → выход 0.

## Сборка

```sh
go build ./...
go vet ./...
go test ./internal/config/...   # обязательный юнит
```

Модуль `sshocks`, Go 1.27.1. Внешние зависимости: `golang.org/x/crypto/ssh`,
`gopkg.in/yaml.v3`.

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
  listen: 127.0.0.1:1080    # только локально
log:
  file: ./sshocks.log
  level: info
drain_timeout: 5s
```

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

Автоматических интеграционных тестов в v1 нет; обязательны юниты парсера/
валидатора конфига (`internal/config`) и SOCKS5-хэндшейка (`internal/socks`).

## Организация кода

Структура `golang-standards` + Clean-Архитектурное разделение слоёв (без
фейкового репозитория — у приложения нет хранилища данных).

```
internal/config   структура, загрузка, валидация конфига (+ юнит)
internal/tunnel   SSH-коннектор (auth-выбор, autoreconnect) + dialer
internal/socks    SOCKS5-сервер: handshake, server, handler, ошибки
internal/app      wiring + Run/Shutdown(drain)
cmd/sshocks       точка входа (инициализация, сигналы)
```

## Открытые вопросы (v2)

- `ssh.HostKeyCallback` по known_hosts.
- Ротация логов (lumberjack / size-based).
- Jitter в backoff.
- systemd/launchd-юниты.
