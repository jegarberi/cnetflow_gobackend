package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// getPostgresMetrics fetches various PostgreSQL database metrics
func getPostgresMetrics() (*PostgresMetrics, error) {
	metrics := &PostgresMetrics{}

	// Get database sizes
	dbStats, err := getDatabaseStats()
	if err != nil {
		log.Printf("Error getting database stats: %v", err)
		return nil, err
	}
	metrics.DatabaseStats = dbStats

	// Get table sizes
	tableStats, err := getTableStats()
	if err != nil {
		log.Printf("Error getting table stats: %v", err)
		return nil, err
	}
	metrics.TableStats = tableStats

	// Get connection stats
	connStats, err := getConnectionStats()
	if err != nil {
		log.Printf("Error getting connection stats: %v", err)
		return nil, err
	}
	metrics.ConnectionStats = connStats

	// Get transaction stats
	txStats, err := getTransactionStats()
	if err != nil {
		log.Printf("Error getting transaction stats: %v", err)
		return nil, err
	}
	metrics.TransactionStats = txStats

	// Build activity stats
	metrics.ActivityStats = ActivityStat{
		ActiveConnections: connStats.Active,
		IdleConnections:   connStats.Idle,
		TPS:               txStats.TPS,
		Commits:           txStats.Commits,
		Rollbacks:         txStats.Rollbacks,
	}

	return metrics, nil
}

// getDatabaseStats returns size and connection information for all databases
func getDatabaseStats() ([]DatabaseStat, error) {
	query := `
		SELECT
			datname as database_name,
			pg_database_size(datname) as size,
			pg_size_pretty(pg_database_size(datname)) as size_formatted,
			numbackends as num_backends
		FROM pg_stat_database
		WHERE datname NOT IN ('template0', 'template1')
		ORDER BY pg_database_size(datname) DESC;
	`

	rows, err := config.Db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DatabaseStat
	for rows.Next() {
		var stat DatabaseStat
		err := rows.Scan(&stat.DatabaseName, &stat.Size, &stat.SizeFormatted, &stat.NumBackends)
		if err != nil {
			log.Printf("Error scanning database stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// getTableStats returns size information for all tables
func getTableStats() ([]TableStat, error) {
	query := `
		SELECT
			schemaname as schema_name,
			relname as table_name,
			pg_total_relation_size(schemaname||'.'||relname) as total_size,
			pg_relation_size(schemaname||'.'||relname) as table_size,
			pg_total_relation_size(schemaname||'.'||relname) - pg_relation_size(schemaname||'.'||relname) as indexes_size,
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname)) as size_formatted,
			n_live_tup as row_estimate
		FROM pg_stat_user_tables
		ORDER BY pg_total_relation_size(schemaname||'.'||relname) DESC
		LIMIT 20;
	`

	rows, err := config.Db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TableStat
	for rows.Next() {
		var stat TableStat
		err := rows.Scan(
			&stat.SchemaName,
			&stat.TableName,
			&stat.TotalSize,
			&stat.TableSize,
			&stat.IndexesSize,
			&stat.SizeFormatted,
			&stat.RowEstimate,
		)
		if err != nil {
			log.Printf("Error scanning table stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// getConnectionStats returns current connection information
func getConnectionStats() (ConnectionStat, error) {
	var stats ConnectionStat

	// Get total, active, and idle connections
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE state = 'active') as active,
			COUNT(*) FILTER (WHERE state = 'idle') as idle
		FROM pg_stat_activity
		WHERE datname IS NOT NULL;
	`

	err := config.Db.QueryRow(query).Scan(&stats.Total, &stats.Active, &stats.Idle)
	if err != nil {
		return stats, err
	}

	// Get max connections setting
	maxConnQuery := "SHOW max_connections;"
	err = config.Db.QueryRow(maxConnQuery).Scan(&stats.MaxConnections)
	if err != nil {
		log.Printf("Error getting max_connections: %v", err)
		stats.MaxConnections = 0
	}

	return stats, nil
}

// getTransactionStats returns transaction statistics
func getTransactionStats() (TransactionStat, error) {
	var stats TransactionStat

	query := `
		SELECT
			SUM(xact_commit) as commits,
			SUM(xact_rollback) as rollbacks
		FROM pg_stat_database
		WHERE datname NOT IN ('template0', 'template1');
	`

	err := config.Db.QueryRow(query).Scan(&stats.Commits, &stats.Rollbacks)
	if err != nil {
		return stats, err
	}

	// Calculate TPS (this is cumulative, for rate we'd need time-series data)
	// For now, we'll get the rate from pg_stat_database
	tpsQuery := `
		SELECT COALESCE(SUM(xact_commit + xact_rollback), 0) /
		       GREATEST(EXTRACT(EPOCH FROM (NOW() - MIN(stats_reset))), 1) as tps
		FROM pg_stat_database
		WHERE datname = current_database();
	`

	err = config.Db.QueryRow(tpsQuery).Scan(&stats.TPS)
	if err != nil {
		log.Printf("Error calculating TPS: %v", err)
		stats.TPS = 0
	}

	return stats, nil
}

// getPostgresMetricsRequest is the HTTP handler for the metrics API
func getPostgresMetricsRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	metrics, err := getPostgresMetrics()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching metrics: %v", err), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.Marshal(metrics)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}
