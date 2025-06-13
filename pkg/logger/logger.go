package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logger.Logger is the interface for logging
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Fatal(msg string, keysAndValues ...interface{})
	With(keysAndValues ...interface{}) Logger
}

// ZapLogger implements the logger.Logger interface using zap
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewLogger creates a new logger
func NewLogger() *ZapLogger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	config := zap.NewProductionConfig()
	config.EncoderConfig = encoderConfig

	logger, _ := config.Build()
	sugar := logger.Sugar()

	return &ZapLogger{
		logger: sugar,
	}
}

// Debug logs a debug message
func (l *ZapLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debugw(msg, keysAndValues...)
}

// Info logs an info message
func (l *ZapLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Infow(msg, keysAndValues...)
}

// Warn logs a warning message
func (l *ZapLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warnw(msg, keysAndValues...)
}

// Error logs an error message
func (l *ZapLogger) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Errorw(msg, keysAndValues...)
}

// Fatal logs a fatal message and exits
func (l *ZapLogger) Fatal(msg string, keysAndValues ...interface{}) {
	l.logger.Fatalw(msg, keysAndValues...)
}

// With returns a logger with additional structured context
func (l *ZapLogger) With(keysAndValues ...interface{}) Logger {
	return &ZapLogger{
		logger: l.logger.With(keysAndValues...),
	}
}
