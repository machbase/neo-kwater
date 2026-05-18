# neo-kwater

`neo-kwater` imports K-water CSV files into a Machbase Neo tag table.

## Usage

```sh
kwater import -dir <dir> -db <host:port> -user <user> -password <password> -table <table> [-c <n>] [-ignore-low-confidence <n>]
```

Example:

```sh
kwater import -dir ./test/data -db 127.0.0.1:5656 -user sys -password manager -table kwdam -c 10 -ignore-low-confidence -1
```

Arguments:

- `-dir`: Directory containing CSV files. Files are processed in filename order.
- `-db`: Machbase Neo server address, such as `127.0.0.1:5656`.
- `-user`: Database user. Default is `sys`.
- `-password`: Database password. Default is `manager`.
- `-table`: Target tag table name.
- `-c`: Number of CSV files to process concurrently. Default is `10`.
- `-ignore-low-confidence <n>`: Skip CSV records whose `CONFIDENCE` value is lower than `n`. If this option is not specified, it defaults to the minimum integer value and no records are skipped. When `VALUE` is empty or omitted, `NULL` is appended for the value field.

The command must include the `import` subcommand. For example, this is invalid:

```sh
kwater -dir ./test/data -db 127.0.0.1:5656 -table kwdam
```

Use this instead:

```sh
kwater import -dir ./test/data -db 127.0.0.1:5656 -table kwdam
```

## CSV Format

Each CSV file must contain rows in this order:

```csv
NAME,TIME,VALUE,CONFIDENCE
ADD1AIG01ACTI01,2016-04-28 04:52:00,0,100
```

Fields:

- `NAME`: tag name, stored as varchar.
- `TIME`: timestamp parsed in the `Asia/Seoul` location with layout `YYYY-MM-DD HH:MM:SS`.
- `VALUE`: float64 value. Empty or omitted values are appended as `NULL`.
- `CONFIDENCE`: integer confidence value.

The header row is optional. If present, it is skipped.

## Target Table Example

```sql
create tag table kwdam (
    name varchar(80) primary key,
    time datetime base time,
    value double,
    conf int
)
```

## Development

Run unit tests:

```sh
go test ./...
```

Run the local integration test against a running Neo server at `127.0.0.1:5656`:

```sh
KWATER_INTEGRATION=1 go test ./... -run TestIntegrationImportKWDam -count=1
```

Build locally:

```sh
go build -o kwater .
```

## Releases

Pushing to `main` runs tests.

Pushing a tag, such as `v0.1.0`, runs tests, builds these binaries, packages them with this README, and uploads `neo-kwater-<tag>.zip` to the GitHub Release:

- `kwater-linux-amd64`
- `kwater-darwin-arm64`
- `kwater-windows-amd64.exe`
