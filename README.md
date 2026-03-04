<p align="center">
  <img src="GoLeM.png" width="600" alt="GoLeM — a tiny wizard commanding clay golems to do the heavy lifting" />
</p>

<h1 align="center">GoLeM</h1>

<p align="center">
  <strong>One wizard. Unlimited golems. Zero Anthropic API costs.</strong>
</p>

<p align="center">
  Spawn autonomous Claude Code agents powered by GLM-5 via Z.AI.<br>
  Each golem is a full Claude Code instance — reads files, edits code, runs tests, uses MCP servers and skills.<br>
  You stay on Opus. Your golems run free and parallel through Z.AI. Ship faster.
</p>

---

![Architecture](docs/architecture.svg?v=4)

## установка

Нужно: [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code), ключ [Z.AI Coding Plan](https://z.ai/subscribe), Go 1.25+.

```bash
go install github.com/veschin/GoLeM/cmd/glm@latest
glm _install
```

Или из исходников:

```bash
git clone https://github.com/veschin/GoLeM.git
cd GoLeM
go build -o glm ./cmd/glm/
./glm _install
```

`_install` делает следующее: создаёт `~/.config/GoLeM/`, спрашивает Z.AI API ключ (сохраняет в `zai_api_key` с правами 0600), записывает `glm.toml` с `permission_mode`, создаёт `~/.claude/subagents/`, инжектит инструкции в `~/.claude/CLAUDE.md` между маркерами `<!-- GLM-SUBAGENT-START -->` и `<!-- GLM-SUBAGENT-END -->`, и для source-инсталлов ставит symlink в `~/.local/bin/glm`.

Если у тебя уже был ключ в `~/.config/zai/env` (старый формат) — он мигрируется автоматически.

Shell completions ставятся отдельно:

- **Bash:** `~/.local/share/bash-completion/completions/glm`
- **Fish:** `~/.config/fish/completions/glm.fish`
- **Zsh:** `~/.config/zsh/completions/_glm` (нужно добавить `fpath=(~/.config/zsh/completions $fpath)` в `.zshrc`)

Перезапусти шелл после установки.

## обновление и удаление

```bash
glm update          # git pull + go build для source, go install для go-install
glm _uninstall      # удаляет symlink, секцию в CLAUDE.md, спрашивает про ключи и job-артефакты
```

## команды

```
glm session [flags] [claude flags]     # интерактивный claude code (syscall.Exec, заменяет процесс)
glm run   [flags] "prompt"             # синхронно: создаёт job, запускает claude, печатает stdout, удаляет job dir
glm start [flags] "prompt"             # асинхронно: то же, но в горутине, сразу печатает job ID
glm chain [flags] "p1" "p2" ...        # последовательно: stdout шага N инжектится в промпт шага N+1
glm status  JOB_ID                     # читает status файл
glm result  JOB_ID                     # читает stdout.txt
glm log     JOB_ID                     # читает changelog.txt
glm list    [--status S] [--since D]   # перебирает job dirs с фильтрами
glm clean   [--days N]                 # удаляет старые jobs
glm kill    JOB_ID                     # SIGTERM → SIGKILL через секунду
glm doctor                             # 6 проверок: claude CLI, API key, Z.AI reachability, models, slots, platform
glm config  {show|set KEY VAL}         # show показывает все значения с источником, set пишет в glm.toml
glm update                             # обновляет glm
glm version                            # версия
```

Примеры:

```bash
glm run -d ~/project "найди баги в auth.go"
glm run -m glm-4 "отрефакторь auth"               # все три слота → glm-4
glm run --opus glm-4.7 --haiku glm-4 "задача"     # per-slot модели
glm session --sonnet glm-4                         # сессия с кастомным sonnet
glm run --unsafe "deploy hotfix"                   # bypass permission checks
glm start -t 600 "долгая задача"                   # async с таймаутом 10 минут
glm list --status running
glm list --status done,failed --since 2h
glm list --json                                    # JSON для скриптинга
glm chain "напиши тесты для auth.go" "запусти тесты и исправь ошибки"
glm chain --continue-on-error "шаг1" "шаг2" "шаг3"
```

## флаги

Работают с `session`, `run`, `start`, `chain`.

| Флаг | Что делает |
|---|---|
| `-m`, `--model MODEL` | Все три слота (opus, sonnet, haiku) → MODEL |
| `--opus MODEL` | Только opus слот |
| `--sonnet MODEL` | Только sonnet слот |
| `--haiku MODEL` | Только haiku слот |
| `-d DIR` | Рабочая директория |
| `-t SEC` | Таймаут в секундах (для session игнорируется) |
| `--unsafe` | bypassPermissions |
| `--mode MODE` | Режим разрешений: `bypassPermissions`, `acceptEdits`, `default`, `plan` |
| `--json` | JSON-вывод для list, status, result, log |

Claude Code использует три слота внутри: тяжёлые задачи — opus, обычные — sonnet, быстрые — haiku. По умолчанию все три указывают на `glm-4.7`. `-m` меняет все сразу, `--opus`/`--sonnet`/`--haiku` — по отдельности.

`session` пробрасывает неизвестные флаги напрямую в `claude` — например `--resume`, `--verbose`.

`chain` дополнительно принимает `--continue-on-error`: без него цепочка останавливается при первом неудачном шаге.

## конфигурация

`~/.config/GoLeM/glm.toml` — читается при каждом запуске `glm`. Приоритет: флаг CLI > переменная окружения > glm.toml > hardcoded дефолт.

```bash
glm config show                   # все значения с пометками (default), (config), (env)
glm config set max_parallel 5
glm config set model glm-4
```

| Ключ | Env | Дефолт | Описание |
|---|---|---|---|
| `model` | `GLM_MODEL` | `glm-4.7` | Дефолтная модель для всех трёх слотов |
| `opus_model` | `GLM_OPUS_MODEL` | (model) | Модель для тяжёлых задач |
| `sonnet_model` | `GLM_SONNET_MODEL` | (model) | Модель для обычных задач |
| `haiku_model` | `GLM_HAIKU_MODEL` | (model) | Модель для быстрых задач |
| `permission_mode` | `GLM_PERMISSION_MODE` | `bypassPermissions` | Дефолтный режим разрешений |
| `max_parallel` | `GLM_MAX_PARALLEL` | `3` | Макс. параллельных агентов |

Debug логгирование (`GLM_DEBUG=1`) читается напрямую из окружения, не из конфига:

```bash
GLM_DEBUG=1 glm run "задача"                  # debug в stderr
GLM_DEBUG=1 GLM_LOG_FORMAT=json glm doctor    # структурированные JSON-логи
GLM_LOG_FILE=/tmp/glm.log glm run "задача"    # дополнительно в файл
```

Уровни логов: `[D]` debug, `[+]` info, `[!]` warn, `[x]` error. Цвета на TTY, plain text при пайпе.

### мульти-провайдер

`glm.toml` поддерживает секции `[providers.NAME]` — можно подключить любой OpenAI-совместимый эндпойнт вместо Z.AI:

```toml
default_provider = "custom"

[providers.custom]
base_url = "https://custom.api.com/v1/anthropic"
api_key_file = "~/.config/GoLeM/custom_key"
timeout_ms = "5000"
opus_model = "custom-opus"
sonnet_model = "custom-sonnet"
haiku_model = "custom-haiku"
```

Если секций `[providers.*]` нет — используются hardcoded Z.AI defaults (`https://api.z.ai/api/anthropic`, `~/.config/GoLeM/zai_api_key`).

## как это работает

**session** — это `syscall.Exec`. `glm session` строит `argv` и окружение, потом полностью заменяет себя процессом `claude`. Job-директорий нет, вывод не захватывается. В окружение инжектируется: `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `API_TIMEOUT_MS`, и три слота моделей (`ANTHROPIC_DEFAULT_OPUS_MODEL` и т.д.).

**run** — синхронный. Создаёт job dir под `~/.claude/subagents/<project-id>/<job-id>/`, пишет метаданные (prompt.txt, workdir.txt, model.txt и т.д.), запускает `claude -p --no-session-persistence --output-format json` с контекстом таймаута, захватывает stdout в `raw.json` и stderr в `stderr.txt`. После завершения парсит `raw.json` — извлекает `.result` в `stdout.txt` и генерирует `changelog.txt` из tool_use блоков. Потом печатает `stdout.txt` и удаляет job dir.

**start** — то же самое, но в горутине. Job ID печатается немедленно, процесс ждёт SIGINT/SIGTERM.

**chain** — запускает шаги последовательно. Промпт шага N+1 получает результат шага N в формате `"Previous agent result:\n{stdout}\n\nYour task:\n{prompt}"`. Job dir каждого шага остаётся.

Project ID вычисляется из абсолютного пути рабочей директории: `{basename}-{crc32(path)}`. Так jobs одного проекта группируются, но не конфликтуют при одинаковых именах.

**audit** — каждый job пишет changelog из tool_use: Edit, Write, Bash (только delete-команды), NotebookEdit. Полная история tool calls — в `raw.json`.

```bash
glm log job-20260226-143022-a1b2c3d4
# EDIT src/auth.py: 142 chars
# WRITE tests/test_auth.py
# DELETE via bash: rm tmp/cache.db
```

Если агент упёрся в permission wall — статус становится `permission_error`, не просто `failed`.

## тестирование

```bash
go test ./...                                     # unit тесты, без API-вызовов
go test -tags e2e ./internal/e2e/... -v           # e2e с реальным claude и API ключом
```

E2E тесты помечены `//go:build e2e` и требуют рабочий `claude` CLI и валидный ключ.

## коды ошибок

| Код | Смысл |
|---|---|
| 0 | Успех |
| 1 | User error (неправильные аргументы, невалидный конфиг) |
| 3 | Not found (job не существует) |
| 124 | Timeout |
| 127 | Dependency missing (claude CLI не найден) |

Ошибки пишутся в stderr в формате `err:<category> "message"` для программного разбора.

## файлы

**Runtime:**

| Путь | Что |
|---|---|
| `~/.local/bin/glm` | Binary или symlink (для source-инсталлов) |
| `~/.claude/CLAUDE.md` | Инструкции делегирования (между маркерами) |
| `~/.config/GoLeM/glm.toml` | Конфиг: модели, разрешения, параллелизм |
| `~/.config/GoLeM/zai_api_key` | Z.AI API ключ (chmod 600) |
| `~/.config/GoLeM/config.json` | Метаданные установки (version, install_mode) |
| `~/.claude/subagents/<project>/<job-id>/` | Job артефакты |

Внутри job dir: `prompt.txt`, `workdir.txt`, `model.txt`, `permission_mode.txt`, `started_at.txt`, `finished_at.txt`, `raw.json`, `stdout.txt`, `stderr.txt`, `changelog.txt`, `status`, `pid.txt`, `exit_code.txt` (только при ошибке).

**Исходники (Go):**

| Путь | Что |
|---|---|
| `cmd/glm/main.go` | Точка входа, CLI dispatch, signal handling |
| `internal/config/config.go` | Загрузка TOML, env overrides, валидация |
| `internal/config/provider.go` | Мульти-провайдер: LoadProvider, ListProviders, ResolveModelEnv |
| `internal/cmd/session.go` | SessionCmd: строит argv+env для syscall.Exec |
| `internal/cmd/chain.go` | ChainCmd: последовательные шаги с инжекцией stdout |
| `internal/cmd/flags.go` | ParseFlags, Validate |
| `internal/cmd/doctor.go` | DoctorCmd: 6 диагностических проверок |
| `internal/cmd/install.go` | InstallCmd, UninstallCmd, UpdateCmd, InjectClaudeMD |
| `internal/claude/claude.go` | Execute: subprocess, env, захват вывода |
| `internal/claude/parser.go` | ParseRawJSON, GenerateChangelog |
| `internal/job/job.go` | Job lifecycle, status machine, FindJobDir |
| `internal/slot/slot.go` | Concurrency control: flock/mkdir, PID liveness |
| `internal/log/log.go` | Структурированный логгер (human + JSON) |
| `internal/exitcode/exitcode.go` | Типизированные ошибки, коды выхода |
| `internal/e2e/` | End-to-end тесты (//go:build e2e) |

## платформы

Linux (amd64, arm64), macOS (amd64, arm64), WSL.

## troubleshooting

```bash
glm doctor          # claude_cli, api_key, zai_reachable, models, slots, platform
```

| Симптом | Фикс |
|---|---|
| `claude CLI not found` | Установи Claude Code, добавь в PATH |
| `credentials not found` | Запусти `glm _install` |
| Пустой вывод после run | Проверь `glm result JOB_ID` или `~/.claude/subagents/.../stderr.txt` |
| `~/.local/bin` не в PATH | `export PATH="$HOME/.local/bin:$PATH"` в `.bashrc`/`.zshrc` |
| Jobs зависают в queued | `glm doctor` — смотри slots, чисти стейл-джобы `glm clean --days 0` |
| Статус `permission_error` | Добавь `--unsafe` или смени `permission_mode` на `bypassPermissions` |
