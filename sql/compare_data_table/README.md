# PostgreSQL Master Data Comparison Tool

A Go script that compares master data tables between PostgreSQL databases in different environments (dev vs staging) and outputs the differences as an Excel file. This tool specifically focuses on comparing the content of master data tables to identify any discrepancies.

## Features

- Connects to both development and staging PostgreSQL databases
- Automatically identifies master data tables based on naming conventions
- Compares row counts and actual data values between environments
- Detects records that exist in only one environment
- Identifies specific value differences between matching records
- Outputs detailed results to an Excel file with color-coded indicators
- Shows primary key information to easily identify specific records
- Smart handling of tables without defined primary keys:
  - Automatically attempts to identify logical key columns
  - Creates composite keys using multiple columns when needed
  - Special handling for relationship tables with composite primary keys:
    - Detects tables like `role_permissions` with columns such as `role_code` and `permission_code`
    - Properly handles composite key relationships for accurate comparison
    - Intelligent detection of table patterns common in many-to-many relationships
  - Uses all columns as a composite key as a last resort

## Requirements

- Go 1.19 or higher
- PostgreSQL databases in both environments
- Required Go packages:
  - github.com/joho/godotenv
  - github.com/xuri/excelize/v2
  - gorm.io/driver/postgres
  - gorm.io/gorm

## Setup

1. Clone this repository
2. Copy `.env.example` to `.env` and update with your database credentials:

```bash
cp .env.example .env
```

3. Install dependencies:

```bash
go mod tidy
```

## Usage

### Basic Usage

Run the script with default options (compares all master tables):

```bash
go run cmd/main.go
```

### Command Line Options

The script supports several command-line options for flexibility:

```bash
# List all available tables and exit
go run cmd/main.go -list

# Compare specific tables (comma-separated)
go run cmd/main.go -tables=users,products,categories

# Compare tables matching a pattern
go run cmd/main.go -pattern=user

# Compare all tables (not just master tables)
go run cmd/main.go -master=false

# Specify output file name
go run cmd/main.go -output=comparison_report.xlsx

# Combine multiple options
go run cmd/main.go -tables=users,products -output=user_product_comparison.xlsx
```

### Available Options

| Option | Description |
|--------|-------------|
| `-list` | Lists all available tables and exits |
| `-tables=table1,table2` | Compares only the specified tables |
| `-pattern=string` | Compares tables whose names contain the pattern |
| `-master=bool` | When true (default), only includes master tables; when false, includes all tables |
| `-output=filename` | Specifies the output Excel filename |

### Example: Comparing a Relationship Table

For tables with special structures like `role_permissions` (which typically have columns like `role_code` and `permission_code`), the tool will automatically detect this pattern and use both columns as a composite key for accurate comparison:

```bash
# Compare the role_permissions table
.\run_comparison.bat -master=false -tables=role_permissions
```

The tool will automatically:
1. Detect that `role_permissions` is a relationship table
2. Use `role_code` and `permission_code` columns together as a composite key
3. Generate keys in the format `role_code:value|permission_code:value` for comparison
4. Identify any differences between the dev and staging environments

The tool will recognize this as a relation table and use both columns as a composite key for comparison.

The script will:
1. Connect to both development and staging databases
2. Retrieve and compare the selected tables between environments
3. Generate an Excel file with comparison results

## Output

The generated Excel file will contain:
- A summary sheet showing tables, row counts, and number of differences
- Individual detailed sheets for each master table:
  - `TableName_Diff`: Shows specific value differences with dev and staging values side-by-side
  - `TableName_OnlyInDev`: Records that exist in dev but not staging
  - `TableName_OnlyInStaging`: Records that exist in staging but not dev
- Color-coded cells to easily identify discrepancies

## Environment Variables

- `DEV_DB_HOST`: Development database host
- `DEV_DB_PORT`: Development database port
- `DEV_DB_USER`: Development database username
- `DEV_DB_PASSWORD`: Development database password
- `DEV_DB_NAME`: Development database name
- `STAGING_DB_HOST`: Staging database host
- `STAGING_DB_PORT`: Staging database port
- `STAGING_DB_USER`: Staging database username
- `STAGING_DB_PASSWORD`: Staging database password
- `STAGING_DB_NAME`: Staging database name
