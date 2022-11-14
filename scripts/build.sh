dest=./dest
rm -rf $dest

mkdir $dest

GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/allinone cmd/allinone/main.go
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/customerserver cmd/customerserver/main.go
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/customeruserserver cmd/customeruserserver/main.go
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/servicerserver cmd/servicerserver/main.go
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/serviceruserserver cmd/serviceruserserver/main.go

GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/wscustomer cmd/ws-be/customer/main.go
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/wsservicer cmd/ws-be/servicer/main.go

upx --brute $dest/allinone
upx --brute $dest/customerserver
upx --brute $dest/customeruserserver
upx --brute $dest/servicerserver
upx --brute $dest/serviceruserserver

upx --brute $dest/wscustomer
upx --brute $dest/wsservicer
