package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Database configuration struct
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// Function to 	abase
func connectDB(config DBConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		config.Host, config.User, config.Password, config.DBName, config.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

// Function to get all tables in the database
func getAllTables(db *gorm.DB) ([]string, error) {
	var tables []string

	// Query to get all table names in public schema
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	err := db.Raw(query).Scan(&tables).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	return tables, nil
}

// Function to get master tables in the database
func getMasterTables(db *gorm.DB) ([]string, error) {
	var tables []string

	// Query to get all table names in public schema that are likely master tables
	// This assumes master tables typically have names containing "master" or starting with "m_"
	// You can customize this query based on your naming convention
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		AND (
			table_name LIKE '%master%' 
			OR table_name LIKE 'm\\_%' 
			OR table_name IN (
				'categories', 'products', 'customers', 'suppliers', 
				'regions', 'countries', 'departments', 'currencies',
				'users', 'roles', 'permissions'
			)
		)
		ORDER BY table_name
	`

	err := db.Raw(query).Scan(&tables).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get master tables: %w", err)
	}

	return tables, nil
}

// Function to filter tables based on pattern
func filterTables(allTables []string, pattern string) []string {
	if pattern == "" {
		return allTables
	}

	var filtered []string
	for _, table := range allTables {
		if strings.Contains(table, pattern) {
			filtered = append(filtered, table)
		}
	}

	return filtered
}

// Function to identify potential key columns based on naming conventions
func tryIdentifyKeyColumns(columns []string) []string {
	if len(columns) == 0 {
		return []string{}
	}

	// Common primary key column names
	potentialIdColumns := []string{
		"id", "ID", "Id", "_id", "uuid", "guid", "key",
		"primary_key", "primarykey", "primary_id", "primaryid",
	}

	// Look for exact matches of common ID column names
	for _, column := range columns {
		for _, idCol := range potentialIdColumns {
			if strings.EqualFold(column, idCol) {
				return []string{column}
			}
		}
	}

	// Special case for relationship tables like role_permissions
	// Check for relationship table pattern first, since it takes precedence over individual column detection
	if len(columns) == 2 {
		// For tables with exactly 2 columns, check if one is likely a role/code and the other is permission
		col1Lower := strings.ToLower(columns[0])
		col2Lower := strings.ToLower(columns[1])

		// Check if columns match role_permissions pattern (role_code + permission)
		if (strings.Contains(col1Lower, "role") && strings.Contains(col2Lower, "permission")) ||
			(strings.Contains(col2Lower, "role") && strings.Contains(col1Lower, "permission")) ||
			(strings.Contains(col1Lower, "code") && strings.Contains(col2Lower, "permission")) ||
			(strings.Contains(col2Lower, "code") && strings.Contains(col1Lower, "permission")) {
			log.Printf("Identified relationship table pattern (role/code + permission). Using both columns as composite key: %v", columns)
			return columns // Use both columns as composite key
		}
	}

	// Look for columns ending with "_id" or "_code" pattern (e.g., role_id, role_code)
	var idColumns []string
	for _, column := range columns {
		lowerCol := strings.ToLower(column)
		if strings.HasSuffix(lowerCol, "_id") || strings.HasSuffix(lowerCol, "_code") {
			idColumns = append(idColumns, column)
		}
	}

	// Found columns with _id or _code suffix, but need to check if it's a relationship table
	if len(idColumns) > 0 {
		// Check if we have a code/id column and a permission column - common in relationship tables
		for _, column := range columns {
			if strings.Contains(strings.ToLower(column), "permission") {
				// We likely have a role_code/id + permission pattern, use both as composite key
				hasIdOrCode := false
				for _, idCol := range idColumns {
					if strings.Contains(strings.ToLower(idCol), "role") ||
						strings.Contains(strings.ToLower(idCol), "code") {
						hasIdOrCode = true
						break
					}
				}

				if hasIdOrCode {
					log.Printf("Identified role + permission pattern. Using composite key of: %v and %s", idColumns, column)
					// Return a composite key including both the ID columns and the permission column
					return append(idColumns, column)
				}
			}
		}

		return idColumns // Return all columns ending with _id or _code as a composite key
	} // Look for columns containing "id" in the name
	for _, column := range columns {
		if strings.Contains(strings.ToLower(column), "id") &&
			!strings.Contains(strings.ToLower(column), "hide") &&
			!strings.Contains(strings.ToLower(column), "guid") {
			idColumns = append(idColumns, column)
		}
	}

	if len(idColumns) > 0 {
		return idColumns
	}

	// Special case handling for common relational tables like role_permissions
	// These tables often have a composite key of two columns
	if len(columns) == 2 {
		// For a table with exactly two columns, use both as a composite key
		// This is common in many-to-many relationship tables
		log.Printf("Table appears to be a relation table with two columns, using both as composite key: %v", columns)
		return columns
	} else if len(columns) <= 5 {
		// For small tables with <= 5 columns, check if it looks like a relation table
		for _, col := range columns {
			lowerCol := strings.ToLower(col)
			// If any column has "role", "permission", or "code" in its name, it's likely a relation table
			if strings.Contains(lowerCol, "role") ||
				strings.Contains(lowerCol, "permission") ||
				strings.Contains(lowerCol, "code") ||
				strings.Contains(lowerCol, "relation") {
				log.Printf("Table appears to be a relation table, using all columns as composite key")
				return columns
			}
		}
	}

	// If no key columns found, return empty slice and let the caller decide what to do
	return []string{}
} // Function to compare a specific master table between two databases
func compareTable(devDB, stagingDB *gorm.DB, tableName string) (map[string]interface{}, error) {
	// Get column names for the table
	var columns []string
	var primaryKeys []string

	// Get all columns
	err := devDB.Raw("SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = ? ORDER BY ordinal_position",
		tableName).Scan(&columns).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("no columns found for table %s", tableName)
	}

	// Find primary keys
	err = devDB.Raw(`
		SELECT kcu.column_name 
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name 
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' 
			AND tc.table_schema = 'public' 
			AND tc.table_name = ?
		ORDER BY kcu.ordinal_position
	`, tableName).Scan(&primaryKeys).Error

	if err != nil {
		log.Printf("Warning: Could not determine primary keys for table %s: %v", tableName, err)
		primaryKeys = tryIdentifyKeyColumns(columns) // Try to identify potential key columns
	}

	if len(primaryKeys) == 0 {
		// Special case for role_permissions table and similar relationship tables
		if tableName == "role_permissions" || strings.HasSuffix(tableName, "_permissions") {
			// For role_permissions table, we know it's a composite key of role_code + permission
			for _, col := range columns {
				lowerCol := strings.ToLower(col)
				if strings.Contains(lowerCol, "role") ||
					strings.Contains(lowerCol, "code") ||
					strings.Contains(lowerCol, "permission") {
					primaryKeys = append(primaryKeys, col)
				}
			}
			// If we found matching columns, use them
			if len(primaryKeys) > 0 {
				log.Printf("Table '%s' identified as a role_permissions table. Using columns as composite key: %v",
					tableName, primaryKeys)
			}
		}

		// If still no primary keys identified, check other relation table patterns
		if len(primaryKeys) == 0 {
			if len(columns) <= 5 {
				// Look for relation table indicators
				isRelationTable := false
				for _, col := range columns {
					lowerCol := strings.ToLower(col)
					if strings.Contains(lowerCol, "role") ||
						strings.Contains(lowerCol, "permission") ||
						strings.Contains(lowerCol, "code") {
						isRelationTable = true
						break
					}
				}

				if isRelationTable {
					// For relation tables, we use first few columns as the composite key
					if len(columns) <= 2 {
						// For very small tables, use all columns
						primaryKeys = columns
						log.Printf("Table '%s' appears to be a relation table with %d columns. Using all columns as composite key.",
							tableName, len(columns))
					} else {
						// For tables with 3-5 columns, use first 2 columns as likely composite key
						primaryKeys = columns[:2]
						log.Printf("Table '%s' appears to be a relation table. Using first %d columns as composite key: %v",
							tableName, len(primaryKeys), primaryKeys)
					}
				} else {
					log.Printf("Warning: No primary keys found for table '%s', using all columns as composite key", tableName)
					primaryKeys = columns
				}
			} else {
				log.Printf("Warning: No primary keys found for table '%s', using all columns as composite key", tableName)
				// Create a composite key using all columns when no primary key is defined
				primaryKeys = columns

				// Add to result that this table has no primary keys (used for reporting)
				log.Printf("Note: Table '%s' doesn't have primary keys defined. Comparison may be less accurate.", tableName)
			}
		}
	} // Count rows in both databases
	var devCount, stagingCount int64

	if err := devDB.Table(tableName).Count(&devCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count rows in dev table %s: %w", tableName, err)
	}

	if err := stagingDB.Table(tableName).Count(&stagingCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count rows in staging table %s: %w", tableName, err)
	}

	// Select columns joined with commas for the query
	columnsStr := strings.Join(columns, ", ")

	// Prepare data structures
	var devData, stagingData []map[string]interface{}
	var differences []map[string]interface{}
	var onlyInDev []map[string]interface{}
	var onlyInStaging []map[string]interface{}

	// Get ALL data from both tables
	// For real master data tables, this should be fine as they typically don't have massive amounts of data
	// But we'll limit to 1000 rows just in case
	if err := devDB.Raw(fmt.Sprintf("SELECT %s FROM %s LIMIT 1000", columnsStr, tableName)).Scan(&devData).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch data from dev table %s: %w", tableName, err)
	} else {
		log.Printf("Retrieved %d rows from dev table %s", len(devData), tableName)
		// If it's a small number of rows, log them for debugging
		if tableName == "role_permissions" && len(devData) <= 10 {
			log.Printf("Dev data for %s: %+v", tableName, devData)
		}
	}

	if err := stagingDB.Raw(fmt.Sprintf("SELECT %s FROM %s LIMIT 1000", columnsStr, tableName)).Scan(&stagingData).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch data from staging table %s: %w", tableName, err)
	} else {
		log.Printf("Retrieved %d rows from staging table %s", len(stagingData), tableName)
		// If it's a small number of rows, log them for debugging
		if tableName == "role_permissions" && len(stagingData) <= 10 {
			log.Printf("Staging data for %s: %+v", tableName, stagingData)
		}
	}

	// Create maps for easy lookup by primary key
	devDataMap := make(map[string]map[string]interface{})
	stagingDataMap := make(map[string]map[string]interface{})

	// Use static variable to track which tables we've already logged information for
	// This avoids excessive repeated log messages
	var loggedRelationshipTables = make(map[string]bool)

	// Helper function to create a composite key from multiple columns
	makeKey := func(row map[string]interface{}, keyColumns []string) string {
		// Special handling for role_permissions pattern
		isRolePermissionPattern := false

		// Check if this is a relationship table with role and permission columns
		if len(keyColumns) >= 2 && tableName == "role_permissions" {
			// Only log once per table per script execution
			if !loggedRelationshipTables[tableName] {
				log.Printf("Using composite key pattern for relationship table: %v", keyColumns[:2])
				loggedRelationshipTables[tableName] = true
			}
			isRolePermissionPattern = true
		}

		// If using all columns as key (for tables without PK), limit to first 5 non-null columns
		// to avoid excessively long keys
		useAllColumnsMode := len(keyColumns) > 5 && len(keyColumns) == len(columns) && !isRolePermissionPattern

		var keyParts []string
		nonNullCount := 0

		for _, col := range keyColumns {
			if val, ok := row[col]; ok && val != nil {
				// Convert the value to string representation for the key
				valStr := fmt.Sprintf("%v", val)

				// Skip empty values in all-columns mode to make keys more meaningful
				// But for role_permissions pattern, include all key columns even if empty
				if useAllColumnsMode && !isRolePermissionPattern && (valStr == "" || valStr == "0" || valStr == "<nil>" || valStr == "null") {
					continue
				}

				keyParts = append(keyParts, fmt.Sprintf("%s:%v", col, val))
				nonNullCount++

				// In all-columns mode, limit to first 5 non-null values
				// But for role_permissions pattern, include all key columns
				if useAllColumnsMode && !isRolePermissionPattern && nonNullCount >= 5 {
					break
				}
			} else {
				// For explicitly specified key columns, include nulls
				// Always include nulls for role_permissions pattern
				if !useAllColumnsMode || isRolePermissionPattern {
					keyParts = append(keyParts, fmt.Sprintf("%s:null", col))
				}
			}
		}

		// If we couldn't build a meaningful key, use a row number as fallback
		// This might lead to inaccurate comparisons, but prevents empty keys
		if len(keyParts) == 0 {
			// Use the first column, whatever it is
			if val, ok := row[columns[0]]; ok {
				keyParts = append(keyParts, fmt.Sprintf("%s:%v", columns[0], val))
			} else {
				// Last resort - random identifier
				keyParts = append(keyParts, fmt.Sprintf("row:%d", time.Now().UnixNano()))
			}
		}

		return strings.Join(keyParts, "|")
	}

	// Populate maps
	for _, row := range devData {
		key := makeKey(row, primaryKeys)
		devDataMap[key] = row
	}

	for _, row := range stagingData {
		key := makeKey(row, primaryKeys)
		stagingDataMap[key] = row
	}

	// Find differences and records that exist only in one environment
	if tableName == "role_permissions" {
		log.Printf("Analyzing differences in role_permissions table with %d dev rows and %d staging rows",
			len(devDataMap), len(stagingDataMap))
		// Log a few sample keys to debug
		count := 0
		for key := range devDataMap {
			if count < 3 {
				log.Printf("Sample key in dev data: %s", key)
				count++
			} else {
				break
			}
		}
		count = 0
		for key := range stagingDataMap {
			if count < 3 {
				log.Printf("Sample key in staging data: %s", key)
				count++
			} else {
				break
			}
		}
	}

	for key, devRow := range devDataMap {
		if stagingRow, exists := stagingDataMap[key]; exists {
			// Record exists in both - check for differences in values
			diffRow := make(map[string]interface{})
			hasDiff := false

			for _, col := range columns {
				devVal := devRow[col]
				stagingVal := stagingRow[col]

				// Simple string comparison - may need to be enhanced for specific data types
				if fmt.Sprintf("%v", devVal) != fmt.Sprintf("%v", stagingVal) {
					diffRow["key"] = key
					diffRow["column"] = col
					diffRow["dev_value"] = devVal
					diffRow["staging_value"] = stagingVal

					// For composite key tables like role_permissions, add key columns for clarity
					for _, pkCol := range primaryKeys {
						if pkVal, ok := devRow[pkCol]; ok {
							diffRow["pk_"+pkCol] = pkVal
						}
					}

					differences = append(differences, diffRow)
					hasDiff = true
					diffRow = make(map[string]interface{}) // Create a new map for next difference
				}
			}

			if hasDiff {
				// Add identifiers for easier reading
				for _, pkCol := range primaryKeys {
					diffRow["pk_"+pkCol] = devRow[pkCol]
				}
			}
		} else {
			// Record only exists in dev
			onlyInDev = append(onlyInDev, devRow)
			if tableName == "role_permissions" {
				log.Printf("Found record only in dev with key: %s, data: %+v", key, devRow)
			}
		}
	}

	// Find records only in staging
	for key, stagingRow := range stagingDataMap {
		if _, exists := devDataMap[key]; !exists {
			onlyInStaging = append(onlyInStaging, stagingRow)
			if tableName == "role_permissions" {
				log.Printf("Found record only in staging with key: %s, data: %+v", key, stagingRow)
			}
		}
	}

	// Return comparison result
	result := map[string]interface{}{
		"table_name":      tableName,
		"columns":         columns,
		"primary_keys":    primaryKeys,
		"has_primary_key": len(primaryKeys) < len(columns), // true if we're not using all columns as key
		"using_composite": len(primaryKeys) > 1,            // true if using multiple columns as key
		"dev_count":       devCount,
		"staging_count":   stagingCount,
		"count_diff":      devCount - stagingCount,
		"dev_data":        devData,
		"staging_data":    stagingData,
		"differences":     differences,
		"only_in_dev":     onlyInDev,
		"only_in_staging": onlyInStaging,
	}

	return result, nil
}

// Function to export comparison results to Excel
func exportToExcel(results []map[string]interface{}, filename string) error {
	f := excelize.NewFile()

	// Create summary sheet
	summarySheet := "Summary"
	f.SetSheetName("Sheet1", summarySheet)

	// Set summary sheet headers
	headers := []string{"Table Name", "PK Type", "Dev Count", "Staging Count", "Count Difference", "Value Differences", "Only in Dev", "Only in Staging"}
	for i, header := range headers {
		cell := fmt.Sprintf("%c%d", 'A'+i, 1)
		f.SetCellValue(summarySheet, cell, header)
	}

	// Apply header style
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})
	// Apply header style to first row
	f.SetRowStyle(summarySheet, 1, 1, style)

	// Set column widths
	f.SetColWidth(summarySheet, "A", "A", 20)
	f.SetColWidth(summarySheet, "B", "B", 18) // PK Type column
	f.SetColWidth(summarySheet, "C", "H", 15)

	// Fill summary data
	for i, result := range results {
		rowNum := i + 2
		tableName := result["table_name"].(string)
		differences := result["differences"].([]map[string]interface{})
		onlyInDev := result["only_in_dev"].([]map[string]interface{})
		onlyInStaging := result["only_in_staging"].([]map[string]interface{})
		primaryKeys := result["primary_keys"].([]string)
		hasPrimaryKey := result["has_primary_key"].(bool)
		usingComposite := result["using_composite"].(bool)

		// Determine what kind of key is being used for comparison
		var keyTypeText string
		if !hasPrimaryKey {
			keyTypeText = "All Columns"
		} else if usingComposite {
			keyTypeText = fmt.Sprintf("Composite (%d cols)", len(primaryKeys))
		} else {
			keyTypeText = primaryKeys[0] // Single column PK
		}

		f.SetCellValue(summarySheet, fmt.Sprintf("A%d", rowNum), tableName)
		f.SetCellValue(summarySheet, fmt.Sprintf("B%d", rowNum), keyTypeText)
		f.SetCellValue(summarySheet, fmt.Sprintf("C%d", rowNum), result["dev_count"])
		f.SetCellValue(summarySheet, fmt.Sprintf("D%d", rowNum), result["staging_count"])
		f.SetCellValue(summarySheet, fmt.Sprintf("E%d", rowNum), result["count_diff"])
		f.SetCellValue(summarySheet, fmt.Sprintf("F%d", rowNum), len(differences))
		f.SetCellValue(summarySheet, fmt.Sprintf("G%d", rowNum), len(onlyInDev))
		f.SetCellValue(summarySheet, fmt.Sprintf("H%d", rowNum), len(onlyInStaging))

		// Add color to rows with differences
		if len(differences) > 0 || len(onlyInDev) > 0 || len(onlyInStaging) > 0 {
			diffStyle, _ := f.NewStyle(&excelize.Style{
				Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFEB9C"}, Pattern: 1},
			})
			f.SetRowStyle(summarySheet, rowNum, rowNum, diffStyle)
		}

		// Create detailed sheets for each table
		createDetailedSheets(f, result, tableName)
	}

	// Save the Excel file
	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	return nil
}

// Helper function to create detailed sheets for each table comparison
func createDetailedSheets(f *excelize.File, result map[string]interface{}, tableName string) {
	columns := result["columns"].([]string)
	primaryKeys := result["primary_keys"].([]string)
	differences := result["differences"].([]map[string]interface{})
	onlyInDev := result["only_in_dev"].([]map[string]interface{})
	onlyInStaging := result["only_in_staging"].([]map[string]interface{})

	// Create a differences sheet
	if len(differences) > 0 {
		diffSheet := fmt.Sprintf("%s_Diff", tableName)
		if len(diffSheet) > 31 {
			diffSheet = diffSheet[:31]
		}
		f.NewSheet(diffSheet)

		// Headers for diff sheet
		diffHeaders := []string{"Primary Key"}
		for _, pk := range primaryKeys {
			diffHeaders = append(diffHeaders, pk)
		}
		diffHeaders = append(diffHeaders, "Column", "Dev Value", "Staging Value")

		for i, header := range diffHeaders {
			cell := fmt.Sprintf("%c%d", 'A'+i, 1)
			f.SetCellValue(diffSheet, cell, header)
		}

		// Apply header style
		style, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1},
		})
		f.SetRowStyle(diffSheet, 1, 1, style)

		// Set column widths
		for i := 0; i < len(diffHeaders); i++ {
			colName := fmt.Sprintf("%c", 'A'+i)
			f.SetColWidth(diffSheet, colName, colName, 18)
		}

		// Fill differences data
		for i, diff := range differences {
			rowNum := i + 2

			// Key column
			f.SetCellValue(diffSheet, fmt.Sprintf("A%d", rowNum), diff["key"])

			// Primary key columns
			colOffset := 1
			for j, pk := range primaryKeys {
				if pkVal, ok := diff["pk_"+pk]; ok {
					f.SetCellValue(diffSheet, fmt.Sprintf("%c%d", 'A'+colOffset+j, rowNum), pkVal)
				}
			}
			colOffset += len(primaryKeys)

			// Difference details
			f.SetCellValue(diffSheet, fmt.Sprintf("%c%d", 'A'+colOffset, rowNum), diff["column"])
			f.SetCellValue(diffSheet, fmt.Sprintf("%c%d", 'A'+colOffset+1, rowNum), diff["dev_value"])
			f.SetCellValue(diffSheet, fmt.Sprintf("%c%d", 'A'+colOffset+2, rowNum), diff["staging_value"])

			// Add background color for easy visibility
			diffStyle, _ := f.NewStyle(&excelize.Style{
				Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFEB9C"}, Pattern: 1},
			})
			f.SetRowStyle(diffSheet, rowNum, rowNum, diffStyle)
		}
	}

	// Create a sheet for records only in dev
	if len(onlyInDev) > 0 {
		devOnlySheet := fmt.Sprintf("%s_OnlyInDev", tableName)
		if len(devOnlySheet) > 31 {
			devOnlySheet = devOnlySheet[:31]
		}
		f.NewSheet(devOnlySheet)

		// Headers
		for i, col := range columns {
			cell := fmt.Sprintf("%c%d", 'A'+i, 1)
			f.SetCellValue(devOnlySheet, cell, col)
		}

		// Style headers
		style, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1},
		})
		f.SetRowStyle(devOnlySheet, 1, 1, style)

		// Data
		for rowIdx, row := range onlyInDev {
			for colIdx, col := range columns {
				cell := fmt.Sprintf("%c%d", 'A'+colIdx, rowIdx+2)
				f.SetCellValue(devOnlySheet, cell, row[col])
			}
		}

		// Set column widths
		for i := 0; i < len(columns); i++ {
			colName := fmt.Sprintf("%c", 'A'+i)
			f.SetColWidth(devOnlySheet, colName, colName, 15)
		}
	}

	// Create a sheet for records only in staging
	if len(onlyInStaging) > 0 {
		stagingOnlySheet := fmt.Sprintf("%s_OnlyInStaging", tableName)
		if len(stagingOnlySheet) > 31 {
			stagingOnlySheet = stagingOnlySheet[:31]
		}
		f.NewSheet(stagingOnlySheet)

		// Headers
		for i, col := range columns {
			cell := fmt.Sprintf("%c%d", 'A'+i, 1)
			f.SetCellValue(stagingOnlySheet, cell, col)
		}

		// Style headers
		style, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true},
			Fill: excelize.Fill{Type: "pattern", Color: []string{"#DDEBF7"}, Pattern: 1},
		})
		f.SetRowStyle(stagingOnlySheet, 1, 1, style)

		// Data
		for rowIdx, row := range onlyInStaging {
			for colIdx, col := range columns {
				cell := fmt.Sprintf("%c%d", 'A'+colIdx, rowIdx+2)
				f.SetCellValue(stagingOnlySheet, cell, row[col])
			}
		}

		// Set column widths
		for i := 0; i < len(columns); i++ {
			colName := fmt.Sprintf("%c", 'A'+i)
			f.SetColWidth(stagingOnlySheet, colName, colName, 15)
		}
	}
}

func main() {
	// Define command-line flags
	listTablesFlag := flag.Bool("list", false, "List available tables and exit")
	specificTablesFlag := flag.String("tables", "", "Comma-separated list of specific tables to compare")
	patternFlag := flag.String("pattern", "", "Pattern to filter table names (e.g. 'user' will match 'users', 'user_roles', etc.)")
	masterTablesFlag := flag.Bool("master", true, "Only include master tables in comparison")
	outputFlag := flag.String("output", "", "Output file name (default: auto-generated with timestamp)")

	// Parse command-line arguments
	flag.Parse()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Configure database connections
	devConfig := DBConfig{
		Host:     getEnv("DEV_DB_HOST", "localhost"),
		Port:     getEnv("DEV_DB_PORT", "5432"),
		User:     getEnv("DEV_DB_USER", "postgres"),
		Password: getEnv("DEV_DB_PASSWORD", ""),
		DBName:   getEnv("DEV_DB_NAME", ""),
	}

	stagingConfig := DBConfig{
		Host:     getEnv("STAGING_DB_HOST", "localhost"),
		Port:     getEnv("STAGING_DB_PORT", "5432"),
		User:     getEnv("STAGING_DB_USER", "postgres"),
		Password: getEnv("STAGING_DB_PASSWORD", ""),
		DBName:   getEnv("STAGING_DB_NAME", ""),
	}

	// Connect to databases
	log.Println("Connecting to development database...")
	devDB, err := connectDB(devConfig)
	if err != nil {
		log.Fatalf("Failed to connect to development database: %v", err)
	}

	log.Println("Connecting to staging database...")
	stagingDB, err := connectDB(stagingConfig)
	if err != nil {
		log.Fatalf("Failed to connect to staging database: %v", err)
	}

	// Get tables based on flags
	var allTables []string
	var tablesToCompare []string

	if *masterTablesFlag {
		log.Println("Retrieving master data tables from database...")
		allTables, err = getMasterTables(devDB)
	} else {
		log.Println("Retrieving all tables from database...")
		allTables, err = getAllTables(devDB)
	}

	if err != nil {
		log.Fatalf("Failed to get tables: %v", err)
	}

	// Handle list tables flag - just show tables and exit
	if *listTablesFlag {
		fmt.Println("Available tables:")
		for i, table := range allTables {
			fmt.Printf("%d. %s\n", i+1, table)
		}
		return
	}

	// Determine which tables to compare
	if *specificTablesFlag != "" {
		// Use specific tables provided in the flag
		requestedTables := strings.Split(*specificTablesFlag, ",")
		for _, tableName := range requestedTables {
			trimmedName := strings.TrimSpace(tableName)
			// Check if this table exists in the database
			found := false
			for _, availableTable := range allTables {
				if availableTable == trimmedName {
					found = true
					tablesToCompare = append(tablesToCompare, trimmedName)
					break
				}
			}

			if !found {
				log.Printf("Warning: Table '%s' not found in database - skipping", trimmedName)
			}
		}
	} else if *patternFlag != "" {
		// Filter tables by pattern
		tablesToCompare = filterTables(allTables, *patternFlag)
	} else {
		// Use all tables
		tablesToCompare = allTables
	}

	if len(tablesToCompare) == 0 {
		log.Println("No tables selected for comparison. Use -list to see available tables.")
		return
	}

	log.Printf("Selected %d tables for comparison: %s", len(tablesToCompare), strings.Join(tablesToCompare, ", "))

	// Ask for confirmation if more than 10 tables are selected
	if len(tablesToCompare) > 10 {
		fmt.Printf("You've selected %d tables for comparison. This might take a while. Continue? (y/n): ", len(tablesToCompare))
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			log.Println("Operation cancelled by user")
			return
		}
	}

	// Compare selected tables
	var results []map[string]interface{}
	for _, tableName := range tablesToCompare {
		log.Printf("Comparing table: %s", tableName)

		result, err := compareTable(devDB, stagingDB, tableName)
		if err != nil {
			log.Printf("Error comparing table %s: %v", tableName, err)
			continue
		}

		// Get actual differences to show in log
		differences := result["differences"].([]map[string]interface{})
		onlyInDev := result["only_in_dev"].([]map[string]interface{})
		onlyInStaging := result["only_in_staging"].([]map[string]interface{})

		log.Printf("Table %s: %d value differences, %d records only in dev, %d records only in staging",
			tableName, len(differences), len(onlyInDev), len(onlyInStaging))

		results = append(results, result)
	}

	// Generate filename with timestamp
	var filename string
	if *outputFlag != "" {
		filename = *outputFlag
	} else {
		timestamp := time.Now().Format("20060102_150405")
		filename = fmt.Sprintf("data_comparison_%s.xlsx", timestamp)
	}

	// Export results to Excel
	log.Printf("Exporting comparison results to %s", filename)
	if err := exportToExcel(results, filename); err != nil {
		log.Fatalf("Failed to export to Excel: %v", err)
	}

	log.Printf("Comparison completed successfully. Results saved to %s", filename)
}

// Helper function to get environment variable with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
