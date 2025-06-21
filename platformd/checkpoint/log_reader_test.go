package checkpoint

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
)

type mockRemoteCmdExecutor struct {
}

func (e *mockRemoteCmdExecutor) Stream(_ remotecommand.StreamOptions) error {
	panic("implement me")
}

func (e *mockRemoteCmdExecutor) StreamWithContext(ctx context.Context, opts remotecommand.StreamOptions) error {
	t := time.NewTicker(1 * time.Second)
	counter := 0
	for {
		select {
		case <-t.C:
			if counter == 3 {
				_, _ = opts.Stdout.Write([]byte(`Done (30.0s)! For help, type "help"`))
			}
			_, _ = opts.Stdout.Write([]byte(fmt.Sprintf("%d", counter)))
			counter++
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestLogReader(t *testing.T) {
	tests := []struct {
		name     string
		deadline time.Duration
		err      error
	}{
		{
			name:     "works",
			deadline: 10 * time.Second,
		},
		{
			name:     "timeout reached",
			deadline: 1 * time.Second,
			err:      context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newLogReader(&mockRemoteCmdExecutor{})

			ctx, cancel := context.WithTimeout(context.Background(), tt.deadline)
			defer cancel()

			err := r.WaitForRegex(ctx, paperServerReadyRegex)

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}

			require.NoError(t, err)
		})
	}
}
