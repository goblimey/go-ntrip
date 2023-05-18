// displayrtcm3 reads bytes from stdin or a file, ignores anything
// that's not in RTCM version 3 format and writes a readable version of
// the messages to the standard output channel.
//
// Raw RTCM3 is a tightly compressed binary format, not designed to be
// readable by a human.  The standard defines a large number of message
// types, all in different binary formats.
//
// The most important message types for accurate GNSS systems are message
// type 1005, which gives the position of the base station, and Multiple
// Signal Messages, either type 4 (MSM4) or type 7 (MSM7).  These both
// contain the base station's observations of satellite signals.  Type 4
// messages are of sufficient precision for 2-cm accurate positioning.
// Type 7 messages are of even high precision.  A GNSS device is typically
// configured to emit messages of type 1005 plus either MSM4 or MSM7
// messages for each of the constellations that it can handle.
//
// The tool interprets messages of type 1005 and MSMs as a hex dump and
// then as plain text so they can be easily understood.  It handles MSMs
// for GPS, Galileo, GLONASS and BeiDou satellites.  These can be observed
// from anywhere in the world given an open view of the sky.  MSMs for
// constellations are dumped in hex but not interpreted.
//
// For example:
//
//	2023-02-14 01:02:03.004 +0000 UTC
//	message type 1077, frame length 838
//	00000000  43 50 00 67 00 97 62 00  00 08 40 a0 65 00 00 00  |CP.g..b...@.e...|
//	00000010  00 20 00 80 00 6d ff a8  aa 26 23 a6 a2 23 24 00  |. ...m...&#..#$.|
//	00000020  00 00 00 36 68 cb 83 7a  6f 9d 7c 04 92 fe f2 05  |...6h..zo.|.....|
//	00000030  b0 4a a0 ec 7b 0e 09 27  d0 3f 23 7c b9 6f bd 73  |.J..{..'.?#|.o.s|
//	00000040  ee 1f 01 64 96 f5 7b 27  46 f1 f2 1a bf 19 fa 08  |...d..{'F.......|
//	00000050  41 08 7b b1 1b 67 e1 a6  70 71 d9 df 0c 61 7f 19  |A.{..g..pq...a..|
//	00000060  9c 7e 66 66 fb 86 c0 04  e9 c7 7d 85 83 7d ac ad  |.~ff......}..}..|
//	00000070  fc be 2b fc 3c 84 02 1d  eb 81 a6 9c 87 17 5d 86  |..+.<.........].|
//	00000080  f5 60 fb 66 72 7b fa 2f  48 d2 29 67 08 c8 72 15  |.` + "`" + `.fr{./H.)g..r.|
//	00000090  0d 37 ca 92 a4 e9 3a 4e  13 80 00 14 04 c0 e8 50  |.7....:N.......P|
//	000000a0  16 04 c1 40 46 17 05 41  70 52 17 05 01 ef 4b de  |...@F..ApR....K.|
//	000000b0  70 4c b1 af 84 37 08 2a  77 95 f1 6e 75 e8 ea 36  |pL...7.*w..nu..6|
//	000000c0  1b dc 3d 7a bc 75 42 80  00 00 00 00 00 00 00 00  |..=z.uB.........|
//	000000d0  00 00 00 00 00 00 00 00  00 00 00 00 fe 69 e8 6a  |.............i.j|
//	000000e0  d3 00 c3 43 f0 00 a2 93  7c 22 00 00 04 0e 03 80  |...C....|"......|
//	000000f0  00 00 00 00 20 80 00 00  7f fe 9c 8a 80 94 86 84  |.... ...........|
//	00000100  99 0c a0 95 2a 8b d8 3a  92 f5 74 7d 56 fe b7 ec  |....*..:..t}V...|
//	00000110  e8 0d 41 69 7c 00 0e f0  61 42 9c f0 27 38 86 2a  |..Ai|...aB..'8.*|
//	00000120  da 62 36 3c 8f eb c8 27  1b 77 6f b9 4c be 36 2b  |.b6<...'.wo.L.6+|
//	00000130  e4 26 1d c1 4f dc d9 01  16 24 11 9a e0 91 02 00  |.&..O....$......|
//	00000140  7a ea 61 9d b4 e1 52 f6  1f 22 ae df 26 28 3e e0  |z.a...R.."..&(>.|
//	00000150  f6 be df 90 df b8 01 3f  8e 86 bf 7e 67 1f 83 8f  |.......?...~g...|
//	00000160  20 51 53 60 46 60 30 43  c3 3d cf 12 84 b7 10 c4  | QS` + "`F`" + `0C.=......|
//	00000170  33 53 3d 25 48 b0 14 00  00 04 81 28 60 13 84 81  |3S=%H......(` + "`" + `...|
//	00000180  08 54 13 85 40 e8 60 12  85 01 38 5c 67 b7 67 a5  |.T..@.` + "`" + `...8\g.g.|
//	00000190  ff 4e 71 cd d3 78 27 29  0e 5c ed d9 d7 cc 7e 04  |.Nq..x').\....~.|
//	000001a0  f8 09 c3 73 a0 40 70 d9  6d 6a 75 6e 6b d3 00 c3  |...s.@p.mjunk...|
//	000001b0  44 90 00 67 00 97 62 00  00 21 18 00 c0 08 00 00  |D..g..b..!......|
//	000001c0  00 20 01 00 00 7f fe ae  be 90 98 a6 9c b4 00 00  |. ..............|
//	000001d0  00 08 c1 4b c1 32 f8 0b  08 c5 83 c8 01 e8 25 3f  |...K.2........%?|
//	000001e0  74 7c c4 02 a0 4b c1 47  90 12 86 62 72 92 28 53  |t|...K.G...br.(S|
//	000001f0  18 9d 8d 85 82 c6 e1 8a  6a 2f dd 5e cd d3 e1 1a  |........j/.^....|
//	00000200  15 01 a1 2b dc 56 3f c4  ea c0 5e dc 40 48 d3 80  |...+.V?...^.@H..|
//	00000210  b2 25 60 9c 7b 7e 32 dd  3e 22 f7 01 b6 f3 81 af  |.%` + "`" + `.{~2.>"......|
//	00000220  b7 1f 78 e0 7f 6c aa fe  9a 7e 7e 94 9f bf 06 72  |..x..l...~~....r|
//	00000230  3f 15 8c b1 44 56 e1 b1  92 dc b5 37 4a d4 5d 17  |?...DV.....7J.].|
//	00000240  38 4e 30 24 14 00 04 c1  50 3e 0f 85 41 40 52 13  |8N0$....P>..A@R.|
//	00000250  85 61 50 5a 16 04 a1 38  12 5b 24 7e 03 6c 07 89  |.aPZ...8.[$~.l..|
//	00000260  db 93 bd ba 0d 34 27 68  75 d0 a6 72 24 e4 88 dc  |.....4'hu..r$...|
//	00000270  61 a9 40 b1 9d 0d d3 00  aa 46 70 00 66 ff bc a0  |a.@......Fp.f...|
//	00000280  00 00 00 04 00 26 18 00  00 00 20 02 00 00 75 53  |.....&.... ...uS|
//	00000290  fa 82 42 62 9a 80 00 00  06 95 4e a7 a0 bf 1e 78  |..Bb......N....x|
//	000002a0  7f 0a 10 08 18 7f 35 04  ab ee 50 77 8a 86 f0 51  |......5...Pw...Q|
//	000002b0  f1 4d 82 46 38 29 0a 8c  35 57 23 87 82 24 2a 01  |.M.F8)..5W#..$*.|
//	000002c0  b5 40 07 eb c5 01 37 a8  80 b3 88 03 23 c4 fc 61  |.@....7.....#..a|
//	000002d0  e0 4f 33 c4 73 31 cd 90  54 b2 02 70 90 26 0b 42  |.O3.s1..T..p.&.B|
//	000002e0  d0 9c 2b 0c 02 97 f4 08  3d 9e c7 b2 6e 44 0f 19  |..+.....=...nD..|
//	000002f0  48 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |H...............|
//	00000300  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
//	00000310  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00  |................|
//	00000320  00 00 00 e5 1e d8 d3 00  aa 46 70 00 66 ff bc a0  |.........Fp.f...|
//	00000330  00 00 00 04 00 26 18 00  00 00 20 02 00 00 75 53  |.....&.... ...uS|
//	00000340  fa 82 42 62 9a 80                                 |..Bb..|
//
//	type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
//	stationID 0, timestamp 432023000, multiple message, sequence number 0
//	session transmit time 0, clock steering 0, external clock 0
//	divergence free smoothing false, smoothing interval 0
//	8 satellites, 2 signal types, 16 signals
//	Satellite ID {approx range m, extended info, phase range rate}:
//
//	 4 {24410542.339, 0, -135}
//	 9 {25264833.738, 0, 182}
//
//	16 {22915678.774, 0, 597}
//	18 {21506595.669, 0, 472}
//	25 {23345166.602, 0, -633}
//	26 {20661965.550, 0, 292}
//	29 {21135953.821, 0, -383}
//	31 {21670837.435, 0, -442}
//	Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
//
//	 4  2 {24410527.355, 128282115.527, 513, false, 80, -136.207}
//	 4 16 {24410523.313, 99955313.523, 320, false, 82, -134.869}
//	 9 16 {25264751.952, 103451227.387, 606, false, 78, 182.267}
//
//	16  2 {22915780.724, 120426622.169, 40, true, 86, 597.233}
//	18  2 {21506547.550, 113021883.224, 968, false, 84, 471.599}
//	18 16 {21506542.760, 88061705.706, 37, false, 90, 472.270}
//	25  2 {23345103.037, 122677706.869, 51, true, 88, -632.317}
//	25 16 {23345100.838, 95594616.762, 78, false, 74, -634.411}
//	26  2 {20662003.308, 108581645.522, 329, false, 78, 291.597}
//	26 16 {20662000.914, 84606022.946, 80, false, 18, 290.429}
//	29  2 {21136079.188, 111074207.143, 664, false, 364, -382.650}
//	29 16 {21136074.598, 86547263.526, 787, false, 583, -382.997}
//	31  2 {21670772.711, 113885408.778, 710, true, 896, -443.036}
//	31 16 {21670767.783, 88736342.592, 779, false, 876, -441.969}
//
// Any other messages types are also shown, but only as a hex dump
// plus the message type.
//
// The tool is useful for trouble-shooting, particularly when you
// have a misbehaving base station and you are trying to figure out
// what it's doing.  You can see what types of messages it's sending,
// if any.  if it;s send MSMs you can see which satellites it's
// received signals from, which signal types they were and what data
// the signals contained.
//
// A GNSS base station will typically be configured to emit a batch
// of messages every second, so the tool will produce A LOT of output
// if run for any length of time.
//
// Usage:
//
//	displayrtcm3 file date
//
// Examples:
//
//		displayrtcm3 testdata.rtcm 2020-11-13
//
//	 displayrtcm3 - 2020-11-13 # take input from the standard input channel.
//
// The RTCM data may contain other messages and these are displayed in
// "od" format - hex values and readable text.  They are mostly ASCII
// strings, for example NMEA messages, so they should be fairly readable.
//
// The program takes one argument which should be in the format
// "yyyy-mm-dd".  This is turned into a date/time at midnight UTC on
// that day.  This is used to figure out the start of the various
// GNSS weeks - the GPS week and so on.
//
// Each Multiple Signal Message (MSM) contains a timestamp.  The
// timestamps in the input data should relate to the current GNSS weeks.
// The timestamp is represented as milliseconds since some start date, in
// most cases the start of the week, which is midnight at the start of
// Sunday in some time zone.  The timestamp rolls over to zero at the
// start of the next period, so the tool needs to know which period it's
// handling.
//
// If the tool runs for many days, it handles the week rolling over into
// the next week, and the next, and so on.
//
// The GPS timestamp rolls over a few seconds BEFORE midnight at the
// start of Sunday in UTC (since 2021, 18 seconds before midnight).
// Galileo uses the same time as GPS.  The Beidou timestamp rolls
// over 14 seconds AFTER midnight on Sunday.  The GLONASS timestamp
// uses the Moscow timezone, which is 3 hours ahead of UTC.  It
// contains two fields representing the day of the week and milliseconds
// since the start of the day.  Day 0 is Sunday which starts at 21:00 on
// Saturday evening in UTC.  Day 7 is an illegal value and should
// never occur.
package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"time"

	AppCore "github.com/goblimey/go-ntrip/apps/appcore"
	// handler "github.com/goblimey/go-ntrip/file_handler"
	"github.com/goblimey/go-ntrip/jsonconfig"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

func main() {

	var startTime time.Time
	var reader io.Reader
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s file yyyy-mm-dd", os.Args[0])
	}
	appName := os.Args[0]

	// The format of arg[2] should be yyyy-mm-dd.  arg[3] is a file containing RTCM messages.
	var timeError error
	startTime, timeError = getTime(os.Args[2])
	if timeError != nil {
		log.Printf("usage: %s file yyyy-mm-dd", appName)
		log.Fatalf(timeError.Error())
	}

	fileName := os.Args[1]
	reader, openError := openFile(fileName)
	if openError != nil {
		log.Fatalf("%s: cannot open %s - %v", appName, fileName, openError)
	}

	// We just need the EOF timeout to be zero, which causes HandleMessages
	// to stop when the input file is exhausted, so the zero value of the
	// config is suitable.
	var config jsonconfig.Config

	HandleMessages(startTime, reader, os.Stdout, &config)

	os.Exit(0)
}

func HandleMessages(startTime time.Time, reader io.Reader, writer io.Writer, config *jsonconfig.Config) {

	bufferedReader := bufio.NewReader(reader)
	// Write the heading.
	writer.Write([]byte("RTCM data\n"))
	writer.Write([]byte("\nNote: times are in UTC.  RINEX format uses GPS time, which is currently (Jan 2021)\n"))
	writer.Write([]byte("18 seconds ahead of UTC\n\n"))

	messageChan := make(chan rtcm.Message, 2)
	go DisplayMessages(messageChan, writer)

	channels := make([]chan rtcm.Message, 0)
	channels = append(channels, messageChan)
	appCore := AppCore.New(config, channels)
	appCore.HandleMessagesUntilEOF(startTime, bufferedReader)

	close(messageChan)
}

// DisplayMessages receives messages from the given channel, produces a
// readable display of each and writes them to the writer.  It can be
// run in a goroutine.
func DisplayMessages(messageChan chan rtcm.Message, writer io.Writer) error {
	for {
		message, ok := <-messageChan
		if !ok {
			return nil
		}
		// Decode the message.  (The result is very verbose!)
		display := message.String() + "\n"
		_, writeError := writer.Write([]byte(display))
		if writeError != nil {
			return writeError
		}
	}
}

// getTime gets a time from a string in one of three formats,
// yyyy-mm-dd{:hh:mm:ss{:timezone}}".  Timezones are listed
// here:  https://en.wikipedia.org/wiki/List_of_tz_database_time_zones.
// Note that three letter abbreviations such as "CET" are deprecated.
func getTime(timeStr string) (time.Time, error) {
	const dateLayout = "2006-01-02"

	if len(dateLayout) == len(timeStr) {
		dateTime, err := time.Parse(dateLayout, timeStr)
		return dateTime, err
	} else {
		dateTime, err := time.Parse(time.RFC3339, timeStr)
		return dateTime, err
	}
}

// openFile opens the given file and returns a Reader connected
// to it.  If the file name is "-" it returns os.Stdin
func openFile(fileName string) (io.Reader, error) {
	if fileName == "-" {
		return os.Stdin, nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	return file, nil
}
