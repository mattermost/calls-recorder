package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunCmd(t *testing.T) {
	t.Run("non-existant command", func(t *testing.T) {
		cmd, err := runCmd("calls", "")
		require.Error(t, err)
		require.Nil(t, cmd)
	})

	t.Run("valid command", func(t *testing.T) {
		cmd, err := runCmd("ls", ".")
		require.NoError(t, err)
		require.NotNil(t, cmd)
		require.NoError(t, cmd.Wait())
	})
}
