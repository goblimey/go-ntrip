module github.com/goblimey/go-ntrip/rtcmlogger

go 1.14

require (
	github.com/goblimey/go-tools/switchWriter v0.0.0-00010101000000-000000000000
	github.com/robfig/cron v1.2.0
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/tools v0.0.0-20200326210457-5d86d385bf88 // indirect
)

replace github.com/goblimey/go-tools/switchWriter => ../../go-tools/switchWriter
