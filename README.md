## Go tools to support the NTRIP protocol

This repository contains Go software to support Network Transport of RTCM over IP (NTRIP) messages.

The RTCM protocol is named after the organisation that defined it, the Radio Technical Commission for Maritime services or RTCM.  It's used to carry observations of global positioning satellites from fixed base stations to moving rovers to allow the rovers to better find their positions.

There are a number of networks ("constellations") of global positioning satellites.
The American Global Positioning System (GPS),
the European Galileo, the Chinese Beidou and the Russian GLONASS constellations all provide a global service - some satellites from each of those constellations should be visible from any point on the Earth's surface.
Receivers are available that can use any and all of them.

Strictly these constellations are called Global Navigation Satellite Systems (GNSS) rather than GPS, which is just the first and best known. 

A GNSS device on the ground receives signals from the satellites and uses trigonometry to find its position.  It needs signals from 4 satellites to do that.  A multi-constellation receiver can use signals from all of the constellation.  In good conditions a receiver can see upwards of twenty satellites at any time.  

Given signals from four satellites a single receiver can find its position to within about 3 metres.
This is acceptable for vehicle navigation
but other purposes such as land surveying
require greater accuracy.  Seeing more satellites doesn't produce more accuracy,
but it does allow a faster fix.

More accuracy is possible using two receivers within a few kilometers of each other.
The signals from the positioning satellites suffer distortion, particularly as they pass through the ionosphere on their way to the Earth.
The receivers on the ground use these signals to figure out their position,
but the distortions produce inaccuracies.

Each RTCM base station is in a known location.  It scans all of the global positioning satellites that it can see for signals and passes the result to any rovers that have subscribed to receive it, along with its real position.
Rovers close to the base see the same satellites with the same distortions and they can use the information from the base to correct the calculation of their own position.
In ideal conditions a rover within about 8km of a base station can use the RTCM data to estimate its position to within 2cm.
Beyond 8km, the accuracy falls linearly -
within 16km a rover can attain an accuracy of 4cm,
within 24km an accuracy of 8cm, and so on.
At about 64 km the RTCM data only allows about 2.5m accuracy,
which is close to what the rover can achieve without help.

RTCM data can be carried over radio
or, using the NTRIP protocol, over the Internet.
The latter requires the rover and the base station to be connected to the network.
The amount of data is fairly low, a few kilobytes per second,
low enough to be carried by a domestic broadband connection or a mobile phone's Internet connection.

To avoid the rover or the base station needing a fixed Internet (IP) address,
they communicate via an intermediate server called an NTRIP caster (ie broadcaster).
In the simple case,
the caster offers a set of named endpoints, each fed by a base station.
A rover subscribes to an NTRIP feed from one endpoint.
The base station reads signals from the satellites that it can see,
encodes them as RTCM messages, packages those up into NTRIP messages
and sends them over the Internet to its assigned endpoint on the caster.
Te caster sends those messages on to any rovers that have subscribed to that endpoint:

A typical configuration is a GNSS receiver such as a Sparkfun
RTK board producing RTCM and other messages, connected via
I2C or serial USB to a host computer running this filter.  The
host could be a Windows machine but something as cheap as
a Raspberry Pi single board computer is quite adequate:

 -------------          messages          --------------
| GPS device  | -----------------------> | Raspberry Pi |
 -------------    serial connection       --------------

With a bit more free software that can be used to create an NTRIP
 base station:


  messages    ------------   RTCM    --------------   RTCM over NTRIP
-----------> | rtcmfilter | ------> | NTRIP server | ----------------->
              ------------   stdout  --------------      Internet

The base station sends RTCM corrections over the Internet using the
NTRIP protocol to an NTRIP caster and on to moving GNSS rovers:

                                              ---------> rover
 ------------                     --------   /
| NTRIP base |  RTCM over NTRIP  | NTRIP  | /
| station    | ----------------> | caster | \
 ------------                     --------   \
                                              ----------> rover


RTCM
=====

The RTCM protocol is currently at version 3.
The format of the messages is described by RTCM STANDARD 10403.3
Differential GNSS (Global Navigation Satellite Systems) Services –
Version 3 (RTCM3 10403.3).  This is not an open-source standard and
it costs about $300 to buy a copy.  RTCM messages are in a very
compact binary form and not readable by eye.

The NTRIP protocol defines simple wrapper to carry the messages over an Internet connection. It's an open-source standard, published for free by the RTCM. 

The RTCM standard defines a large number of message types,
each with its own format.  Fortunately most of them are redundant.
A complete correction service can be created using just six or seven types of message.

There is a little bit of useful information scattered around
various public web pages.  
There's also a library of C code to handle them, RTKLIB.
(I've copied some of
the more useful RTKLIB source files into this repository as a handy reference.)

There are already open-source tools available to convert an RTCM3 data stream into messages
in another format called RINEX.  That's an open standard and the result is human readable. 

To figure out the format of the RTCM message I'm
interested in, I read what I could find, including the RTKLIB source code.
Then I took the RTCM3 messages
that my device produced, used the tools to convert them into RINEX format and examined
the result. 
These data form some of my unit and integration tests.

The rtcm package in this repo is the main result.
It contains the logic to decode and display RTCM3
messages produced by GNSS devices such as the U-Blox ZED-F9P.  

The RTKLIB source code does a very good job of decoding RTCM messages,
but comments ar scant and the code is opaque in places.
Developers may find the source code in this repo easier
to understand.

An RTCM3 message is binary and variable length.  Each message frame
is composed of a three-byte header, an embedded message and 3 bytes of
Cyclic Redundancy Check (CRC) data.  The header starts with 0xd3 and
includes the length of the embedded message.  Each message starts with
a 12-bit message number which defines the type.  Apart from that
message number, each type of message is in a different format.
Fortunately, I only have to worry about a few types types.

This is a hex dump of a complete RTCM3 message frame and the
start of another:

```
d3 00 aa 44 90 00 33 f6  ea e2 00 00 0c 50 00 10
08 00 00 00 20 01 00 00  3f aa aa b2 42 8a ea 68
00 00 07 65 ce 68 1b b4  c8 83 7c e6 11 30 10 3f
05 ff 4f fc e0 4f 61 68  59 b6 86 b5 1b a1 31 b9
d9 71 55 57 07 a0 00 d3  2e 0c 99 01 98 c4 fa 16
0e fa 6e ac 07 19 7a 07  3a a4 fc 53 c4 fb ff 97
00 4c 6f f8 65 da 4e 61  e4 75 2c 4b 01 e5 21 0d
4f c0 0b 02 b0 b0 2f 0c  02 70 94 23 0b c3 e9 e0
97 d1 70 63 00 45 8d e9  71 d7 e5 eb 5f f8 78 00
00 00 00 00 00 00 00 00  00 00 00 00 00 00 00 00
00 00 00 00 00 00 00 00  00 00 00 00 00 4d f5 5a
d3 00 6d 46 40 00 33 f6  10 22 00 00 02 40 08 16
```

The message starts at byte zero.  That byte has the value d3, which
announces the start of the message frame.  The frame is composed of a
3-byte header, and embedded message and 3 bytes of Cyclic Rdundancy
Check (CRC) data.

Byte 0 of the frame is always d3.  The top six bits of byte 1 are
always zero.  The lower two bits of byte 1 and the bits of byte 2 form the 10-bit
message length, in this case hex 0aa, decimal 176.  So the embedded
message is 176 bytes long.  With the header and CRC,
the whole message frame is 182
bytes long.  As above, the embedded message may end with some
padding bits which are always zero.

The last three bytes of the frame (in this case 4d, f5 and 5a) are the
CRC value.  To check the CRC, take the header and the embedded message,
run the CRC calculation over those bytes and compare the result with
the given CRC.  If they are different then the message is not RTCM3 or
it's been corrupted in transit.

The CRC check is calculated using an algorithm from Qualcomm.  I use Mark
Rafter's implementation, also in this github account at https://github.com/goblimey/go-crc24q.

The first 12 bits of the embedded message give the message number, in
this case hex 449, decimal 1097, which is a type 7 Multiple Signal Message
(MSM7) containing high resolution observations of signals from Galileo
satellites.

The messages are binary and can contain a d3 byte.  Note the one on the
fifth line of the hex dump above.  This is not the start of another
message.  One clue is that it's not followed by six zero bits.  To extract
a message frame from a stream of data and decode it, you need to read the
header and the next two bytes, check the header, find the message length,
read the whole message frame and check the CRC value.  This matters
particularly when you start to receive a stream of data from a device.  You
may come into the data stream part-way through and blunder into a d3 byte.
You can't assume that it's the start of a message.

The CRC data is there to check that the message has not been corrupted in
transit.  If the CRC check fails, the mesage must be discarded.

RTCM3 message frames in the NTRIP data stream are contiguous with no separators or
newlines.  In the example, the last line contains the start of the next
message.  Other data in other formats may be interspersed between frames.
My rtcm software discards anything that's not an RTCM3 message frame with a
correct CRC value.

There are many hundreds of RTCM3 message types, some of which are just
different ways of representing the same information.  To get an accurate fix
on its position, a rover only needs to know the position of the base station
and a recent set of the base's observations of satellite signals, which is to
say a type 1005 message and a set of MSM7 or MSM4 messages, one for each constellation
of satellites (GPS, GLONASS or whatever).

Message type 1005 gives the position of the base station (or more strictly,
of a point in space a few centimetres above its antenna).

Message types 1077, 1087 and so on are Multiple Signal Messages
(MSMs) of type 7.  Types 1074, 1084 and so on are the equivalent MSM type 4 messages.
Each contains observations by the base station of signals from
satellites in one constellation.  Type 1077 is in MSM7 format and contains
high resolution signal data from GPS satellites.  Type 1074 is in MSM4 format
which is simply a lower resolution version of the same data.  Similarly for the
other constellations: 1087 messages contain high resolution observations of
GLONASS satellites, 1087 is for Galileo and 1127 is for Bediou.

There are other constellations which are only visible in certain parts of
the world, and not in the UK where I live.  I don't decode those messages
either.  If I tried, I wouldn't have any real data with which to check the
results.

Each
satellite in a constellation is numbered.  An MSM allows 64 satellites
numbered 1-64.  At any point on the Earth's surface only some satellites will
be visible.  Signals from some of those may be too weak to register, so the
message will contain readings of just some signals from just some satellites.
My base stations are dual band(receiving on two frequency bands) and can see one or two signals from each satellites.  They typically see signals from 6-8 satellites
from each of the four visible constellations in each scan, and produce four
MSMs every second containing those results.

MSM7 messages are just the format that allows the most content and the most accuracy.
The shorter MSM4 messages carry the same content with less accuracy.
Even given the educed accuracy,
MSM4 messages are sufficient to provide accuracy down to 2cm.  

An MSM message starts with a header, represented here by an MSMHeader
structure.  Following the header is a set of cells listing the satellites
for which signals were observed.  Those data is represented by a
[]MSM7SatelliteCell.  The message ends with a set of signal readings,
at least one per satellite cell and currently no more than two.  Those
data are represented in Go by a [][]MSM7SignalCell (a slice of slices of MSM7SignalCell objects, one outer slice per
satellite).
If signals from seven satellites were observed, there will be seven sets
of signal cells with one or two entries in each set.

The header includes a satellite mask, a signal mask and a cell mask.  These
bit masks show how to relate the cells that come after the header, to satellite
and signal numbers.  For example, for each of the satellites observed a bit is
set in the 64-bit satellite mask, bit 63 for satellite 1, bit 0 for satellite 64.
For example, if the satellite mask is

```
0101100000000010000101000000010000000000000000000000000000
```

seven bits are set in that mask so there will be seven satellite cells and they will contain
data for satellites, 2, 4, 5 and so on.

The standard supports up to 32 types of signal numbered 1-32.  Each signal can be on a different
frequency, although some signals share the same frequency.  
The RTCM standard defines the meaning of each signal
type and the frequency that it is broadcast on.

If the 32-bit signal mask is

```
1000000000001000000000000000000
```

Bits 1 and 13 are set
which means that the device observed 
signal types 1 and 13 from the satellites that it can see.
It may have observed signal type 1 from satellite,
signal type 13 from another,
signals of both types from a third, and so on.
hat becomes clear when we look at the cell mask.

The cell mask is variable length, nSignals X nSatellites bits, where nSignals
is the number of signal types observed (2 in the above example) and nSatellites is the number of
satellites (7 in the example).  The cell mask is an array of bits with nsatellite
elements of nSignals each - in this example 7X2 = 14 bits long, showing which signals
were observed for each satellite. For example,
if the satellite mask and signal mask are as above and the cell mask is

```
01 11 11 10 10 10 10
```

the first pair of bits 01 means that signal 1 from satellite 2 was not observed but signal 13 was.
The second pair 11 means that
both signals were observed from satellite 4, and so on.

The cell mask is the last item in the header.

The header is followed by the satellite cell list.  It's m X 36 bits long where m is
the number of satellite from which signals were observed.  However,
it's not simply an array of those data.
The bit
stream is divided into fields
and we get all of the first fields for the satellites,
followed by all of the second fields,
and so on.
Also, the range (the distance to the satellite)
is expressed in milliseconds transit time
(at the speed if light)
and we get it them as an approximate value and a separate delta
to be added or subtracted to correct the first value.

So for m satellites the stream contains sets of m values:

* m approximate range values in milliseconds
* m extended data values
* m range delta values
* m approximate phase range rate values

The phase range rate deltas are given in the signal list.
That's laid out in the same way, a set of values for the first field, a set for the next field and so on.
It's an array s X 80 bits where
s is the total number of signals satellite by satellite.  For example, if
signal 1 was observed from satellite 1, signals 1 and 3 from satellite 3 and
signal 3 from satellite 5, there will be four signal cells, once again
arranged as s fields of one type followed by s fields of another type and so on.

The signal list is followed by any padding necessary to fill the last byte,
possibly followed by a few zero bytes of more padding.


Finally comes the 3-byte
CRC value.  The next message frame starts immediately after.

My base station is driven by a UBlox ZED-F9P device, which operates in a fairly
typical way.  It scans for signals from satellites and sends messages at intervals
of a multiple of one second.  The useful life of an MSM message is short, so you
might configure the device to scan and send a batch of observations once per second.
For type 1005 messages, which give the position of the device, the situation is
different.  When a rover connects to a base station and starts to receive messages,
it needs a type 1005 (base station position) message to make sense of the MSM (signal
observation) messages.  The base station doesn't move, so the rover only needs the
first the 1005 message.  A good compromise is to configure the device to send one
type 1005 message every ten seconds.  That reduces the traffic a little while ensuring
that when a rover connects to the data stream it will start to produce position fixes
reasonably quickly.

For RTCM correction to work properly, the operator has to tell the base station its position.  If that setting is out by, say,
10cm to the North,
the rover's position will be out by the same amount.
so the problem becomes finding the position of the base accurately.

The accuracy figures quoted earlier are possible in ideal conditions.
Unfortunately, we don't live in ideal conditions and in the real world we get less accuracy,
but typically at least 100cm.
However, it's worth mentioning that traditional surveying with theodolites can only produce accuracy of about 1m.  A base and rover communicating via RTCM can cost as little as $2,000 and the system is easier to use, faster and more accurate than traditional systems.

Timestamps
========

The handler needs a start time because MSM7 messages contain a
timestamp, in most cases milliseconds from the constellation's
epoch, which rolls over every week.  (The exception is GLONASS
which uses a two-part timestamp containing a day of the week and
a millisecond offset from the start of day.)  The handler displays
all these timestamps as times in UTC, so given a stream of
observations advancing in time, it needs to know which week the
first ones are in.

The timestamps for different constellatons roll over at different
times.  For example, the GPS timestamp rolls over to zero a few
seconds after midnight UTC at the start of Sunday.  The GLONASS
timestamp rolls over to day zero, millisecond zero at midnight at
the start of Sunday in the Moscow timezone, which is 21:00 on
Saturday in UTC.  So, if the handler is processing a stream of
messages which started at 20:45 on a Saturday in UTC, the GLONASS
timestamp value will be quite large.  At 21:00 the epoch rolls
over and the timestamps start again at (zero, zero).  Meanwhile
the GPS timestamps will also be large and they will roll over to
zero a few seconds after the next midnight UTC.

The handler can keep track of this as long as (a) it knows the time
of the first observation, and (b) there are no large gaps in the
observations.  If there was a gap, how long was it and has it taken
us into a different epoch?

All of the timestamps roll over at the weekend, so if the handler is
started on a weekday, it just needs a start time in same week as the
first observation.  If it's started close to any rollover, it needs a
more accurate start time.

If the handler is run without supplying a start time, it assumes
that the data is coming from a live source and uses the system time
when it starts up.  So the system clock needs to be correct.  For
example, if you start the handler near midnight at the start of Sunday
UTC and your system clock is out by a few seconds, the handler might
assume the wrong GPS week.