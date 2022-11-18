package loging

import (
	"os"

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

func init() {
	pe := zap.NewDevelopmentConfig()
	pe.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	pe.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(pe.EncoderConfig)

	level := zap.DebugLevel

	core :=
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)

	l := zap.New(core)
	Logger = l.Sugar()
	ZapWrapper = &fwdToZapWriter{
		logger: Logger,
	}
	defer Logger.Sync()
}
