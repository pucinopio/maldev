//go:build never
// +build never

// This file exists solely to host the go:generate directives that build the
// embedded probe binaries. The "never" build tag excludes it from regular
// compilation — only `go generate` invokes the directives.
//
//go:generate env GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../linux-amd64 .
//go:generate env GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -C gen -o ../linux-arm64 .
//go:generate env GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../darwin-amd64 .
//go:generate env GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -C gen -o ../darwin-arm64 .
//go:generate env GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -C gen -o ../windows-amd64.exe .

package agents
