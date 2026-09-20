package jsonrpc

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/pkg/metric"
	"github.com/komari-monitor/komari/pkg/rpc"
)

const (
	databaseTargetMain    = "main"
	databaseTargetMetrics = "metrics"
)

type databaseTablesParams struct {
	Database *string `json:"database"`
}

type databaseTablesResponse struct {
	Database string   `json:"database"`
	Driver   string   `json:"driver"`
	Tables   []string `json:"tables"`
}

func init() {

	RegisterWithGroupAndMeta("dbTables", rpc.RoleAdmin, adminDBTables, &rpc.MethodMeta{
		Name:    "admin:dbTables",
		Summary: "List tables in the main or metrics database",
		Params: []rpc.ParamMeta{
			{Name: "database", Type: "main | metrics", Description: "Database target (default main)"},
		},
		Returns: "{ database: string, driver: string, tables: string[] }",
	})
}

func adminDBTables(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params databaseTablesParams
	if err := req.BindParams(&params); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid request body: "+err.Error(), nil)
	}
	target, rpcErr := parseDatabaseTarget(params.Database)
	if rpcErr != nil {
		return nil, rpcErr
	}

	response, err := listDatabaseTables(ctx, target)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Failed to list database tables: "+err.Error(), nil)
	}
	return response, nil
}

func parseDatabaseTarget(value *string) (string, *rpc.JsonRpcError) {
	if value == nil {
		return databaseTargetMain, nil
	}
	target := strings.TrimSpace(*value)
	if target != databaseTargetMain && target != databaseTargetMetrics {
		return "", rpc.MakeError(rpc.InvalidParams, "database must be main or metrics", nil)
	}
	return target, nil
}

func listDatabaseTables(ctx context.Context, target string) (databaseTablesResponse, error) {
	var (
		rows         *sql.Rows
		actualDriver metric.Driver
		release      func()
		err          error
	)
	switch target {
	case databaseTargetMain:
		statement, err := tableListSQL(metric.DriverSQLite)
		if err != nil {
			return databaseTablesResponse{}, err
		}
		rows, actualDriver, release, err = openDatabaseRows(ctx, target, statement)
	case databaseTargetMetrics:
		rows, actualDriver, release, err = metricstore.QueryForDriver(ctx, tableListSQL)
	default:
		return databaseTablesResponse{}, fmt.Errorf("unsupported database target: %s", target)
	}
	if err != nil {
		return databaseTablesResponse{}, err
	}
	defer func() {
		_ = rows.Close()
		release()
	}()

	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return databaseTablesResponse{}, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return databaseTablesResponse{}, err
	}
	sort.Strings(tables)
	return databaseTablesResponse{Database: target, Driver: string(actualDriver), Tables: tables}, nil
}

func openDatabaseRows(ctx context.Context, target, statement string, args ...any) (*sql.Rows, metric.Driver, func(), error) {
	switch target {
	case databaseTargetMain:
		db, err := dbcore.GetDBInstance().DB()
		if err != nil {
			return nil, "", nil, err
		}
		rows, err := db.QueryContext(ctx, statement, args...)
		return rows, metric.DriverSQLite, func() {}, err
	case databaseTargetMetrics:
		return metricstore.QueryContext(ctx, statement, args...)
	default:
		return nil, "", nil, fmt.Errorf("unsupported database target: %s", target)
	}
}

func collectDatabaseRows(rows *sql.Rows, limit int) ([]string, [][]any, bool, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	resultRows := make([][]any, 0, limit)
	values := make([]any, len(columns))
	scans := make([]any, len(columns))
	for i := range scans {
		scans[i] = &values[i]
	}

	for rows.Next() {
		if len(resultRows) == limit {
			return columns, resultRows, true, nil
		}
		if err := rows.Scan(scans...); err != nil {
			return nil, nil, false, err
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = normalizeDatabaseValue(value)
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return columns, resultRows, false, nil
}

func normalizeDatabaseValue(value any) any {
	switch value := value.(type) {
	case []byte:
		return string(value)
	case time.Time:
		return value.Format(time.RFC3339Nano)
	default:
		return value
	}
}

func tableListSQL(driver metric.Driver) (string, error) {
	switch driver {
	case metric.DriverSQLite:
		return "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'", nil
	case metric.DriverMySQL:
		return "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'", nil
	case metric.DriverPostgreSQL:
		return "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = current_schema()", nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}
}
