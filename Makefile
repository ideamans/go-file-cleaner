HOME := $(shell echo $$HOME)

file-cleaner: *.go
	go build -o file-cleaner *.go

linux: *.go
	GOOS=linux GOARCH=amd64 go build -o file-cleaner *.go

run: *.go
	time go run *.go -c 12 /$(HOME)/.anyenv 80gb

test: *.go
	go test -v