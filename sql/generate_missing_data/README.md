# Generate Missing Permissions

This script checks for permissions in the `role_permissions` table that are missing from the `permissions` table and inserts them.

## Usage

1. Copy `.env.example` to `.env` and update the database connection details:
   ```
   cp .env.example .env
   ```

2. Edit the `.env` file with your database connection information:
   ```
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=your_password
   DB_NAME=your_database_name
   ```

3. Run the script:
   ```
   go run main.go
   ```

   Or use the provided batch file:
   ```
   .\run_sync_permissions.bat
   ```

4. To specify a different .env file:
   ```
   go run main.go -env=path/to/env/file
   ```

## Output

The script will:
1. Connect to the specified database
2. Find all distinct permissions in `role_permissions` table
3. Check which ones don't exist in the `permissions` table
4. Insert the missing permissions
5. Generate a log file with details of the operation

## Log Files

Log files are generated in the format `permissions_insert_YYYYMMDD_HHMMSS.log` in the current directory.
