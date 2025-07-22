package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/influxdata/influxql"
	log "github.com/sirupsen/logrus"

	"gm-influxdb-proxy/internal/config"
)

// QueryFilter validates and filters InfluxDB queries
type QueryFilter struct {
	rules           config.FilteringRules
	blockedPatterns []*regexp.Regexp
}

// FilterResult represents the result of query filtering
type FilterResult struct {
	Allowed bool
	Reason  string
	Query   string
}

// NewQueryFilter creates a new query filter with the given configuration
func NewQueryFilter(rules config.FilteringRules) *QueryFilter {
	var patterns []*regexp.Regexp

	// Compile regex patterns for blocked functions
	for _, fn := range rules.BlockedFunctions {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(fn) + `\b`)
		patterns = append(patterns, pattern)
	}

	return &QueryFilter{
		rules:           rules,
		blockedPatterns: patterns,
	}
}

// ValidateQuery validates a query string and returns whether it should be allowed
func (qf *QueryFilter) ValidateQuery(queryString string) FilterResult {
	log.WithField("query", queryString).Debug("Validating query")

	// Parse the query
	query, err := influxql.ParseQuery(queryString)
	if err != nil {
		return FilterResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Invalid query syntax: %v", err),
			Query:   queryString,
		}
	}

	// Check each statement in the query
	for _, stmt := range query.Statements {
		if result := qf.validateStatement(stmt, queryString); !result.Allowed {
			return result
		}
	}

	// Check for blocked functions using regex
	for i, pattern := range qf.blockedPatterns {
		if pattern.MatchString(queryString) {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("Query contains blocked function: %s", qf.rules.BlockedFunctions[i]),
				Query:   queryString,
			}
		}
	}

	return FilterResult{
		Allowed: true,
		Reason:  "Query passed all filters",
		Query:   queryString,
	}
}

func (qf *QueryFilter) validateStatement(stmt influxql.Statement, queryString string) FilterResult {
	// Check if statement type is allowed
	stmtType := qf.getStatementType(stmt)
	if !qf.isStatementAllowed(stmtType) {
		return FilterResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Statement type '%s' not allowed", stmtType),
			Query:   queryString,
		}
	}

	switch s := stmt.(type) {
	case *influxql.SelectStatement:
		return qf.validateSelectStatement(s, queryString)
	case *influxql.ShowSeriesStatement:
		return qf.validateShowSeriesStatement(s, queryString)
	case *influxql.ShowMeasurementsStatement:
		return FilterResult{Allowed: true, Query: queryString}
	case *influxql.ShowDatabasesStatement:
		return FilterResult{Allowed: true, Query: queryString}
	case *influxql.ShowTagKeysStatement:
		return FilterResult{Allowed: true, Query: queryString}
	case *influxql.ShowTagValuesStatement:
		return FilterResult{Allowed: true, Query: queryString}
	case *influxql.ShowFieldKeysStatement:
		return FilterResult{Allowed: true, Query: queryString}
	default:
		// Block other statement types by default
		return FilterResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Statement type not allowed: %T", stmt),
			Query:   queryString,
		}
	}
}

func (qf *QueryFilter) validateSelectStatement(stmt *influxql.SelectStatement, queryString string) FilterResult {
	// Check if time filter is required and present
	if qf.rules.RequireTimeFilter {
		if !qf.hasTimeFilter(stmt.Condition) {
			return FilterResult{
				Allowed: false,
				Reason:  "Query must include a time filter (WHERE time > ... AND time < ...)",
				Query:   queryString,
			}
		}
	}

	// Check time range if time filter exists
	if timeRange := qf.extractTimeRange(stmt.Condition); timeRange != nil {
		maxDuration := time.Duration(qf.rules.MaxTimeRangeHours) * time.Hour
		if timeRange.Duration() > maxDuration {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("Time range (%v) exceeds maximum allowed (%v)", timeRange.Duration(), maxDuration),
				Query:   queryString,
			}
		}
	}

	// Check for potentially expensive operations
	if qf.isExpensiveQuery(stmt) {
		return FilterResult{
			Allowed: false,
			Reason:  "Query contains potentially expensive operations without proper constraints",
			Query:   queryString,
		}
	}

	return FilterResult{Allowed: true, Query: queryString}
}

func (qf *QueryFilter) validateShowSeriesStatement(stmt *influxql.ShowSeriesStatement, queryString string) FilterResult {
	// Check if expensive SHOW queries are blocked
	if qf.rules.BlockExpensiveShows {
		// SHOW SERIES can be expensive without LIMIT
		if stmt.Limit == 0 {
			return FilterResult{
				Allowed: false,
				Reason:  "SHOW SERIES queries must include a LIMIT clause",
				Query:   queryString,
			}
		}

		// Check if limit is reasonable
		if stmt.Limit > qf.rules.MaxShowSeriesLimit {
			return FilterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("SHOW SERIES LIMIT cannot exceed %d", qf.rules.MaxShowSeriesLimit),
				Query:   queryString,
			}
		}
	}

	return FilterResult{Allowed: true, Query: queryString}
}

func (qf *QueryFilter) hasTimeFilter(condition influxql.Expr) bool {
	if condition == nil {
		return false
	}

	return qf.containsTimeCondition(condition)
}

func (qf *QueryFilter) containsTimeCondition(expr influxql.Expr) bool {
	switch e := expr.(type) {
	case *influxql.BinaryExpr:
		// Check if this is a time condition
		if ref, ok := e.LHS.(*influxql.VarRef); ok && ref.Val == "time" {
			return true
		}
		// Recursively check both sides
		return qf.containsTimeCondition(e.LHS) || qf.containsTimeCondition(e.RHS)
	case *influxql.ParenExpr:
		return qf.containsTimeCondition(e.Expr)
	}
	return false
}

// TimeRange represents a time range with start and end times
type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (tr *TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

func (qf *QueryFilter) extractTimeRange(condition influxql.Expr) *TimeRange {
	if condition == nil {
		return nil
	}

	var start, end time.Time
	qf.extractTimeConditions(condition, &start, &end)

	if !start.IsZero() && !end.IsZero() {
		return &TimeRange{Start: start, End: end}
	}
	return nil
}

func (qf *QueryFilter) extractTimeConditions(expr influxql.Expr, start, end *time.Time) {
	switch e := expr.(type) {
	case *influxql.BinaryExpr:
		if ref, ok := e.LHS.(*influxql.VarRef); ok && ref.Val == "time" {
			if timeLit, ok := e.RHS.(*influxql.TimeLiteral); ok {
				switch e.Op {
				case influxql.GT, influxql.GTE:
					*start = timeLit.Val
				case influxql.LT, influxql.LTE:
					*end = timeLit.Val
				}
			}
		}
		qf.extractTimeConditions(e.LHS, start, end)
		qf.extractTimeConditions(e.RHS, start, end)
	case *influxql.ParenExpr:
		qf.extractTimeConditions(e.Expr, start, end)
	}
}

func (qf *QueryFilter) isExpensiveQuery(stmt *influxql.SelectStatement) bool {
	// Check for SELECT * without LIMIT (if configured)
	if qf.rules.BlockWildcardSelect && qf.hasWildcardSelect(stmt.Fields) && stmt.Limit == 0 {
		return true
	}

	// Check for GROUP BY without time bucketing and without LIMIT (if configured)
	if qf.rules.BlockUnlimitedGroupBy && len(stmt.Dimensions) > 0 && stmt.Limit == 0 {
		hasTimeBucket := false
		for _, dim := range stmt.Dimensions {
			if call, ok := dim.Expr.(*influxql.Call); ok && call.Name == "time" {
				hasTimeBucket = true
				break
			}
		}
		if !hasTimeBucket {
			return true
		}
	}

	return false
}

func (qf *QueryFilter) hasWildcardSelect(fields influxql.Fields) bool {
	for _, field := range fields {
		if ref, ok := field.Expr.(*influxql.Wildcard); ok && ref != nil {
			return true
		}
	}
	return false
}

// getStatementType returns a string representation of the statement type
func (qf *QueryFilter) getStatementType(stmt influxql.Statement) string {
	switch stmt.(type) {
	case *influxql.SelectStatement:
		return "SELECT"
	case *influxql.ShowSeriesStatement:
		return "SHOW SERIES"
	case *influxql.ShowMeasurementsStatement:
		return "SHOW MEASUREMENTS"
	case *influxql.ShowDatabasesStatement:
		return "SHOW DATABASES"
	case *influxql.ShowTagKeysStatement:
		return "SHOW TAG KEYS"
	case *influxql.ShowTagValuesStatement:
		return "SHOW TAG VALUES"
	case *influxql.ShowFieldKeysStatement:
		return "SHOW FIELD KEYS"
	case *influxql.CreateDatabaseStatement:
		return "CREATE"
	case *influxql.DropDatabaseStatement:
		return "DROP"
	case *influxql.DeleteSeriesStatement:
		return "DELETE"
	default:
		return fmt.Sprintf("%T", stmt)
	}
}

// isStatementAllowed checks if a statement type is allowed based on configuration
func (qf *QueryFilter) isStatementAllowed(stmtType string) bool {
	// Check if explicitly blocked
	for _, blocked := range qf.rules.BlockedStatements {
		if strings.EqualFold(blocked, stmtType) {
			return false
		}
	}

	// Check if explicitly allowed
	for _, allowed := range qf.rules.AllowedStatements {
		if strings.EqualFold(allowed, stmtType) {
			return true
		}
	}

	// If no allowed statements specified, allow by default (except explicitly blocked)
	if len(qf.rules.AllowedStatements) == 0 {
		return true
	}

	// If allowed statements are specified but this type is not in the list, block it
	return false
}
