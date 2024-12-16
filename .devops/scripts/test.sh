export PATH="$PATH:$(go env GOPATH)/bin"
go mod tidy
# ginkgo -r
go run github.com/onsi/ginkgo/v2/ginkgo -r --fail-on-pending --fail-on-empty --keep-going --cover --coverprofile=cover.profile --race --trace ---github-output --poll-progress-after=10s --poll-progress-interval=10s .
