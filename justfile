set shell := ["pwsh", "-NoProfile", "-Command"]

build:
    $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o ./out/who-is-spy-be main.go
    
deploy:
    scp -r ./out/who-is-spy-be b2:/root/who-is-spy-be