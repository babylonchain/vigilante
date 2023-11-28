package log

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func init() {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}

	Logger = zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		os.Stderr,
		zap.DebugLevel,
	))
}
