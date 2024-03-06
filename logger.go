package slogGorm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type LogType string

const (
	ErrorLogType     LogType = "sql_error"
	SlowQueryLogType LogType = "slow_query"
	DefaultLogType   LogType = "default"

	SourceField    = "file"
	ErrorField     = "error"
	QueryField     = "query"
	DurationField  = "duration"
	SlowQueryField = "slow_query"
	RowsField      = "rows"
)

// New creates a new logger for gorm.io/gorm
func New(options ...Option) *logger {
	l := logger{
		ignoreRecordNotFoundError: true,
		errorField:                ErrorField,
		sourceField:               SourceField,

		// log levels
		logLevel: map[LogType]slog.Level{
			ErrorLogType:     slog.LevelError,
			SlowQueryLogType: slog.LevelWarn,
			DefaultLogType:   slog.LevelInfo,
		},
	}

	// Apply options
	for _, option := range options {
		option(&l)
	}

	if l.sloggerHandler == nil {
		// If no sloggerHandler is defined, use the default Handler
		l.sloggerHandler = slog.Default().Handler()
	}

	return &l
}

type logger struct {
	sloggerHandler            slog.Handler
	ignoreTrace               bool
	ignoreRecordNotFoundError bool
	traceAll                  bool
	slowThreshold             time.Duration
	logLevel                  map[LogType]slog.Level
	contextKeys               map[string]string

	sourceField string
	errorField  string
}

// LogMode log mode
func (l logger) LogMode(_ gormlogger.LogLevel) gormlogger.Interface {
	// log level is set by slog
	return l
}

// Info logs info
func (l logger) Info(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelInfo, format, args...)
}

// Warn logs warn messages
func (l logger) Warn(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelWarn, format, args...)
}

// Error logs error messages
func (l logger) Error(ctx context.Context, format string, args ...any) {
	l.log(ctx, slog.LevelError, format, args...)
}

// log adds context attributes and logs a message with the given slog level
func (l logger) log(ctx context.Context, level slog.Level, format string, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !l.sloggerHandler.Enabled(ctx, level) {
		return
	}

	// Properly handle the PC for the caller
	var pc uintptr
	var pcs [1]uintptr
	// skip [runtime.Callers, this function, this function's caller]
	runtime.Callers(3, pcs[:])
	pc = pcs[0]
	r := slog.NewRecord(time.Now(), level, fmt.Sprintf(format, args...), pc)
	r.Add(l.appendContextAttributes(ctx, nil)...)

	_ = l.sloggerHandler.Handle(ctx, r)
}

// Trace logs sql message
func (l logger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.ignoreTrace {
		return // Silent
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFoundError):
		sql, rows := fc()

		// Append context attributes
		attributes := l.appendContextAttributes(ctx, []any{
			slog.Any(l.errorField, err),
			slog.String(QueryField, sql),
			slog.Duration(DurationField, elapsed),
			slog.Int64(RowsField, rows),
			slog.String(l.sourceField, utils.FileWithLineNum()),
		})

		l.log(ctx, l.logLevel[ErrorLogType], err.Error(), attributes...)

	case l.slowThreshold != 0 && elapsed > l.slowThreshold:
		sql, rows := fc()

		// Append context attributes
		attributes := l.appendContextAttributes(ctx, []any{
			slog.Bool(SlowQueryField, true),
			slog.String(QueryField, sql),
			slog.Duration(DurationField, elapsed),
			slog.Int64(RowsField, rows),
			slog.String(l.sourceField, utils.FileWithLineNum()),
		})
		l.log(ctx, l.logLevel[SlowQueryLogType], fmt.Sprintf("slow sql query [%s >= %v]", elapsed, l.slowThreshold), attributes...)

	case l.traceAll:
		sql, rows := fc()

		// Append context attributes
		attributes := l.appendContextAttributes(ctx, []any{
			slog.String(QueryField, sql),
			slog.Duration(DurationField, elapsed),
			slog.Int64(RowsField, rows),
			slog.String(l.sourceField, utils.FileWithLineNum()),
		})

		l.log(ctx, l.logLevel[DefaultLogType], fmt.Sprintf("SQL query executed [%s]", elapsed), attributes...)
	}
}

func (l logger) appendContextAttributes(ctx context.Context, args []any) []any {
	if args == nil {
		args = []any{}
	}
	for k, v := range l.contextKeys {
		if value := ctx.Value(v); value != nil {
			args = append(args, slog.Any(k, value))
		}
	}
	return args
}
