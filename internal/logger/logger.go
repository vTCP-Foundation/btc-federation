package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Config represents logger configuration
type Config struct {
	ConsoleOutput bool   `yaml:"console_output"`
	ConsoleColor  bool   `yaml:"console_color"`
	FileOutput    bool   `yaml:"file_output"`
	FileName      string `yaml:"file_name"`
	FileMaxSize   string `yaml:"file_max_size"`
	Level         string `yaml:"level"`
}

// Logger wraps zerolog functionality with isolated dependencies
type Logger struct {
	zlog   zerolog.Logger
	config Config
}

var globalLogger *Logger

// Init initializes the global logger with given configuration
func Init(config Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// New creates a new logger instance
func New(config Config) (*Logger, error) {
	// Parse log level
	level, err := parseLogLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create writers array
	var writers []io.Writer

	// Console writer
	if config.ConsoleOutput {
		var consoleWriter io.Writer = os.Stdout
		if config.ConsoleColor {
			consoleWriter = zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
				FormatLevel: func(i interface{}) string {
					level := strings.ToUpper(fmt.Sprintf("%s", i))
					switch level {
					case "DEBUG":
						return "\033[36mDEBUG\033[0m" // Cyan
					case "INFO":
						return "\033[32mINFO\033[0m" // Green
					case "WARN":
						return "\033[33mWARN\033[0m" // Yellow
					case "ERROR":
						return "\033[31mERROR\033[0m" // Red
					default:
						return level
					}
				},
			}
		}
		writers = append(writers, consoleWriter)
	}

	// File writer with rotation
	if config.FileOutput {
		if config.FileName == "" {
			return nil, fmt.Errorf("file_name is required when file_output is enabled")
		}

		maxSizeMB, err := parseMaxSize(config.FileMaxSize)
		if err != nil {
			return nil, fmt.Errorf("invalid file_max_size: %w", err)
		}

		// Get executable directory for log file placement
		execDir, err := getExecutableDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get executable directory: %w", err)
		}

		logFilePath := filepath.Join(execDir, config.FileName)

		fileWriter := &lumberjack.Logger{
			Filename: logFilePath,
			MaxSize:  maxSizeMB, // megabytes
			Compress: true,      // compress rotated files
		}
		writers = append(writers, fileWriter)
	}

	if len(writers) == 0 {
		// If no output is configured, default to console
		writers = append(writers, os.Stdout)
	}

	// Create multi writer
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// Create zerolog logger
	zlogger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Logger()

	return &Logger{
		zlog:   zlogger,
		config: config,
	}, nil
}

// parseLogLevel converts string to zerolog level
func parseLogLevel(levelStr string) (zerolog.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	default:
		return zerolog.InfoLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}

// parseMaxSize converts size string (e.g., "10MB") to megabytes
func parseMaxSize(sizeStr string) (int, error) {
	if sizeStr == "" {
		return 10, nil // default 10MB
	}

	sizeStr = strings.ToUpper(sizeStr)
	if strings.HasSuffix(sizeStr, "MB") {
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, fmt.Errorf("invalid size format: %s", sizeStr)
		}
		return size, nil
	}

	// Try to parse as plain number (assume MB)
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}
	return size, nil
}

// getExecutableDir returns the directory containing the executable
func getExecutableDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(execPath), nil
}

// Global logging functions that use the global logger

// Debug logs a debug message
func Debug(msg string, fields ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(msg, fields...)
	}
}

// Info logs an info message
func Info(msg string, fields ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(msg, fields...)
	}
}

// Warn logs a warning message
func Warn(msg string, fields ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(msg, fields...)
	}
}

// Error logs an error message
func Error(msg string, fields ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(msg, fields...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatal(msg, fields...)
	}
	os.Exit(1)
}

// Logger instance methods

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.zlog.Debug().Fields(fieldsToMap(fields...)).Msg(msg)
	} else {
		l.zlog.Debug().Msg(msg)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.zlog.Info().Fields(fieldsToMap(fields...)).Msg(msg)
	} else {
		l.zlog.Info().Msg(msg)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.zlog.Warn().Fields(fieldsToMap(fields...)).Msg(msg)
	} else {
		l.zlog.Warn().Msg(msg)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.zlog.Error().Fields(fieldsToMap(fields...)).Msg(msg)
	} else {
		l.zlog.Error().Msg(msg)
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		l.zlog.Fatal().Fields(fieldsToMap(fields...)).Msg(msg)
	} else {
		l.zlog.Fatal().Msg(msg)
	}
}

// fieldsToMap converts variadic fields to map for zerolog
func fieldsToMap(fields ...interface{}) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}

	fieldMap := make(map[string]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			if key, ok := fields[i].(string); ok {
				fieldMap[key] = fields[i+1]
			}
		}
	}
	return fieldMap
}
