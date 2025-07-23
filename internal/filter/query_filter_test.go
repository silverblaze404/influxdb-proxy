package filter

import (
	"testing"

	"gm-influxdb-proxy/internal/config"
)

func TestQueryFilter_ValidateQuery(t *testing.T) {
	rules := config.FilteringRules{
		RequireTimeFilter:     true,
		MaxTimeRangeHours:     24,
		BlockWildcardSelect:   true,
		BlockUnlimitedGroupBy: true,
		BlockExpensiveShows:   true,
		MaxShowSeriesLimit:    100,
		BlockedFunctions:      []string{"count(*)"},
		AllowedStatements:     []string{"SELECT", "SHOW DATABASES", "SHOW MEASUREMENTS", "SHOW TAG KEYS", "SHOW TAG VALUES", "SHOW FIELD KEYS", "SHOW SERIES"},
	}
	filter := NewQueryFilter(rules)

	tests := []struct {
		name     string
		query    string
		expected bool
		reason   string
	}{
		{
			name:     "Valid query with time filter",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > '2023-01-01T00:00:00Z' AND time < '2023-01-02T00:00:00Z'",
			expected: true,
		},
		{
			name:     "Query without time filter",
			query:    "SELECT value FROM mydb..mymeasurement",
			expected: false,
			reason:   "Query must include a time filter",
		},
		{
			name:     "Query with blocked function",
			query:    "SELECT count(*) FROM mydb..mymeasurement WHERE time > '2023-01-01T00:00:00Z' AND time < '2023-01-02T00:00:00Z'",
			expected: false,
			reason:   "Query contains blocked function: count(*)",
		},
		{
			name:     "Valid SHOW DATABASES",
			query:    "SHOW DATABASES",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ValidateQuery(tt.query)
			if result.Allowed != tt.expected {
				t.Errorf("ValidateQuery() = %v, expected %v", result.Allowed, tt.expected)
			}
			if !tt.expected && tt.reason != "" {
				if result.Reason == "" || result.Reason != tt.reason {
					t.Errorf("Expected reason to contain '%s', got '%s'", tt.reason, result.Reason)
				}
			}
		})
	}
}

func TestQueryFilter_TimeRange(t *testing.T) {
	rules := config.FilteringRules{
		RequireTimeFilter: true,
		MaxTimeRangeHours: 1, // 1 hour max
	}
	filter := NewQueryFilter(rules)

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "Time range within limit",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > '2023-01-01T00:00:00Z' AND time < '2023-01-01T00:30:00Z'",
			expected: true,
		},
		{
			name:     "Time range within limit 2",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > now() - 59m limit 1",
			expected: true,
		},
		{
			name:     "Time range within limit 3",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > now() - 62m and xd=1 and time < now() - 3m limit 1",
			expected: true,
		},
		{
			name:     "Time range exceeds limit",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > '2023-01-01T00:00:00Z' AND time < '2023-03-02T00:00:00Z'",
			expected: false,
		},
		{
			name:     "Time range exceeds limit 2",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > now() - 30d limit 1",
			expected: false,
		},
		{
			name:     "Time range exceeds limit 2_2",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > now() - 30d AND time < now() - 10d LIMIT 1",
			expected: false,
		},
		{
			name:     "Time range exceeds limit 3",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > 1750614600000000000 LIMIT 1",
			expected: false,
		},
		{
			name:     "Time range exceeds limit 3",
			query:    "SELECT value FROM mydb..mymeasurement WHERE time > 1750614600000000000 LIMIT 1",
			expected: false,
		},
		{
			name:     "Time range exceeds limit 4",
			query:    "SELECT value FROM mymeasurement WHERE time > 1750617000000000000 AND time < 1751826600000000000 LIMIT 1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ValidateQuery(tt.query)
			if result.Allowed != tt.expected {
				t.Errorf("ValidateQuery() = %v, expected %v. Reason: %s", result.Allowed, tt.expected, result.Reason)
			}
		})
	}
}
