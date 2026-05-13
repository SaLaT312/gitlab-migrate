# GitLab Migrator

Перенос проектов и групп между GitLab инстансами с сохранением иерархической структуры.

## Возможности

- Перенос групп и проектов из исходного GitLab в целевой
- Сохранение структуры вложенных групп
- Поддержка clone/push через **SSH** и **HTTPS** — транспорт настраивается отдельно для source и target
- Фильтрация: перенос только нужных подгрупп и проектов, исключение ненужных
- Логирование с временными метками (plaintext или JSON)
- Вывод логов: stdout, файл, или оба одновременно

## Требования

- Go 1.22+
- SSH-ключ для доступа к GitLab (при использовании SSH транспорта)
- Personal Access Token для source и target GitLab (с правами на чтение/запись)

## Установка

```bash
go build -o gitlab-migrate .
```

## Конфигурация

Все настройки задаются в YAML файле (по умолчанию `config.yaml`):

```yaml
source:
  url: "https://gitlab.example.com"
  token: "glpat-xxxxxxxx"
  transport: "ssh"         # ssh или https
  ssh_port: 22
  ssh_key: "~/.ssh/id_rsa"

target:
  url: "https://gitlab2.example.com"
  token: "glpat-yyyyyyyy"
  transport: "https"
  ssh_port: 22
  ssh_key: "~/.ssh/id_rsa"

parent_group: "migrated"        # всё создаётся внутри этой группы

groups:
  - path: "docker"          # все проекты и подгруппы
  - path: "k8"
    include_subgroups:      # только из указанных подгрупп
      - "helm"
      - "kustomize"
  - path: "myapp"
    exclude_subgroups:      # исключить подгруппу
      - "test_infra"
    exclude_projects:       # исключить проект
      - "test_monitor"
```

### Параметры групп

| Поле | Описание |
|------|----------|
| `path` | Путь группы в исходном GitLab |
| `include_subgroups` | Переносить проекты только из указанных подгрупп |
| `include_projects` | Переносить только указанные проекты |
| `exclude_subgroups` | Исключить подгруппы из переноса |
| `exclude_projects` | Исключить проекты из переноса |

Если не заданы `include_*`, переносятся все проекты (кроме исключённых).

### Логирование

```yaml
log:
  format: "plaintext"       # plaintext или json
  output: "stdout"          # stdout, file, или both
  file: "/var/log/gitlab-migrate.log"
  json_unix_timestamp: true # добавить unix timestamp в JSON
```

## Запуск

```bash
./gitlab-migrate -config config.yaml
```

### Флаги

| Флаг | Описание |
|------|----------|
| `-config <file>` | Путь к конфигурационному файлу (по умолчанию: `config.yaml`) |
| `-gen-config-file <file>` | Сгенерировать пример конфигурационного файла |
| `-help`, `-h` | Показать справку |

## Как это работает

1. Приложение читает YAML конфиг
2. Проверяет подключение к source и target GitLab через API
3. Для каждой группы из `groups` получает список проектов (с подгруппами) через API source GitLab
4. Применяет фильтры (include/exclude)
5. Создаёт недостающие группы в target GitLab через API
6. Клонирует репозиторий зеркальным clone из source и пушит mirror push в target
7. Логирует результат каждого проекта (время переноса, размер)

## Примеры конфигов

**Только SSH (source и target за одним SSH сервером):**
```yaml
source:
  transport: "ssh"
  ssh_port: 22
target:
  transport: "ssh"
  ssh_port: 22286
```

**Source через SSH, target через HTTPS:**
```yaml
source:
  transport: "ssh"
target:
  transport: "https"
  token: "glpat-zzzzzzzz"
```

**Логи в JSON для OpenSearch:**
```yaml
log:
  format: "json"
  output: "both"
  file: "/var/log/gitlab-migrate.json"
  json_unix_timestamp: true
```
