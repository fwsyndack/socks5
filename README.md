# socks5 server

run server: go run ss5.go

test: curl --socks5 127.0.0.1:1080 http://www.baidu.com
