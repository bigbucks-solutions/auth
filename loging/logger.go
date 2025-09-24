package loging

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Logger     *zap.SugaredLogger
	ZapWrapper *fwdToZapWriter
)

type fwdToZapWriter struct {
	logger *zap.SugaredLogger
}

func (fw *fwdToZapWriter) Write(p []byte) (n int, err error) {
	fw.logger.Infow(string(p))
	return len(p), nil
}

// Config holds logger configuration
type Config struct {
	Level string `mapstructure:"level" json:"level"`
}

// parseLogLevel converts string level to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	case "panic":
		return zap.PanicLevel
	default:
		return zap.InfoLevel // default to info level
	}
}

// Initialize initializes the logger with the provided config
func Initialize(config Config) {
	pe := zap.NewDevelopmentConfig()
	pe.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	pe.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(pe.EncoderConfig)

	level := parseLogLevel(config.Level)

	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)

	l := zap.New(core)
	Logger = l.Sugar()
	ZapWrapper = &fwdToZapWriter{
		logger: Logger,
	}
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// Default initialization for backward compatibility
func init() {
	// Initialize with default config if not explicitly configured
	Initialize(Config{Level: "debug"})
}

// InterceptorLogger returns a gRPC logging interceptor using the provided zap.Logger
func InterceptorLogger(l *zap.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make([]zap.Field, 0, len(fields)/2)

		for i := 0; i < len(fields); i += 2 {
			key := fields[i]
			value := fields[i+1]

			switch v := value.(type) {
			case string:
				f = append(f, zap.String(key.(string), v))
			case int:
				f = append(f, zap.Int(key.(string), v))
			case bool:
				f = append(f, zap.Bool(key.(string), v))
			default:
				f = append(f, zap.Any(key.(string), v))
			}
		}

		logger := l.WithOptions(zap.AddCallerSkip(1)).With(f...)

		switch lvl {
		case logging.LevelDebug:
			logger.Debug(msg)
		case logging.LevelInfo:
			logger.Info(msg)
		case logging.LevelWarn:
			logger.Warn(msg)
		case logging.LevelError:
			logger.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}
