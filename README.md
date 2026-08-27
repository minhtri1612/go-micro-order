# go-micro-order

Split from monorepo go-micro (order-service/). Original go-micro folder is left untouched.

## Build

```bash
go test ./...
docker build -t minhtri1612/order-service:dev .
```

## CI

Thin Jenkinsfile -> Shared Library go-micro-ci (repo go-micro-pipeline-lib).