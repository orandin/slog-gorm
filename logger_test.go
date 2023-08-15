package slogGorm

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestNew(t *testing.T) {
	t.Run("Without options", func(t *testing.T) {
		l := New()

		require.NotNil(t, l.slogger)
		assert.Equal(t, slog.Default(), l.slogger)
	})

	t.Run("WithLogger(nil)", func(t *testing.T) {
		l := New(
			WithLogger(nil),
		)

		require.NotNil(t, l.slogger)
		assert.Equal(t, slog.Default(), l.slogger)
	})
}

func Test_logger_LogMode(t *testing.T) {
	l := logger{}
	actual := l.LogMode(gormlogger.Info)

	assert.Equal(t, l, actual)
}

func Test_logger(t *testing.T) {
	receiver, gormLogger := getReceiverAndLogger(nil)
	expectedMsg := "awesome message"

	tests := []struct {
		name      string
		function  func(context.Context, string, ...any)
		wantMsg   string
		wantLevel slog.Level
	}{
		{
			name:      "Info",
			function:  gormLogger.Info,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "Warn",
			function:  gormLogger.Warn,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "Error",
			function:  gormLogger.Error,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver.Reset()

			// Act
			tt.function(context.Background(), tt.wantMsg)

			// Assert
			require.NotNil(t, receiver.Record)
			assert.Equal(t, tt.wantMsg, receiver.Record.Message)
			assert.Equal(t, tt.wantLevel, receiver.Record.Level)
		})
	}
}

func Test_logger_Trace(t *testing.T) {
	customLogLevel := slog.Level(42)

	type args struct {
		begin time.Time
		fc    func() (sql string, rowsAffected int64)
		err   error
	}

	selectQueryArgs := args{
		begin: time.Now().Add(-1 * time.Minute),
		err:   nil,
		fc: func() (string, int64) {
			return "SELECT * FROM user", 1
		},
	}

	errorQueryArgs := args{
		begin: time.Now().Add(-1 * time.Minute),
		err:   fmt.Errorf("awesome error"),
		fc: func() (string, int64) {
			return "SELECT * FROM user", 1
		},
	}

	notFoundErrorQueryArgs := args{
		begin: time.Now().Add(-1 * time.Minute),
		err:   gorm.ErrRecordNotFound,
		fc: func() (string, int64) {
			return "SELECT * FROM user", 1
		},
	}

	tests := []struct {
		name    string
		args    args
		options []Option

		wantNoRecord       bool
		wantContainMessage string
		wantLevel          slog.Level
	}{
		{
			name: "With trace all mode",
			options: []Option{
				WithTraceAll(),
			},
			args:               selectQueryArgs,
			wantContainMessage: "SQL query executed",
			wantLevel:          slog.LevelInfo,
		},
		{
			name: "With trace all mode and custom log level",
			options: []Option{
				WithTraceAll(),
				SetLogLevel(DefaultLogType, customLogLevel),
			},
			args:               selectQueryArgs,
			wantContainMessage: "SQL query executed",
			wantLevel:          customLogLevel,
		},
		{
			name:         "Without trace all mode",
			args:         selectQueryArgs,
			wantNoRecord: true,
		},
		{
			name: "With trace all mode but ignoreTrace option is enabled",
			options: []Option{
				WithTraceAll(),
				WithIgnoreTrace(),
			},
			args:         selectQueryArgs,
			wantNoRecord: true,
		},
		{
			name: "Slow query",
			options: []Option{
				WithSlowThreshold(1 * time.Second),
			},
			args:               selectQueryArgs,
			wantContainMessage: "slow sql query",
			wantLevel:          slog.LevelWarn,
		},
		{
			name: "Slow query and custom log level",
			options: []Option{
				WithSlowThreshold(1 * time.Second),
				SetLogLevel(SlowQueryLogType, customLogLevel),
			},
			args:               selectQueryArgs,
			wantContainMessage: "slow sql query",
			wantLevel:          customLogLevel,
		},
		{
			name: "Slow query but ignoreTrace option is enabled",
			options: []Option{
				WithSlowThreshold(1 * time.Second),
				WithIgnoreTrace(),
			},
			args:         selectQueryArgs,
			wantNoRecord: true,
		},
		{
			name:               "Error",
			args:               errorQueryArgs,
			wantContainMessage: errorQueryArgs.err.Error(),
			wantLevel:          slog.LevelError,
		},
		{
			name: "Error and custom log level",
			options: []Option{
				SetLogLevel(ErrorLogType, customLogLevel),
			},
			args:               errorQueryArgs,
			wantContainMessage: errorQueryArgs.err.Error(),
			wantLevel:          customLogLevel,
		},
		{
			name: "Error but ignoreTrace option is enabled",
			options: []Option{
				WithIgnoreTrace(),
			},
			args:         errorQueryArgs,
			wantNoRecord: true,
		},
		{
			name: "Not found error",
			options: []Option{
				WithRecordNotFoundError(),
			},
			args:               notFoundErrorQueryArgs,
			wantContainMessage: notFoundErrorQueryArgs.err.Error(),
			wantLevel:          slog.LevelError,
		},
		{
			name: "Not found error but ignoreTrace option is enabled",
			options: []Option{
				WithRecordNotFoundError(),
				WithIgnoreTrace(),
			},
			args:         notFoundErrorQueryArgs,
			wantNoRecord: true,
		},
		{
			name:         "Not found error is ignored",
			args:         notFoundErrorQueryArgs,
			wantNoRecord: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, gormLogger := getReceiverAndLogger(tt.options)

			// Act
			gormLogger.Trace(context.Background(), tt.args.begin, tt.args.fc, tt.args.err)

			// Assert
			if tt.wantNoRecord {
				assert.Nil(t, receiver.Record)
			} else {
				require.NotNil(t, receiver.Record)
				assert.Equal(t, tt.wantLevel, receiver.Record.Level)
				assert.Contains(t, receiver.Record.Message, tt.wantContainMessage)
			}
		})
	}
}

// private functions

func getReceiverAndLogger(options []Option) (*DummyHandler, *logger) {
	receiver := NewDummyHandler()
	options = append(options, WithLogger(slog.New(receiver)))

	return receiver, New(options...)
}

// Mock

func NewDummyHandler() *DummyHandler {
	dh := DummyHandler{}
	dh.Reset()

	return &dh
}

type DummyHandler struct {
	EnabledResponse map[slog.Level]bool
	Attrs           []slog.Attr
	Record          *slog.Record
}

func (h *DummyHandler) Reset() {
	h.Record = nil
	h.Attrs = []slog.Attr{}
}

func (h *DummyHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *DummyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.Attrs = append(h.Attrs, attrs...)
	return h
}

func (h *DummyHandler) WithGroup(_ string) slog.Handler {
	return h // not used
}

func (h *DummyHandler) Handle(_ context.Context, r slog.Record) error {
	h.Record = &r
	return nil
}
