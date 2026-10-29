module github.com/graydraft/graydraft

go 1.24

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/graydraft/gocommon v0.0.0
)

replace github.com/graydraft/gocommon => ../../pkg/gocommon
