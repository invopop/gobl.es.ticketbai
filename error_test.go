package ticketbai

import (
	"errors"
	"fmt"
	"testing"

	"github.com/invopop/gobl.ticketbai/internal/gateways"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewErrorFrom(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, newErrorFrom(nil))
	})

	t.Run("gateway error", func(t *testing.T) {
		err := newErrorFrom(gateways.ErrValidation)
		assert.ErrorIs(t, err, ErrValidation)
		assert.Equal(t, "validation", err.Key())
		assert.Same(t, gateways.ErrValidation, err.Cause())
	})

	t.Run("wrapped gateway error keeps the key", func(t *testing.T) {
		src := fmt.Errorf("sending request: %w", gateways.ErrServer)
		err := newErrorFrom(src)
		assert.ErrorIs(t, err, ErrServer)
		assert.Equal(t, "server", err.Key())
		require.ErrorIs(t, err, gateways.ErrServer)
		assert.Same(t, src, err.Cause())
	})

	t.Run("wrapped local error", func(t *testing.T) {
		src := ErrValidation.withMessage("missing taxes")
		err := newErrorFrom(fmt.Errorf("preparing: %w", src))
		assert.Same(t, src, err)
	})

	t.Run("unknown error", func(t *testing.T) {
		src := errors.New("boom")
		err := newErrorFrom(src)
		assert.ErrorIs(t, err, ErrInternal)
		assert.Empty(t, err.Code())
		assert.Equal(t, "boom", err.Message())
	})
}
