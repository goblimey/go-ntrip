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
the message is type 1087, a Glonass Multiple Signal Message.
It was sent on the 1st January 2021 in the early morning. 

    Message type 1097, Galileo Full Pseudoranges and PhaseRanges plus Carrier to Noise Ratio (high resolution)
    The type 7 Multiple Signal Message format for Europe’s Galileo system.
    Frame length 201 bytes:
    00000000  d3 00 c3 44 90 00 67 00  97 66 00 00 50 07 04 10  |...D..g..f..P...|
    00000010  00 00 00 00 20 01 00 00  7f fe b8 b6 a0 98 9c a4  |.... ...........|
    00000020  a6 00 00 00 02 9f e8 36  c2 c0 c0 cd 3c 83 c2 0e  |.......6....<...|
    00000030  b8 23 62 76 7e 6a 01 3f  c3 d8 12 29 7e b9 62 70  |.#bv~j.?...)~.bp|
    00000040  62 27 51 c3 f1 62 3e 7d  66 1d c6 5f c6 df 83 3d  |b'Q..b>}f.._...=|
    00000050  f7 8c 80 94 3c 07 d2 04  ca 1a 4c 25 dd ff 96 fd  |....<.....L%....|
    00000060  fa 5e 40 97 65 80 a1 39  c1 26 6a c1 11 c2 c1 89  |.^@.e..9.&j.....|
    00000070  06 61 7e f4 bf e9 cf 9f  f9 73 e0 08 91 00 19 2c  |.a~......s.....,|
    00000080  81 30 44 41 23 5c 30 74  1c fd 3e b3 04 c2 da 36  |.0DA#\0t..>....6|
    00000090  93 f4 fd 33 cd 14 15 04  00 04 e1 50 52 14 85 c1  |...3.......PR...|
    000000a0  88 4e 15 85 a1 68 5e 17  85 81 68 0c d4 1a 6a 82  |.N...h^...h...j.|
    000000b0  0d 04 70 ff 11 fe 00 89  c1 0e 6c 3e d8 c2 3a 04  |..p.......l>..:.|
    000000c0  73 e5 be db 70 60 71 93  5a                       |s...p`q.Z|
    
    Sent at 2021-01-01 00:00:05.001 +0000 UTC
    Start of Galileo week 2020-12-26 23:59:42 +0000 UTC plus timestamp 432023001 (5d 0h 0m 23s 1ms)
    stationID 0, multiple message, issue of data station 0
    session transmit time 0, clock steering 0, external clock 0
    divergence free smoothing false, smoothing interval 0
    7 satellites, 2 signal types, 14 signals
    Satellite ID {approx range m, extended info, phase range rate}:
     1 {27605205.720, 0, 481}
     3 {27577392.943, 0, 471}
    13 {24015308.142, 0, 283}
    14 {22940563.891, 0, 1260}
    15 {23390838.110, 0, -203}
    21 {24612843.695, 0, 39}
    27 {25068094.938, 0, -482}
    Signals: sat ID sig ID {range m, phase range, lock time ind, half cycle ambiguity, Carrier Noise Ratio, phase range rate}:
     1  2 {27605060.635, 145065565.311, 526, false, 624, 481.328}
     1 15 {27605057.878, 111154128.448, 526, false, 672, 481.338}
     3  2 {27577437.572, 144920405.511, 506, false, 656, 471.832}
     3 15 {27577437.909, 111042919.434, 501, false, 656, 471.833}
    13  2 {24015380.289, 126201738.417, 608, false, 736, 283.408}
    13 15 {24015379.606, 96700009.553, 609, false, 784, 283.408}
    14  2 {22940675.806, 120554067.409, 436, false, 624, 1260.055}
    14 15 {22940673.422, 92372585.514, 436, false, 688, 1260.054}
    15  2 {23390829.192, 122919650.568, 638, false, 720, -203.506}
    15 15 {23390828.445, 94185204.806, 638, false, 720, -203.502}
    21  2 {24612854.291, 129341379.472, 615, false, 752, 39.371}
    21 15 {24612852.639, 99105751.442, 616, false, 752, 39.371}
    27  2 {25068182.575, 131734185.169, 642, false, 704, -482.923}
    27 15 {25068182.021, 100939165.985, 642, false, 720, -482.934}

The satellites broadcast different signal types on different
frequency bands.
My receiver is dual band,
meaning that it listens for signals on two frequency bands.
In this case it received type 2 signals and type 15 signals
from 7 satellites.
Each satellite has an ID and in this case
satellites 1, 3, 13, 14, 15, 21 and 27 were in view.
The receiver doesn't always pick up every signal
but this time it did,
14 in all.

In the signal list, the first number is the distance to the satellite in metres.
This is derived from the transit time.
Notice that the distance is a little different for the two signals
even though they are coming from the same satellite
and were sent at the same time.
That's because each signal was distorted by the ionosphere.
Different frequencies are distorted differently
so the signals traveled through two different paths,
one longer than the other.
The receiver analyses these discrepencies to figure out
the distortion and correct for it,
allowing it to better estimate its position.
