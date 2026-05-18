# neo-kwater

`neo-kwater`는 K-water CSV 파일을 읽어 Machbase Neo 태그 테이블에 입력하는 명령행 도구입니다.

## 사용법

```sh
kwater import -dir <dir> -db <host:port> -user <user> -password <password> -table <table> [-c <n>] [-ignore-low-confidence <n>]
kwater dryrun -dir <dir> [-c <n>] [-ignore-low-confidence <n>]
```

입력 예:

```sh
kwater import -dir ./test/data -db 127.0.0.1:5656 -user sys -password manager -table kwdam -c 10 -ignore-low-confidence -1
```

검증 예:

```sh
kwater dryrun -dir ./test/data -c 10 -ignore-low-confidence 90
```

옵션:

- `-dir`: CSV 파일이 있는 디렉터리입니다. 디렉터리 안의 CSV 파일은 파일명 기준으로 정렬하여 처리합니다.
- `-db`: Machbase Neo 서버 주소입니다. 예: `127.0.0.1:5656`
- `-user`: 데이터베이스 사용자입니다. 기본값은 `sys`입니다.
- `-password`: 데이터베이스 비밀번호입니다. 기본값은 `manager`입니다.
- `-table`: 입력 대상 태그 테이블 이름입니다.
- `-c`: 동시에 처리할 CSV 파일 수입니다. 기본값은 `10`입니다.
- `-ignore-low-confidence <n>`: CSV 레코드의 `CONFIDENCE <= n`이면 해당 레코드를 무시합니다. 무시된 레코드는 데이터베이스에 입력되지 않습니다. 이 옵션을 명시하지 않으면 최소 정수값이 기본값으로 사용되므로 confidence 값과 관계없이 모든 레코드를 입력합니다.

`dryrun`은 `import`와 동일한 CSV 파싱, 파일 정렬, 동시 처리, confidence 필터링, 진행률 표시, 최종 요약을 사용합니다. 단, Machbase Neo에는 연결하지 않고 원본 파일 형식만 검사합니다. 잘못된 레코드가 있으면 파일명, 라인 번호, 원본 내용, 파싱 오류를 출력합니다. 잘못된 레코드가 하나라도 있으면 `dryrun`은 종료 코드 `1`로 종료합니다.

명령에는 반드시 `import` 또는 `dryrun` 하위 명령을 포함해야 합니다. 다음 명령은 잘못된 예입니다.

```sh
kwater -dir ./test/data -db 127.0.0.1:5656 -table kwdam
```

아래처럼 실행해야 합니다.

```sh
kwater import -dir ./test/data -db 127.0.0.1:5656 -table kwdam
kwater dryrun -dir ./test/data
```

## CSV 형식

각 CSV 파일은 다음 순서의 필드를 가져야 합니다.

```csv
NAME,TIME,VALUE,CONFIDENCE
ADD1AIG01ACTI01,2016-04-28 04:52:00,0,100
```

필드 설명:

- `NAME`: 태그 이름입니다. varchar로 저장됩니다.
- `TIME`: 시각입니다. `Asia/Seoul` 타임존에서 `YYYY-MM-DD HH:MM:SS` 형식으로 파싱합니다.
- `VALUE`: float64 값입니다. 이 필드가 비어 있거나 생략되었거나 double 값으로 파싱할 수 없으면, 해당 레코드를 오류로 처리하지 않고 `value`에 `NULL`을 할당합니다.
- `CONFIDENCE`: 정수 confidence 값입니다.

첫 번째 라인이 NAME,TIME,VALUE,CONFIDENCE 형식이면 헤더로 판단하여 건너뜁니다.

## 대상 테이블 예

```sql
create tag table kwdam (
    name varchar(80) primary key,
    time datetime base time,
    value double,
    conf int
) TAG_DUPLICATE_CHECK_DURATION=1440;
```

- `TAG_DUPLICATE_CHECK_DURATION` 단위는 분입니다. 예를 들어 `1440`은 24시간입니다.

## 개발

단위 테스트 실행:

```sh
go test ./...
```

`127.0.0.1:5656`에서 실행 중인 Neo 서버를 대상으로 로컬 통합 테스트 실행:

```sh
KWATER_INTEGRATION=1 go test ./... -run TestIntegrationImportKWDam -count=1
```

로컬 빌드:

```sh
go build -o kwater .
```

## 릴리스

`main` 브랜치에 push하면 테스트를 실행합니다.

`v0.1.0` 같은 태그를 push하면 테스트를 실행한 뒤 아래 실행 파일을 빌드하고, 이 README와 함께 `neo-kwater-<tag>.zip`으로 패키징하여 GitHub Release에 업로드합니다.

- `kwater-linux-amd64`
- `kwater-darwin-arm64`
- `kwater-windows-amd64.exe`
