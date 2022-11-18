export PATH="$PATH:$(go env GOPATH)/bin"
go mod tidy
ginkgo -r