package traefik

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRawLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		input string
		want  []Log
	}{
		{
			desc:  "empty input",
			input: "",
			want:  []Log{},
		},
		{
			desc:  "plain text log",
			input: "simple log message",
			want: []Log{
				{Message: "simple log message"},
			},
		},
		{
			desc:  "valid JSON log",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":"test message","error":"test error","custom_field":"value"}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "test message",
					Error:     "test error",
					Fields: map[string]interface{}{
						"custom_field": "value",
					},
				},
			},
		},
		{
			desc:  "invalid log level",
			input: `{"level":"invalid","time":"2023-01-01T00:00:00Z","message":"test message"}`,
			want: []Log{
				{Message: `{"level":"invalid","time":"2023-01-01T00:00:00Z","message":"test message"}`},
			},
		},
		{
			desc: "multiple logs",
			input: `
{"level":"info","time":"2023-01-01T00:00:00Z","message":"first message"}
{"level":"warning","time":"2023-01-01T00:00:00Z","message":"warning message"}
{"level":"error","time":"2023-01-01T00:00:01Z","message":"second message"}
invalid json
{"level":"debug","time":"2023-01-01T00:00:02Z","message":"third message"}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "first message",
					Fields:    map[string]interface{}{},
				},
				{
					Level:     LogLevelWarn,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "warning message",
					Fields:    map[string]interface{}{},
				},
				{
					Level:     LogLevelError,
					Timestamp: "2023-01-01T00:00:01Z",
					Message:   "second message",
					Fields:    map[string]interface{}{},
				},
				{
					Message: "invalid json",
				},
				{
					Level:     LogLevelDebug,
					Timestamp: "2023-01-01T00:00:02Z",
					Message:   "third message",
					Fields:    map[string]interface{}{},
				},
			},
		},
		{
			desc:  "log with additional fields",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":"test","field1":"value1","field2":42,"field3":true}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "test",
					Fields: map[string]interface{}{
						"field1": "value1",
						"field2": float64(42),
						"field3": true,
					},
				},
			},
		},
		{
			desc:  "malformed JSON",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":}`,
			want: []Log{
				{Message: `{"level":"info","time":"2023-01-01T00:00:00Z","message":}`},
			},
		},
		{
			desc:  "missing message field",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z"}`,
			want: []Log{
				{
					Timestamp: "2023-01-01T00:00:00Z",
					Level:     LogLevelInfo,
					Fields:    map[string]interface{}{},
				},
			},
		},
		{
			desc:  "missing time field",
			input: `{"level":"info","message":"test message"}`,
			want: []Log{
				{
					Level:   LogLevelInfo,
					Message: "test message",
					Fields:  map[string]interface{}{},
				},
			},
		},
		{
			desc:  "nested JSON fields",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":"test","metadata":{"user":"john","id":123}}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "test",
					Fields: map[string]interface{}{
						"metadata": map[string]interface{}{
							"user": "john",
							"id":   float64(123),
						},
					},
				},
			},
		},
		{
			desc:  "array fields",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":"test","tags":["tag1","tag2"]}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "test",
					Fields: map[string]interface{}{
						"tags": []interface{}{"tag1", "tag2"},
					},
				},
			},
		},
		{
			desc:  "null fields",
			input: `{"level":"info","time":"2023-01-01T00:00:00Z","message":"test","nullable":null}`,
			want: []Log{
				{
					Level:     LogLevelInfo,
					Timestamp: "2023-01-01T00:00:00Z",
					Message:   "test",
					Fields: map[string]interface{}{
						"nullable": nil,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			logs := ParseRawLogs(test.input)
			assert.Equal(t, test.want, logs)
		})
	}
}
