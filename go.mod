module github.com/mwat56/reprox

go 1.23.0

toolchain go1.23.5

require (
	github.com/mwat56/apachelogger v1.7.0
	github.com/mwat56/ratelimit v0.2.1
)

replace (
	github.com/mwat56/apachelogger => ../apachelogger
	github.com/mwat56/ratelimit => ../ratelimit
	github.com/mwat56/reprox => ../reprox
)
