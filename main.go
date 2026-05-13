package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab-api-migrate/config"
	"gitlab-api-migrate/logger"
)

type filterSet struct {
	inclSubgroups map[string]bool
	inclProjects  map[string]bool
	exclSubgroups map[string]bool
	exclProjects  map[string]bool
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config yaml file")
	genConfigPath := flag.String("gen-config-file", "", "generate example config to file and exit")
	showHelp := flag.Bool("help", false, "show help")
	flag.BoolVar(showHelp, "h", false, "show help")
	flag.Usage = printUsage
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *genConfigPath != "" {
		if err := generateExampleConfig(*genConfigPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Example config written to %s\n", *genConfigPath)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	sourceClient, err := gitlab.NewClient(cfg.Source.Token, gitlab.WithBaseURL(cfg.Source.URL+"/api/v4"))
	if err != nil {
		log.Error("Failed to create source GitLab client", map[string]string{"gitlab": "source"})
		os.Exit(1)
	}

	targetClient, err := gitlab.NewClient(cfg.Target.Token, gitlab.WithBaseURL(cfg.Target.URL+"/api/v4"))
	if err != nil {
		log.Error("Failed to create target GitLab client", map[string]string{"gitlab": "target"})
		os.Exit(1)
	}

	log.Info("Connection to source GitLab confirmed", map[string]string{"gitlab": "source", "url": cfg.Source.URL})
	log.Info("Connection to target GitLab confirmed", map[string]string{"gitlab": "target", "url": cfg.Target.URL})

	for _, gf := range cfg.Groups {
		filter := &filterSet{
			inclSubgroups: sliceToSet(gf.IncludeSubgroups),
			inclProjects:  sliceToSet(gf.IncludeProjects),
			exclSubgroups: sliceToSet(gf.ExcludeSubgroups),
			exclProjects:  sliceToSet(gf.ExcludeProjects),
		}

		projects, err := listAllGroupProjects(sourceClient, gf.Path)
		if err != nil {
			log.Error(fmt.Sprintf("Failed to list projects in group %s: %v", gf.Path, err))
			continue
		}

		for _, project := range projects {
			if !shouldMigrate(project, filter) {
				continue
			}

			relativePath := strings.TrimPrefix(project.Namespace.FullPath, gf.Path+"/")
			if relativePath == project.Namespace.FullPath {
				relativePath = ""
			}

			groups := buildGroupPath(project.Namespace.FullPath)

			targetFullPath := cfg.ParentGroup + "/" + project.Namespace.FullPath
			newGroupID := findGroupID(targetClient, targetFullPath)
			if newGroupID == 0 {
				log.Info(fmt.Sprintf("Creating subgroups: %s/%s", cfg.ParentGroup, project.Namespace.FullPath))
				err = CreateGroup(targetClient, cfg.ParentGroup, groups)
				if err != nil {
					log.Error(fmt.Sprintf("Failed to create subgroups: %v", err), map[string]string{"group": project.Namespace.FullPath})
					continue
				}
			}

			sourceRepoName := fmt.Sprintf("%s/%s", project.Namespace.FullPath, project.Path)
			targetRepoName := fmt.Sprintf("%s/%s/%s", cfg.ParentGroup, project.Namespace.FullPath, project.Path)
			tmpDir := filepath.Join(os.TempDir(), "gitlab-migrate-"+project.Path)

			_, _, err := MirrorRepo(cfg.Source, cfg.Target, sourceRepoName, targetRepoName, tmpDir, project.Path, log)
			if err != nil {
				log.Error(fmt.Sprintf("Failed to migrate project: %v", err), map[string]string{"project": targetRepoName})
			}
		}
	}

	log.Info("Migration completed")
}

func listAllGroupProjects(client *gitlab.Client, groupPath string) ([]*gitlab.Project, error) {
	opts := &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 50,
		},
		IncludeSubGroups: gitlab.Ptr(true),
	}

	var all []*gitlab.Project
	for {
		projects, resp, err := client.Groups.ListGroupProjects(groupPath, opts)
		if err != nil {
			return nil, fmt.Errorf("list projects for group %s: %w", groupPath, err)
		}
		all = append(all, projects...)
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func shouldMigrate(p *gitlab.Project, f *filterSet) bool {
	if f.exclProjects[p.Path] {
		return false
	}
	if len(f.inclProjects) > 0 && !f.inclProjects[p.Path] {
		return false
	}

	subgroups := strings.Split(p.Namespace.FullPath, "/")
	if len(subgroups) > 0 {
		lastSubgroup := subgroups[len(subgroups)-1]
		if f.exclSubgroups[lastSubgroup] {
			return false
		}
	}

	if len(f.inclSubgroups) > 0 {
		for _, sg := range subgroups {
			if f.inclSubgroups[sg] {
				return true
			}
		}
		return false
	}

	return true
}

func buildGroupPath(fullPath string) GroupsPath {
	parts := strings.Split(fullPath, "/")
	groups := make(GroupsPath, 0, len(parts))
	for _, p := range parts {
		groups = append(groups, GroupsPathData{Name: p, Path: p})
	}
	return groups
}

func sliceToSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}

func generateExampleConfig(path string) error {
	example := `# Настройки подключения к исходному GitLab (откуда переносим)
source:
  # URL GitLab инстанса (без /api/v4)
  url: "https://gitlab.example.com"
  # Personal Access Token для API
  token: "glpat-xxxxxxxxxxxxx"
  # Транспорт для clone: "ssh" или "https"
  transport: "ssh"
  # SSH порт (только для transport: ssh, по умолчанию 22)
  ssh_port: 22
  # Путь к SSH ключу (только для transport: ssh)
  ssh_key: "~/.ssh/id_rsa"

# Настройки подключения к целевому GitLab (куда переносим)
target:
  url: "https://gitlab2.example.com"
  token: "glpat-yyyyyyyyyyyyy"
  transport: "ssh"
  ssh_port: 22
  ssh_key: "~/.ssh/id_rsa"

# Родительская группа в целевом GitLab, в которую будут помещены все проекты
parent_group: "migrated"

# Список групп для миграции с правилами фильтрации
groups:
  # Перенести все проекты и подгруппы из группы "mygroup"
  - path: "mygroup"

  # Из группы "another" перенести только проекты из подгрупп "sub1" и "sub2"
  - path: "another"
    include_subgroups:
      - "sub1"
      - "sub2"

  # Из группы "legacy" перенести всё, кроме подгруппы "archive" и проекта "old-project"
  - path: "legacy"
    exclude_subgroups:
      - "archive"
    exclude_projects:
      - "old-project"

# Настройки логирования
log:
  # Формат: "plaintext" или "json"
  format: "plaintext"
  # Вывод: "stdout", "file", или "both"
  output: "stdout"
  # Путь к файлу лога (требуется при output: file или both)
  # file: "/var/log/gitlab-migrate.log"
  # Добавить unix timestamp в JSON (только для format: json)
  # json_unix_timestamp: true
`
	return os.WriteFile(path, []byte(example), 0644)
}

func printUsage() {
	fmt.Print(`GitLab Migrator - перенос проектов и групп между GitLab инстансами

Использование:
  gitlab-migrate [-config <file>] [-gen-config-file <file>] [-help]

Флаги:
  -config <file>           путь к YAML конфигурационному файлу (по умолчанию: config.yaml)
  -gen-config-file <file>  сгенерировать пример конфига в указанный файл и выйти
  -help, -h                показать эту справку

Формат конфигурационного файла:
  Смотрите config.yaml в директории проекта.

Описание:
  Приложение переносит проекты и группы из исходного GitLab в целевой,
  сохраняя иерархическую структуру групп. Все проекты создаются внутри
  родительской группы (parent_group) в целевом GitLab.

  Поддерживаемые транспорты:
    - SSH (с настраиваемым портом и ключом)
    - HTTPS (с аутентификацией через Personal Access Token)

  Транспорт настраивается отдельно для source и target, что позволяет
  клонировать через SSH, а пушить через HTTPS и наоборот.

  Логирование поддерживает:
    - Формат: plaintext (человекочитаемый) или JSON (для OpenSearch)
    - Вывод: stdout, файл, или оба одновременно
    - Временные метки в каждом сообщении
    - Информация о времени переноса и размере репозитория
`)
}
