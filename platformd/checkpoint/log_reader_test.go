package checkpoint

import (
	"context"
	"testing"
	"time"

	"github.com/spacechunks/explorer/test"
	"github.com/stretchr/testify/require"
)

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
			r := newLogReader(&test.RemoteCmdExecutor{})

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
