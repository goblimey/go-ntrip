The rtcm package contains logic to read and decode and display RTCM3
messages produced by GNSS devices.  See the README for this repository
for a description of the RTCM version 3 protocol.

     import (
         "github.com/goblimey/go-ntrip/rtcm/handler"
     )

imports the code for handling RTCM messages.

     rtcmHandler := handler.New(time.Now(), logger)

 creates an RTCM handler connected to a logger.

To process data, create a buffered channel of bytes for input
and a buffered channel of RTCM messages for output and start the handler:

    ch_source := make(chan byte, channelSourceCapacity)

    ch_result := make(chan Message, channelResultCapacity)

    go rtcmHandler.HandleMessages(ch_source, ch_result)

The handler reads bytes from the input, converts them to RTCM messages, each with a message type, and sends the messages to the message channel.
Any interspersed data that's not in RTCM format is returned as a special type NonRTCMMessage.
The handler runs until ch_source is closed.
If the bit stream is coming from a live GNSS device this may never happen
and the handler will run until the application is forcibly shut down.

Some RTCM messages
 contain a timestamp,
 milliseconds from the start of some period.
 at the end of the current period,
 the timestamp rolls over to zero and
 another period starts.  To make sense of the
 timestamps, the handler needs a date within the period in which the data
 was collected.  If the handler is receiving live data, the current
 date and time can be used, as in the example.

Each rtcm.Message object contains the raw data of the message
and the message type.  The raw data is binary and tightly
encoded.  The String method displays the data in a readable format.

    message.String()

Most message types are displayed as a hex dump with the message type.  Accurate positioning uses message type 1005 (base station position)
and the Multiple Signal Messages or MSMs (observations of signal from satellites).
As well as producing a hex dump of the data,
the String method decodes these messages in greater detail,
showing the various fields and their meaning.
For example, an MSM gives
the transit time of each signal from the
satellite to the base station in three fields which have to be combined together.
This is represented in the display as a distance in metres.
In this example,
two types of signal were observed from 8 satellites
at 5 seconds past midnight UTC on the 13th November 2020:

    2020-11-13 00:00:05 +0000 UTC
    message type 1077, frame length 226
    00000000  d3 00 dc 43 50 00 67 00  97 62 00 00 08 40 a0 65  |...CP.g..b...@.e|
    00000010  00 00 00 00 20 00 80 00  6d ff a8 aa 26 23 a6 a2  |.... ...m...&#..|
    00000020  23 24 00 00 00 00 36 68  cb 83 7a 6f 9d 7c 04 92  |#$....6h..zo.|..|
    00000030  fe f2 05 b0 4a a0 ec 7b  0e 09 27 d0 3f 23 7c b9  |....J..{..'.?#|.|
    00000040  6f bd 73 ee 1f 01 64 96  f5 7b 27 46 f1 f2 1a bf  |o.s...d..{'F....|
    00000050  19 fa 08 41 08 7b b1 1b  67 e1 a6 70 71 d9 df 0c  |...A.{..g..pq...|
    00000060  61 7f 19 9c 7e 66 66 fb  86 c0 04 e9 c7 7d 85 83  |a...~ff......}..|
    00000070  7d ac ad fc be 2b fc 3c  84 02 1d eb 81 a6 9c 87  |}....+.<........|
    00000080  17 5d 86 f5 60 fb 66 72  7b fa 2f 48 d2 29 67 08  |.]..`.fr{./H.)g.|
    00000090  c8 72 15 0d 37 ca 92 a4  e9 3a 4e 13 80 00 14 04  |.r..7....:N.....|
    000000a0  c0 e8 50 16 04 c1 40 46  17 05 41 70 52 17 05 01  |..P...@F..ApR...|
    000000b0  ef 4b de 70 4c b1 af 84  37 08 2a 77 95 f1 6e 75  |.K.pL...7.*w..nu|
    000000c0  e8 ea 36 1b dc 3d 7a bc  75 42 80 00 00 00 00 00  |..6..=z.uB......|
    000000d0  00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 fe  |................|
    000000e0  69 e8                                             |i.|

    type 1077 GPS Full Pseudoranges and PhaseRanges plus CNR (high resolution)
    stationID 0, timestamp 432023000, multiple message, sequence number 0
    session transmit time 0, clock steering 0, external clock 0
    divergence free smoothing false, smoothing interval 0
    8 satellites, 2 signal types, 16 signals
    Satellite ID {approx range m, extended info, phase range rate}:
     4 {24410542.339, 0, -135}
     9 {25264833.738, 0, 182}
    16 {22915678.774, 0, 597}
    18 {21506595.669, 0, 472}
    25 {23345166.602, 0, -633}
    26 {20661965.550, 0, 292}
    29 {21135953.821, 0, -383}
    31 {21670837.435, 0, -442}
    Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier  Noise Ratio, phase range rate}:
     4  2 {24410527.355, 128278179.264, 582, false, 640, -135.107}
     4 16 {24410523.313, 99956970.352, 581, false, 608, -135.107}
     9 16 {25264751.952, 103454935.508, 179, false, 464, 182.123}
    16  2 {22915780.724, 120423177.179, 529, false, 640, 597.345}
    18  2 {21506547.550, 113017684.727, 579, false, 704, 472.432}
    18 16 {21506542.760, 88065739.822, 578, false, 608, 472.418}
    25  2 {23345103.037, 122679365.321, 646, false, 640, -633.216}
    25 16 {23345100.838, 95594272.692, 623, false, 560, -633.187}
    26  2 {20662003.308, 108579565.367, 596, false, 736, 292.755}
    26 16 {20662000.914, 84607418.613, 596, false, 672, 292.749}
    29  2 {21136079.188, 111070868.860, 628, false, 736, -383.775}
    29 16 {21136074.598, 86548719.034, 628, false, 656, -383.770}
    31  2 {21670772.711, 113880577.055, 624, false, 736, -442.539}
    31 16 {21670767.783, 88738155.231, 624, false, 640, -442.550}

