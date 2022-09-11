## Go tools to support the NTRIP protocol

This repository contains Go software to support Network Transport of RTCM over IP (NTRIP) messages.

The RTCM protocol is named after the organisation that defined it, the Radio Technical Commission for Maritime services or RTCM.  It's used to carry observations of global positioning satellites from fixed base stations to moving rovers to allow the rovers to better find their positions.

The NTRIP protocol defines simple wrapper to carry the RTCM 
messages over an Internet connection. 
It's an open-source standard, published for free by the RTCM. 

RTCM data can be carried over radio
or, using NTRIP, over the Internet.
The latter requires the sender and the receiver to be connected to the network.
The data requires little bandwidth - a few kilobytes per second which is
low enough to be carried by a domestic broadband connection or a mobile phone's Internet connection.

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

Each RTCM base station is in a known location.
The GNSS satellites currently broadcast signals on two frequency bands,
one for unencrypted public data and another for encrypted data meant for the owner's police force, military etc.
A dual-band base station scans for signals from all the satellites it can see for signals.
A base station that you and I can buy
can't decrypt the encrypted signals but analysing the effect of the ionosphere on the carrier wave allows a rover to better estimate the distortion.

The base passes its observations to any rovers that have subscribed to receive them, along with its known real position.
A rover close to the base sees the same satellites with the same distortions and it use the information from the base to correct the calculation of its own position.
The rover only needs to pick up the public signals from the satellites
so it can be single-band,
which is cheaper.

In ideal conditions a rover within about 8km of a base station and receiving RTCM data from it
can estimate its position to within 2cm.
Beyond 8km, the accuracy falls linearly -
within 16km a rover can attain an accuracy of 4cm,
within 24km an accuracy of 8cm, and so on.
At about 64 km the RTCM data only allows about 2.5m accuracy,
which is close to what the rover can achieve without help.

Unfortunately, we don't live in ideal conditions.
Here in the real world we get less accuracy,
but typically at least 100cm.
That's still better than traditional surveying techniques using theodolites - 
they achieve accuracy of around 1m at best.

Accurate GNSS systems are also easier and faster to use than theodolites.
The UK's mapping authority the Ordnance Survey has been using them
for many years.
In the early days they were expensive but now
a base and rover communicating via RTCM can cost as little as $2,000
and one base can support many rovers.

For RTCM correction to work properly, the operator has to tell the base station its position.  If that's wrong by, say,
1.5m to the North,
each of the rover's position will be wrong by the same amount.
The positions in the resulting survey will be accurate
relative to each other
but they will each be shifted from their true position by 1.5m to the North. 
So the problem becomes finding the position of the base accurately.

To avoid the rover or the base station needing a fixed Internet (IP) address,
they communicate via an intermediate device called an NTRIP caster (broadcaster).
The caster is just a web server on the Internet so
it doesn't need to be close to the base station or the rover, 
it can be anywhere.

The caster offers a set of named endpoints.
In the simple case, each endpoint is fed by a single base station,
(so endpoint equals base station).
A rover subscribes to an NTRIP feed from one endpoint.
The base station reads signals from the satellites that it can see,
encodes them as RTCM messages, packages those up into NTRIP messages
and sends them over the Internet to its assigned endpoint on the caster.
The caster sends those messages on to any rovers that have subscribed to that endpoint.

(The more complicated case involves taking readings from many base stations spread over a wide area
and merging them together to create a virtual base station feeding a single endpoint.)

A configuration that I've used is a GNSS receiver such as a Sparkfun
RTK board producing RTCM and other messages, connected via
I2C or serial USB to a host computer running my RTCM filter.  The
host could be a Windows machine but something as cheap as
a Raspberry Pi single board computer is quite adequate:

```
 -------------          messages          --------------
| GPS device  | -----------------------> | Raspberry Pi |
 -------------    serial connection       --------------
```

With a bit more free software that can be used to create an NTRIP
base station.
The software for the base station is called an NTRIP server.
This github account contains a ready-made NTRIP server and an NTRIP caster,
derived from open source software originally
developed by the International GNSS Service (IGS).
This is my setup:

```
  messages    ------------   RTCM    --------------   RTCM over NTRIP
-----------> | rtcmfilter | ------> | NTRIP server | ----------------->
              ------------   stdout  --------------      Internet
```

My base station is composed of an antenna on the roof of my garden shed
and a GNSS receiver and a Raspberry Pi in the shed.
The Raspberry Pi connects to my Internet router via WiFi.
The RTCM filter cleans up the data received from the GNSS receiver
and passes it to the NTRIP server, which sends it on to my caster.

My NTRIP caster is the free open source version from IGS.
It runs on a Digital Ocean droplet which costs $5 per month to rent.

My rover is an Emlid Reach M+, which costs about $300 and uses my smartphone to provide an Internet connection in the field. 

The stationary base station sends RTCM messages over the Internet using the
NTRIP protocol to the NTRIP caster which sends them on to moving GNSS rovers:

```
                                                    NTRIP
                                                  ---------> rover
 ------------                       --------     /
| NTRIP base |   RTCM over NTRIP   | NTRIP  | __/
| station    | ------------------> | caster |   \
 ------------                       --------     \  NTRIP
                                                  ---------> rover
```


RTCM
=====

The RTCM protocol is currently at version 3.
RTCM3 messages are in as
compact binary form and not readable by eye.
The format is described by RTCM STANDARD 10403.3
Differential GNSS (Global Navigation Satellite Systems) Services –
Version 3 (RTCM3 10403.3).  This is not an open source standard and
it costs about $300 to buy a copy.  

The standard defines a large number of message types,
each with its own format.  Fortunately most of them are redundant.
A complete NTRIP service can be created using just six or seven types of message.

There is a little bit of useful information scattered around
various public web pages.  
There's also an open source library of C code to handle them, RTKLIB.
I've copied some of
the more relevant RTKLIB source files into this repository as a handy reference.

There are already open-source tools available to convert an RTCM3 data stream into messages
in another format called RINEX.  That's an open standard and the result is human readable. 

To figure out the format of the RTCM message I'm
interested in, I read what I could find, including the RTKLIB source code.
Then I took the RTCM3 messages
that my device produced, used the tools to convert them into RINEX format and examined
the result. 
These data form some of my unit and integration tests.

The rtcm package in this repo contains the logic to decode and display RTCM3
messages produced by GNSS devices such as the U-Blox ZED-F9P.
I've used this to create a few ready-made tools such as the RTCM filter.  

The RTKLIB source code does a very good job of decoding RTCM messages,
but comments are scant and the code is opaque in places.
Developers may find my Go source code easier
to understand.

An RTCM3 message is binary and variable length.  Each message frame
is composed of a three-byte header, an embedded message and 3 bytes of
Cyclic Redundancy Check (CRC) data.  The header starts with 0xd3 and
includes the length of the embedded message.  Each message starts with
a 12-bit message number which defines the type.  Apart from that
message number, each type of message is in a different format.
Fortunately, I only have to worry about a few types.

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
message is 176 bytes long.  With the header and CRC
the whole message frame is 182
bytes long.  As shown above, the embedded message may end with some
zero padding bits to complete the last byte and possibly a few zero padding bytes.

The last three bytes of the frame (in this case 4d, f5 and 5a) are the
CRC value.  To check the CRC, take the header and the embedded message,
run the CRC calculation over those bytes and compare the result with
the given CRC.  If they are different then the message is not RTCM3 or
it's been corrupted in transit.

The CRC check is calculated using an algorithm from Qualcomm.  I use Mark
Rafter's Go implementation, also in this github account at https://github.com/goblimey/go-crc24q.

The first 12 bits of the embedded message give the message number, in
the example hex 449, decimal 1097, which is a type 7 Multiple Signal Message
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
newlines.
The last line of the example contains the start of the next
message.  Other data in other formats (such as NMEA) may be interspersed between frames.
My rtcm software discards anything that's not an RTCM3 message frame with a
correct CRC value.

There are many hundreds of RTCM3 message types, some of which are just
different ways of representing the same information.  To get an accurate fix
on its position, a rover only needs the position of the base station
and a recent set of the base's observations of satellite signals, which is to
say a type 1005 message and a set of MSM7 or MSM4 messages, one for each constellation
of satellites (GPS, GLONASS or whatever).

Message type 1005 gives the position of the base station (or more strictly,
of a point in space a few centimetres above its antenna).

Message types 1074, 1084 and so on are MSM type 4 messages.
Message types 1077, 1087 and so on are the equivalent MSM type 7 messages.
They contain the same data as their MSM4 equivalents
but to higher resolution.
Each contains observations by the base station of signals from
satellites in one constellation.  Type 1074 and type 1077 contain
signal data from GPS satellites.
Similarly for the
other constellations: 1084 and 1087 for
GLONASS satellites, 1084 and 1087 for Galileo and 1124 and 1127 for Bediou.

MSM4 resolution is sufficient for 2cm accuracy.
My guess is that MSM7 is ready for satellites in the future that will deliver more accuracy.


Each
satellite in a constellation is numbered.  An MSM allows 64 satellites
numbered 1-64.  At any point on the Earth's surface only some satellites will
be visible.  Signals from some of those may be too weak to register, so the
message will contain readings of just some signals from just some satellites.
My base stations are dual band
and can see one or two signals from each satellite.  They typically see signals from 6-8 satellites
from each of the four constellations in each scan, and produce four
MSMs every second containing those results. 

An MSM message starts with a header, represented in my RTCM software by an MSMHeader
structure.  Following the header is a set of cells listing the satellites
for which signals were observed.  Those data is represented by a
[]NSM4SatelliteCell object (a list of satellite cells)
or for an MSM7 message, a []MSM7SatelliteCell.  
The message ends with a set of signal readings,
at least one per satellite cell and currently no more than two.
Those
are represented by a [][]MSM4SignalCell or a [][]MSM7SignalCell
(a list of lists of signal cells, one outer list per
satellite).
If signals from seven satellites were observed, there will be seven sets
of signal cells with one or two entries in each set.

The header includes a satellite mask, a signal mask and a cell mask.  These
bit masks show how to relate the cells that come after the header to satellite
and signal numbers.  For example, for each of the satellites observed a bit is
set in the 64-bit satellite mask, the first bit, bit 63, for satellite 1, bit 0 for satellite 64.
For example, if the satellite mask is

```
0101100000000010000101000000010000000000000000000000000000
```

seven bits are set in that mask.
There will be seven satellite cells and they will contain
data for satellites, 2, 4, 5 and so on.

The standard supports up to 32 types of signal numbered 1-32.  
Each signal can be on a different
frequency, although some signals share the same frequency. 
Currently the satellites are dual-band and only send two signal types,
one in each frequency band. 
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

The cell mask shows what signals were observed.
It's variable length, nSignals X nSatellites bits long, where nSignals
is the number of signal types observed (2 in the above example) and nSatellites is the number of
satellites (7 in the example).  The cell mask is an array of bits with nSatellite
elements of nSignals each - in this example 7X2 = 14 bits long. 
For example,
if the satellite mask and signal mask are as above and the cell mask is

```
01 11 11 10 10 10 10
```

the first pair of bits 01 means that the receiver did not pick up 
signal 1 from satellite 2 but it did pick up signal 13.
The second pair 11 means that it observed
both signals from satellite 4, and so on.

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

Distances are given in milliseconds.
(To turn them into distances in metres, multiply by the speed of light.)

The signal list that follows includes deltas for the phase range rate values.
That list is laid out in the same way,
the value of the first field for the each satellite
followed by the value for the second field for each satellite
and so on.
It's an array of s X 80 bits where
s is the total number of signals satellite by satellite.  For example, if
signal 1 was observed from satellite 1, signals 1 and 3 from satellite 3 and
signal 3 from satellite 5, 
that's four signals altogether so
there will be four signal cells.

To make sense of the satellite cell list and signal cell list,
the software needs to look at the masks while it's reading them.

The signal list is followed by any padding necessary to fill the last byte.
The GNSS receiver can then add a few zero bytes on the end if it wishes.

Finally comes the 3-byte
CRC value.

The next message frame starts immediately after,
with no intervening newline byte.
That could be a few NMEA messages,
each separated from the next by a newline,
or it could be another RTCM messages,
signalled by the special 0xd3 byte.

Whe observing GLONASS satellites,
the rover also needs GLONASS code phase bias messages
to supplement the GLONASS MSMs.
They contain a single value,
the bias.
My equipment consistently produces bias values of zero.
(That pretty much summarises what I know about those messages.
I have no clue what they are represent.)

Finally, UBlox advises that my UBlox ZD-F9P base station receiver emits one extra message type.
It's in an unpublished proprietary format
defined by UBlox.
My guess is that this is useful when I connect my UBlox control software to the device to configure it.

As I said earlier, there are many other RTCM message types,
but the ones relevant to finding your location simply repeat the information in the MSMs,
so they are now redundant.
The RTKLIB software has functionality to decode them.
My equipment uses MSMs so I don't bother with the other types.

I have a base station driven by a UBlox ZED-F9P device, which operates in a fairly
typical way.  It scans for signals from satellites and sends messages at intervals
of a multiple of one second.  The useful life of an MSM message is short, so you
might configure the device to scan and send a batch of observations once per second.
For type 1005 messages, which give the position of the device, the situation is
different.  When a rover connects to a base station and starts to receive messages,
it needs a type 1005 (base station position) message to make sense of the MSM (signal
observation) messages.  The base station doesn't move, so the rover only needs one type 1005 message
during the session,
but it can't work properly until it receives it.
To avoid using unnecessary bandwidth,
a good compromise is to configure the device to send one
type 1005 message every ten seconds.  That reduces the traffic a little while ensuring
that when a rover connects to the data stream it will start to produce accurate position fixes
reasonably quickly.


Timestamps
========

To analyse a set of RTCM messages,
the handler needs to know when they were collected.
That's because MSM7 messages contain a
timestamp, in most cases milliseconds from the constellation's
epoch, which rolls over every week.  (The exception is GLONASS
which uses a two-part timestamp containing a day of the week and
a millisecond offset from the start of day.)  The handler displays
all these timestamps as times in UTC, so given a stream of
observations advancing in time, it needs to know which week the
first one was taken.

The handler takes a start date and time when it's created.
This can be any time during the week of first observation.

The timestamps for different constellations roll over at different
times.  For example, the GPS timestamp rolls over to zero a few
seconds after midnight UTC at the start of Sunday.  The GLONASS
timestamp rolls over to day zero, millisecond zero at midnight at
the start of Sunday in the Moscow timezone, which is 21:00 on
Saturday in UTC.  So, if the handler is processing a stream of
messages which started at 20:45 on a Saturday in UTC, the GLONASS
timestamp value will be quite large.  At 21:00 the epoch rolls
over and the timestamps start again at (zero, zero).  Meanwhile
the GPS timestamps will also be large and they will roll over to
zero about three hours later, a few seconds after the next midnight UTC.

The handler can keep track of this as long as (a) it knows the time
of the first observation, and (b) there are no large gaps in the
observations.  If there was a gap, how long was it and has it taken
us into a different week?

All of the timestamps roll over at the weekend, so if the handler is
started on a weekday, it just needs a start time in the same week as the
first observation.  If it's started close to any rollover, it may need a
more accurate start time.

If the handler is processing a live feed from a GNSS device,
the current system time can be used as the start time,
but only if it's correct.
For
example, if you start the handler near midnight at the start of Sunday
UTC and your system clock is out by a few seconds, the handler might
assume the wrong week for GPS observations.
You can use the Network Time Service (NTP)
to keep your system clock synchronised to the correct time.