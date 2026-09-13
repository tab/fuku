package doctor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_environmentSection(t *testing.T) {
	section := environmentSection(context.Background(), &Env{})

	assert.Equal(t, "Environment", section.Title)
	require.Len(t, section.Results, 3)
	assert.Equal(t, CheckSystem, section.Results[0].ID)
	assert.Equal(t, CheckRuntime, section.Results[1].ID)
	assert.Equal(t, CheckInstall, section.Results[2].ID)
}
