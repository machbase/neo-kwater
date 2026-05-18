- 이 프로그램은 windows, linux와 macOS에서 실행한다.
- 이 프로그램은 cmd line에서 입력받은 directory에 존재하는 <tagname>.csv 파일을 읽어서 machbase-neo db table에 입력한다.
- syntax: `kwater import -dir <dir> -db 192.168.1.100:5656 -user <sys> -password <manager> -table <table> -c <n>`
- machbase 에 연결하려면 neo-client의 append를 사용해야 한다. https://github.com/machbase/neo-client
- neo-client의 append 예시는 https://github.com/machbase/neo-client/blob/main/_example/append.go 를 참고한다.
- `-c <n>`은 concurrent로 처리해야할 파일 수이고 default n = 10이다.
- `-dir <dir>` directory내의 파일을 이름순으로 정렬하여 순서대로 처리한다.
- 동시에 읽을 파일의 수는 -c 를 따르고 하나의 connection과 하나의 appender만 생성하여 다수의 입력 stream을 chan으로 처리한다.
- csv는 디렉터리에 tag별로 <tagname>.csv 로 존재하며 파일의 갯수는 n*10,000 수만개 수준이다.
- CSV는 NAME, TIME, VALUE, CONFIDENCE 로 되어 있다.
    `ADD1AIG01ACTI01,2016-04-28 04:52:00,0,100`
이 중에 name은 varchar이고 time의 time location은 Asia/Seoul이다. value는 float64이고, confidence는 int 로 처리한다.
- 이 프로그램은 실행 중에 첫 라인에는 완료파일수/대상파일수(1000단위 콤마(,)를 표현)를 표시한다.
- 화면에 완료한 파일과 현재 처리 중인 파일의 목록을 아래와 같이 prgress로 표현한다.

```
Total 2 of 20,000 files processed.
./dir/file1.csv ################### 12,345 lines Done
./dir/file2.csv ################### 23,456 lines Done
./dir/file3.csv ######............. 34,567/123,456 lines processing
./dir/file4.csv ###................ 34,567/123,456 lines processing
```