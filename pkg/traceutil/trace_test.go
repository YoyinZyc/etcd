package traceutil

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGet(t *testing.T) {
	traceForTest := &Trace{operation: "test"}
	tests := []struct {
		name        string
		inputCtx    context.Context
		outputTrace *Trace
	}{
		{
			name:        "When the context does not have trace",
			inputCtx:    context.TODO(),
			outputTrace: nil,
		},
		{
			name:        "When the context has trace",
			inputCtx:    context.WithValue(context.Background(), "trace", traceForTest),
			outputTrace: traceForTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := Get(tt.inputCtx)
			if trace != tt.outputTrace {
				t.Errorf("Expected %v Got %v", tt.outputTrace, trace)
			}
		})
	}
}

func TestGetOrCreate(t *testing.T) {
	tests := []struct {
		name          string
		inputCtx      context.Context
		outputTraceOp string
	}{
		{
			name:          "When the context does not have trace",
			inputCtx:      context.TODO(),
			outputTraceOp: "test",
		},
		{
			name:          "When the context has trace",
			inputCtx:      context.WithValue(context.Background(), "trace", &Trace{operation: "test"}),
			outputTraceOp: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, trace := GetOrCreate(tt.inputCtx, "test")
			if trace == nil {
				t.Errorf("Expected trace object Got nil")
			} else if trace.operation != tt.outputTraceOp {
				t.Errorf("Expected %v Got %v", tt.outputTraceOp, trace.operation)
			}
			if ctx.Value("trace") == nil {
				t.Errorf("Expected context has attached trace Got nil")
			}
		})
	}
}

func TestStep(t *testing.T) {
	var (
		op    = "Test"
		steps = []string{"Step1, Step2"}
	)

	trace := New(op)
	if trace.operation != op {
		t.Errorf("Expected %v, got %v\n", op, trace.operation)
	}

	for _, v := range steps {
		trace.Step(v)
		trace.Step(v)
	}

	for i, v := range steps {
		if v != trace.steps[i].msg {
			t.Errorf("Expected %v, got %v\n.", v, trace.steps[i].msg)
		}
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name        string
		trace       *Trace
		expectedMsg []string
	}{
		{
			name: "When dump all logs",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time.Now().Add(-80 * time.Millisecond), "msg1"},
					{time.Now().Add(-50 * time.Millisecond), "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1", "msg2",
			},
		},
		{
			name: "Check formatting",
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time.Now(), "msg1"},
				},
			},
			expectedMsg: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
			defer os.RemoveAll(logPath)

			lcfg := zap.NewProductionConfig()
			lcfg.OutputPaths = []string{logPath}
			lcfg.ErrorOutputPaths = []string{logPath}
			lg, _ := lcfg.Build()

			tt.trace.Log(lg)
			data, err := ioutil.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(tt.expectedMsg) > 0 {
				for _, msg := range tt.expectedMsg {
					if !bytes.Contains(data, []byte(msg)) {
						t.Errorf("Expected %v, Got nothing", msg)
					}
				}
			} else {
				pattern := `(.+)Trace\[(\d*)?\](.+)\(duration(.+)start(.+)\)\\n` +
					`Trace\[(\d*)?\](.+)Step(.+)\(duration(.+)\)\\n` +
					`Trace\[(\d*)?\](.+)End(.+)\\n(.+)`
				r, _ := regexp.Compile(pattern)
				if !r.MatchString(string(data)) {
					t.Errorf("Wrong log format.")
				}
			}

		})
	}
}

func TestLogIfLong(t *testing.T) {

	tests := []struct {
		name        string
		threshold   time.Duration
		trace       *Trace
		expectedMsg []string
	}{
		{
			name:      "When the duration is smaller than threshold",
			threshold: time.Duration(200 * time.Millisecond),
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time.Now().Add(-50 * time.Millisecond), "msg1"},
					{time.Now(), "msg2"},
				},
			},
			expectedMsg: []string{},
		},
		{
			name:      "When the duration is longer than threshold",
			threshold: time.Duration(50 * time.Millisecond),
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time.Now().Add(-50 * time.Millisecond), "msg1"},
					{time.Now(), "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1", "msg2",
			},
		},
		{
			name:      "When not all steps are longer than step threshold",
			threshold: time.Duration(50 * time.Millisecond),
			trace: &Trace{
				operation: "Test",
				startTime: time.Now().Add(-100 * time.Millisecond),
				steps: []step{
					{time.Now(), "msg1"},
					{time.Now(), "msg2"},
				},
			},
			expectedMsg: []string{
				"msg1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
			defer os.RemoveAll(logPath)

			lcfg := zap.NewProductionConfig()
			lcfg.OutputPaths = []string{logPath}
			lcfg.ErrorOutputPaths = []string{logPath}
			lg, _ := lcfg.Build()

			tt.trace.LogIfLong(tt.threshold, lg)
			data, err := ioutil.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, msg := range tt.expectedMsg {
				if !bytes.Contains(data, []byte(msg)) {
					t.Errorf("Expected %v, Got nothing", msg)
				}
			}
		})
	}
}
