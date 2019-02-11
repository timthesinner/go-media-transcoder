build: clean
	go build

arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o go-media-transcoder-arm;

windows:
	CGO_ENABLED=0 GOOS=windows go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o go-media-transcoder.exe;

tidy:
	go fmt
	go mod tidy
	go clean --modcache

clean:
	rm -f go-media-transcoder
	rm -f go-media-transcoder-arm
	rm -f go-media-transcoder.exe

all: clean build arm windows
	echo "Built all binaries"