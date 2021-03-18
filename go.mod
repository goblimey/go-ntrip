module github.com/goblimey/go-ntrip

go 1.14

require github.com/goblimey/go-ntrip/rtcmlogger v0.0.0-20200825104214-225f47a7ebb9 // indirect

require (
	github.com/goblimey/go-crc24q v0.0.0-20210107174841-6ea518daa3aa // indirect
	github.com/goblimey/go-ntrip/rtcm v0.0.0-0
)

replace github.com/goblimey/go-ntrip/rtcm => ./rtcm
