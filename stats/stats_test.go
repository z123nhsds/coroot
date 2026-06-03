package stats

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterMCPCallIsCollectedAndReset(t *testing.T) {
	database, err := db.NewSqlite(t.TempDir())
	require.NoError(t, err)
	defer database.DB().Close()
	require.NoError(t, database.Migrate())

	collector := NewCollector(true, "instance-1", "2.7.1", "ce", database, nil, nil, nil)
	collector.RegisterMCPCall("query_logs")
	collector.RegisterMCPCall("query_logs")
	collector.RegisterMCPCall("list_alerts")

	stats := collector.collect()
	assert.Equal(t, map[string]int{"list_alerts": 1, "query_logs": 2}, stats.UX.McpCalls)

	next := collector.collect()
	assert.Empty(t, next.UX.McpCalls)
}
