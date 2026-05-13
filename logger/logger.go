package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"gitlab-api-migrate/config"
)

type Level int

const (
	LevelInfo  Level = iota
	LevelWarn  Level = iota
	LevelError Level = iota
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp     string `json:"@timestamp"`
	TimestampUnix int64  `json:"@timestamp_unix,omitempty"`
	Level         string `json:"level"`
	Message       string `json:"message"`
	Duration      string `json:"duration,omitempty"`
	Size          string `json:"size,omitempty"`
	Project       string `json:"project,omitempty"`
	GitLab        string `json:"gitlab,omitempty"`
}

type Logger struct {
	mu      sync.Mutex
	cfg     config.LogConfig
	writers []io.Writer
	file    *os.File
}

func New(cfg config.LogConfig) (*Logger, error) {
	l := &Logger{cfg: cfg}

	switch cfg.Output {
	case config.LogOutputStdout:
		l.writers = append(l.writers, os.Stdout)
	case config.LogOutputFile:
		if cfg.File == "" {
			return nil, fmt.Errorf("log.file is required when log.output is 'file'")
		}
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", cfg.File, err)
		}
		l.file = f
		l.writers = append(l.writers, f)
	case config.LogOutputBoth:
		if cfg.File == "" {
			return nil, fmt.Errorf("log.file is required when log.output is 'both'")
		}
		f, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", cfg.File, err)
		}
		l.file = f
		l.writers = append(l.writers, os.Stdout, f)
	}

	return l, nil
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) log(level Level, msg string, fields map[string]string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	humanTime := now.Format("15:04:05 02-01-2006")

	if l.cfg.Format == config.LogFormatJSON {
		entry := LogEntry{
			Timestamp: humanTime,
			Level:     level.String(),
			Message:   msg,
		}
		if l.cfg.JSONUnix {
			entry.TimestampUnix = now.Unix()
		}
		for k, v := range fields {
			switch k {
			case "duration":
				entry.Duration = v
			case "size":
				entry.Size = v
			case "project":
				entry.Project = v
			case "gitlab":
				entry.GitLab = v
			}
		}
		data, _ := json.Marshal(entry)
		for _, w := range l.writers {
			fmt.Fprintln(w, string(data))
		}
	} else {
		parts := fmt.Sprintf("[%s] [%s] %s", humanTime, level.String(), msg)
		for k, v := range fields {
			parts += fmt.Sprintf(" [%s: %s]", k, v)
		}
		for _, w := range l.writers {
			fmt.Fprintln(w, parts)
		}
	}
}

func (l *Logger) Info(msg string, fields ...map[string]string) {
	var f map[string]string
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LevelInfo, msg, f)
}

func (l *Logger) Warn(msg string, fields ...map[string]string) {
	var f map[string]string
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LevelWarn, msg, f)
}

func (l *Logger) Error(msg string, fields ...map[string]string) {
	var f map[string]string
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LevelError, msg, f)
}
