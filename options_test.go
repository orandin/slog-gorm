package slogGorm

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithTraceAll(t *testing.T) {
	actual := &logger{}

	WithTraceAll()(actual)

	assert.True(t, actual.traceAll)
}

func TestWithErrorField(t *testing.T) {
	actual := &logger{}
	expected := "error"

	WithErrorField(expected)(actual)

	assert.Equal(t, expected, actual.errorField)
}

func TestWithIgnoreTrace(t *testing.T) {
	actual := &logger{}

	WithIgnoreTrace()(actual)

	assert.True(t, actual.ignoreTrace)
}

func TestWithLogger(t *testing.T) {
	actual := &logger{}
	log := slog.Default()

	WithLogger(log)(actual)

	assert.Equal(t, log, actual.slogger)
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		lType LogType
		level slog.Level
	}{
		{lType: ErrorLogType, level: slog.Level(42)},
		{lType: SlowQueryLogType, level: slog.Level(32)},
		{lType: DefaultLogType, level: slog.Level(22)},
	}

	for _, tt := range tests {
		t.Run(string(tt.lType), func(t *testing.T) {
			actual := &logger{
				logLevel: map[LogType]slog.Level{
					tt.lType: slog.LevelInfo,
				},
			}

			SetLogLevel(tt.lType, tt.level)(actual)

			assert.Equal(t, tt.level, actual.logLevel[tt.lType])
		})
	}
}

func TestWithRecordNotFoundError(t *testing.T) {
	actual := &logger{
		ignoreRecordNotFoundError: true,
	}

	WithRecordNotFoundError()(actual)

	assert.False(t, actual.ignoreRecordNotFoundError)
}

func TestWithSlowThreshold(t *testing.T) {
	actual := &logger{}
	expected := 1 * time.Second

	WithSlowThreshold(expected)(actual)

	assert.Equal(t, expected, actual.slowThreshold)
}

func TestWithSourceField(t *testing.T) {
	actual := &logger{}
	expected := "source"

	WithSourceField(expected)(actual)

	assert.Equal(t, expected, actual.sourceField)
}
