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
	l := logger{gormLevel: gormlogger.Info}
	actual := l.LogMode(gormlogger.Info)

	assert.Equal(t, l, actual)
}

func Test_logger(t *testing.T) {
	receiver, gormLogger := getReceiverAndLogger([]Option{WithContextValue("attrKey", "ctxKey")})
	expectedMsg := "awesome message"

	tests := []struct {
		name           string
		ctx            context.Context
		function       func(context.Context, string, ...any)
		wantMsg        string
		wantAttributes map[string]slog.Attr
		wantLevel      slog.Level
	}{
		{
			name:      "Info",
			ctx:       context.Background(),
			function:  gormLogger.Info,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelInfo,
		},
		{
			name:      "Warn",
			ctx:       context.Background(),
			function:  gormLogger.Warn,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelWarn,
		},
		{
			name:      "Error",
			ctx:       context.Background(),
			function:  gormLogger.Error,
			wantMsg:   expectedMsg,
			wantLevel: slog.LevelError,
		},
		{
			name:           "Error",
			ctx:            context.WithValue(context.Background(), "ctxKey", "ctxVal"),
			function:       gormLogger.Error,
			wantMsg:        expectedMsg,
			wantAttributes: map[string]slog.Attr{"attrKey": slog.Any("attrKey", "ctxVal")},
			wantLevel:      slog.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver.Reset()

			// Act
			tt.function(tt.ctx, tt.wantMsg)

			// Assert
			require.NotNil(t, receiver.Record)
			assert.Equal(t, tt.wantMsg, receiver.Record.Message)
			assert.Equal(t, tt.wantLevel, receiver.Record.Level)

			if tt.wantAttributes != nil {
				for _, v := range tt.wantAttributes {
					found := false
					receiver.Record.Attrs(func(attr slog.Attr) bool {
						if attr.Equal(v) {
							found = true
							return false
						}
						return true
					})
					assert.True(t, found, "expected attribute %v not found", v.Key)
				}
			}
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
		ctx     context.Context

		wantNoRecord       bool
		wantContainMessage string
		wantAttributes     map[string]slog.Attr
		wantLevel          slog.Level
	}{
		{
			name: "With trace all mode",
			options: []Option{
				WithTraceAll(),
			},
			args:               selectQueryArgs,
			ctx:                context.Background(),
			wantContainMessage: "SQL query executed",
			wantLevel:          slog.LevelInfo,
		},
		{
			name: "With trace all mode and custom log level",
			options: []Option{
				WithTraceAll(),
				SetLogLevel(DefaultLogType, customLogLevel),
			},
			ctx:                context.Background(),
			args:               selectQueryArgs,
			wantContainMessage: "SQL query executed",
			wantLevel:          customLogLevel,
		},
		{
			name:         "Without trace all mode",
			args:         selectQueryArgs,
			ctx:          context.Background(),
			wantNoRecord: true,
		},
		{
			name: "With trace all mode but ignoreTrace option is enabled",
			options: []Option{
				WithTraceAll(),
				WithIgnoreTrace(),
			},
			args:         selectQueryArgs,
			ctx:          context.Background(),
			wantNoRecord: true,
		},
		{
			name: "Slow query",
			options: []Option{
				WithSlowThreshold(1 * time.Second),
			},
			args:               selectQueryArgs,
			ctx:                context.Background(),
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
			ctx:                context.Background(),
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
			ctx:          context.Background(),
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
			ctx:                context.Background(),
			wantContainMessage: errorQueryArgs.err.Error(),
			wantLevel:          customLogLevel,
		},
		{
			name: "Error but ignoreTrace option is enabled",
			options: []Option{
				WithIgnoreTrace(),
			},
			args:         errorQueryArgs,
			ctx:          context.Background(),
			wantNoRecord: true,
		},
		{
			name: "Not found error",
			options: []Option{
				WithRecordNotFoundError(),
			},
			args:               notFoundErrorQueryArgs,
			ctx:                context.Background(),
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
			ctx:          context.Background(),
			wantNoRecord: true,
		},
		{
			name:         "Not found error is ignored",
			args:         notFoundErrorQueryArgs,
			ctx:          context.Background(),
			wantNoRecord: true,
		},
		{
			name: "With context value",
			options: []Option{
				WithTraceAll(),
				WithContextValue("attrKey", "ctxKey"),
			},
			args:               selectQueryArgs,
			ctx:                context.WithValue(context.Background(), "ctxKey", "ctxVal"),
			wantContainMessage: "SQL query executed",
			wantAttributes:     map[string]slog.Attr{"attrKey": slog.Any("attrKey", "ctxVal")},
			wantLevel:          slog.LevelInfo,
		},
		{
			name: "With error and context value",
			options: []Option{
				WithContextValue("attrKey", "ctxKey"),
			},
			args:               errorQueryArgs,
			ctx:                context.WithValue(context.Background(), "ctxKey", "ctxVal"),
			wantContainMessage: errorQueryArgs.err.Error(),
			wantAttributes:     map[string]slog.Attr{"attrKey": slog.Any("attrKey", "ctxVal")},
			wantLevel:          slog.LevelError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver, gormLogger := getReceiverAndLogger(tt.options)

			// Act
			gormLogger.Trace(tt.ctx, tt.args.begin, tt.args.fc, tt.args.err)

			// Assert
			if tt.wantNoRecord {
				assert.Nil(t, receiver.Record)
			} else {
				require.NotNil(t, receiver.Record)
				assert.Equal(t, tt.wantLevel, receiver.Record.Level)
				assert.Contains(t, receiver.Record.Message, tt.wantContainMessage)
				if tt.wantAttributes != nil {
					for k, v := range tt.wantAttributes {
						found := false
						receiver.Record.Attrs(func(attr slog.Attr) bool {
							if attr.Key == k && attr.Equal(v) {
								found = true
								return false
							}
							return true
						})
						assert.True(t, found, "expected attribute %v not found", v.Key)
					}

				}
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
