# vim: set ft=make ffs=unix fenc=utf8:
# vim: set noet ts=4 sw=4 tw=72 list:
#
all: freebsd linux man

freebsd: validate
	@env GOOS=freebsd GOARCH=amd64 go install -ldflags "-X main.zkmutexVersion=`git rev-parse --short HEAD`"

linux: validate
	@env GOOS=linux GOARCH=amd64 go install -ldflags "-X main.zkmutexVersion=`git rev-parse --short HEAD`"

man: freebsd linux
	@${GOPATH}/bin/zkmutex --create-manpage > zkmutex.1

validate:
	@golint .
	@go vet .
	@go tool vet -shadow .
	@ineffassign .
	@codecoroner funcs .
	@misspell .
