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

A GNSS device's position can be expressed in all sorts of ways.
This paper from the UK's mapping authority the Ordnance Survey
gives an excellent introduction https://www.ordnancesurvey.co.uk/documents/resources/guide-coordinate-systems-great-britain.pdf

Since the satellites are orbiting around the centre of the Earth,
ECEF format is a fairly natural way to represent a position.
That uses cartesian coordinates measured in metres from the centre of the Earth.  As shown in the Ordnance Survey paper,
the X axis runs from the North Pole to the South Pole, the Y axis runs from the point where zero latitude meets the Equator
to the centre.  The Z axis is perpendicular to the other two.

The Earth spins on its axis, which stretches it slightly at the Equator.
Geographers have defined a perfect elipsoid that's close to its real shape.  A position in ECEF format can be converted to Longitude, latitude and the height above or below this elipsoid.

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

About twenty years ago, Geoffrey Blewitt of Newcastle University
published his paper "Basics of the GPS Technique: Observation Equations".
He's now Professor at the Nevada Bureau of Mines and geology and you can download his paper from there:

https://nbmg.unr.edu/staff/pdfs/blewitt%20basics%20of%20gps.pdf

or just search for title on Google.

Some readers might find the maths in the paper a bit advanced for some people,
so here is a simple explanation:

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
In the early days such equipment was expensive but now
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

There are many hundreds of RTCM3 message types,
but they are not all required to find an accurate position,
and some that can be used for that are redundant,
replaced by Multiple Signal Messages (MSMs),
which carry the observations of signals from satellites.
There is an MSM for each constellation, so I'm interested in:

* 1074 type 4 (low resolution) observations of signals from GPS satellites
* 1077 type 7 (high resolution) observations of signals from GPS satellites
* 1084 type 4 for GLONASS
* 1087 type 7 for GLONASS
* 1094 type 4 for Galileo
* 1097 type 7 for Galileo
* 1124 type 4 for Beidou
* 1127 type 7 for Beidou

A base station should be configured to send either MSM4 or MSM7 messages.
Sending both just wastes bandwidth.

MSM type 7 messages
include the same fields as their MSM4 equivalents
but to higher resolution,
and they have some extra fields,
so they take up a little more network bandwidth.
Both types are sufficient for 2cm accuracy
and many operators use MSM4 instead of MSM7.
I don't know if MSM7 messages have any operational advantage over MSM4
for 2cm accurate working.

UBlox advises that if a rover based on their technology uses GLONASS MSMs (type 4 or 7)
the base station should also send
messages of type 1230
GLONASS code-phase bias values.
Unfortunately
the RTKLIB software doesn't have functionality to decode one of those.

Apart from MSM material, a rover also needs type 1005 messages,
which give the position of the base station.

The UBlox ZED-F9P Integration Manual (2022-02 edition) 
section 3.1.5.5.3 'Base station: RTCM output conﬁguration'
recommends that the base station sends these messages:

* RTCM 1005 Stationary RTK reference station ARP (base position)
* RTCM 1074 GPS MSM4
* RTCM 1084 GLONASS MSM4
* RTCM 1094 Galileo MSM4
* RTCM 1124 BeiDou MSM4
* RTCM 1230 GLONASS code-phase biases

The RTKLIB software decodes all of these except for type 1230,
so I can reverse-engineer the first five types.

Types 1074, 1084, 1094 and 1134 are Type 4 Multiple Signal Messages (MSM4).
Type 7 (MSM7) messages can be used instead - 1077, 1087, 1097 and 1127.
Type 7 messages contain the same information as type 4 but some fields
are of higher
resolution.
(The resolution of MSM4 is sufficient for 2cm accuracy.)

The Integration Manual also mentions two 
u-blox proprietary message types:

* 4072.0 Reference station PVT and
* 4072.1 Additional reference station information,

It appears that these are only required to handle
a moving base station,
and 4072.1 is only used by old firmware.
Type 4072.0 appears to give the base position
(possibly because message type 1005 is intended to describe
the position of a fixed base station and
the values are not expected to change).
The format of those two message types is not published
so they are not handled by this software.

So a working base station might be configured to send out messages of type 1005, 1074, 1084, 1094, 1124 and 1230.  Alternatively it could use high resolution MSMs, so 1005, 1077, 1087, 1097, 1127 and 1230.

Ideally the base should send most of those messages once per second.
Type 1005 can be sent out less often.
For a particular base those messages will always contain the same values
so a rover only needs to receive one of them
at the start of a session to work properly.
The advice is to send one every ten seconds,
so when a rover subscribes to an endpoint on a caster
it will receive a type 1005 within a few seconds.
It takes a while for it to download the information it needs from the satellites,
so a short wait for a base position message is OK.

The RTKLIB source code has functionality to decode most RTCM messages,
so if you need to understand other message types, go there.
However, that code can be difficult to follow and comments are scant.

RTCM Format
=======

An RTCM3 message is binary and variable length.  Each message frame
is composed of a three-byte header, an embedded message and 3 bytes of
Cyclic Redundancy Check (CRC) data.  The header starts with 0xd3 and
includes the length of the embedded message.  Each message starts with
a 12-bit message number which defines the type.  Apart from that
message number, each type of message is in a different format.

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
3-byte header, an embedded message and 3 bytes of Cyclic Redundancy
Check (CRC) data.

Byte 0 of an RTCM frame is always d3.  The top six bits of byte 1 are
always zero.  The lower two bits of byte 1 and the bits of byte 2 form the 10-bit
message length, in this case hex 0aa, decimal 176.  So the embedded
message is 176 bytes long.  With the header and CRC
the whole message frame is 182
bytes long.  

The bit string in the embedded message may end with some
zero padding bits to complete the last byte and that can be followed
by a few zero padding bytes, as in the first message in the example.

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

A d3 byte marks the start of each message but
the data within can also contain one.
Note the d3 value on the
fifth line of the hex dump above.  This is not the start of another
message.  One clue is that it's not followed by six zero bits.  To extract
a message frame from a stream of data and decode it, you need to read the
header and the next two bytes, check the header, find the message length,
read the whole message frame and check the CRC value.  This matters
particularly when you start to receive a stream of data from a device.  You
may come into the data stream part-way through and blunder into a d3 byte.
You can't assume that it's the start of a message.

The CRC data is also used to check that the message has not been corrupted in
transit.  If the CRC check fails, the mesage must be discarded.

RTCM3 message frames in the NTRIP data stream are contiguous with no separators or
newlines.
The last line of the example contains the start of the next
message.  Data in other formats (such as NMEA) may be interspersed between frames.

Each
satellite in a constellation is numbered.
The standard allows 64 satellites
numbered 1-64 in each constellation,
sending up to 32 types of signal
on different frequencies.
At any point on the Earth only some satellites will
be visible.  Signals from some of those may be too weak to register, so the
resulting
message may contain readings of just some signals from just some satellites.
My base stations are dual band
and can receive up to two signals from each satellite.  They typically see signals from 6-8 satellites
from each of the four constellations in each scan. 

By careful reading of the RTKLIB software,
it's possible to reverse-engineer the format of the messages.
(I include copies of the relevant C code in this repository as a handy reference.)
More clues can be found in the source code of the IGS reference software
such as the BNC tool.

Type 1005 - Stationary RTK Reference Station ARP
----------------

See decode_type1005() in rtcm3.c at line 375.

That function defines an object of type sta_t, defined in rtklib.h at line 833:

```
typedef struct {        /* station parameter type */
    char name   [MAXANT]; /* marker name */
    char marker [MAXANT]; /* marker number */
    char antdes [MAXANT]; /* antenna descriptor */
    char antsno [MAXANT]; /* antenna serial number */
    char rectype[MAXANT]; /* receiver type descriptor */
    char recver [MAXANT]; /* receiver firmware version */
    char recsno [MAXANT]; /* receiver serial number */
    int antsetup;       /* antenna setup id */
    int itrf;           /* ITRF realization year */
    int deltype;        /* antenna delta type (0:enu,1:xyz) */
    double pos[3];      /* station position (ecef) (m) */
    double del[3];      /* antenna position delta (e/n/u or x/y/z) (m) */
    double hgt;         /* antenna height (m) */
} sta_t;
```

So amongst other things, the message contains the antenna position expressed in ECEF format.

Reading the function, the format of the message is:

```
message type - 12 bit unsigned integer
Station ID - 12 bit unsigned integer
ITRF realisation year- 6 bit unsigned integer
? - 4 bits
ECEF X position - 38 bit signed integer to be converted to floating point
? - 2 bits
ECEF Y position - 38 bit signed integer to be converted to floating point
? - 2 bits
ECEF Z position - 38 bit signed integer to be converted to floating point
```

(The RTKLIB software ignores the fields marked by "?".
If you need to know what's in them,
you will have to buy a copy of the standard.)

The 38 bit values are the distance from the centre of the Earth 
along the axis (positive or negative) in tenth millimeters.
to convert to meters, divide by 0.0001.

That granularity is much smaller than the accuracy
that the current equipment provides - 2cm.
It's obviously intended to be always more than is needed.



MSM format
----------

Multiple Signal Messages (MSMs)
contain the data from the signals received (observed) from the satellites.
Each message gives the signals observed from a particular constellation.
My equipment is configured to send four every second,
on for GPS, one for Galilaeo, etc. 

Distance information (range) in the message is giving as
the transit time from the satellite to the base station in milliseconds.
To convert this into a distance in metres,
multiply by the speed of light per millisecond.

Ranges and other values may be represented by an approximate value
in one part of the message and delta value in another part,
both scaled integers.
The delta is a signed integer.
Adjust the scales and add the delta to the approximate value to correct it.

An MSM message starts with a header which  
includes three masks, the satellite mask, the signal mask and
the cell mask.
These indicate indicate how many satellite and signal cells there are
in the message and show
which satellites and signals numbers they relate to.

The cell mask is variable length,
so the header is variable length.

All MSM message types use a common header format.
In the RTKLIB code, line of rtcm3.c defines a type msm_h_t
with some useful comments on the right-hand side:

```
typedef struct {                    /* multi-signal-message header type */
    unsigned char iod;              /* issue of data station */
    unsigned char time_s;           /* cumulative session transmitting time */
    unsigned char clk_str;          /* clock steering indicator */
    unsigned char clk_ext;          /* external clock indicator */
    unsigned char smooth;           /* divergence free smoothing indicator */
    unsigned char tint_s;           /* smoothing interval */
    unsigned char nsat,nsig;        /* number of satellites/signals */
    unsigned char sats[64];         /* satellites */
    unsigned char sigs[32];         /* signals */
    unsigned char cellmask[64];     /* cell mask */
} msm_h_t;
```

Line 1745 of rtcm3.c defines a function decode_msm_head which reads the bitstream and
creates an msm_h_t object.

The getbitu() function reads a given set of bits from the bit stream,
interprets them as an unsigned integer of that size and returns the value.

The getbits() function interprets the bit field as a signed two's complement integer.

So we can see that the format of the bitstream is:

```
Message Type - 12 bit unsigned integer
Station ID - 12 bit unsigned integer
timestamp - 30 bit unsigned integer
sync - 1 bit
issue of data station - 3 bit unsigned integer
cumulative session transmitting time - 7 bit unsigned integer
clock steering indicator - 2 bit unsigned integer
external clock indicator - 2 bit unsigned integer
divergence free smoothing indicator - 1 bit
smoothing interval - 3 bit unsigned integer
satellite mask - 64 bit unsigned integer, one bit per satellite for which signals were observed
signal mask - 32 bit unsigned integer, one bit for each signal type observed
cell mask - nSatellitesXnSignals bits(variable length but <= 64)
```

In most constellations, the timestamp is milliseconds from the start of the week.

The week for each constellation starts at different times.

The GPS week starts at midnight at the start of Sunday
but GPS time is ahead of UTC by a few leap seconds, so in
UTC terms the GPS week starts on Saturday a few seconds before midnight.
Since 2017/01/01, GPS time is ahead of UTC by 18 leap seconds
so in UTC terms the timestamp rolls over on Saturday 18 seconds before midnight
at the start of Sunday.
An extra leap
second may be added every four years.  The start of 2021 was a
candidate for adding another leap second but it was not necessary.
One may be added in 2025.

Galileo time is currently (2022) the same as GPS time.

The Beidou timestamp rolls over in UTC terms at 14 seconds after 
midnight at the start of Sunday.

For the GLONASS constellation, the timestamp is in two parts,
the top 3 bits giving the day and the lower 27 bits giving
milliseconds since the start of the day.  The day is 0: Sunday,
1: Monday and so on.
The Glonass day starts at midnight but in the Moscow timezone,
which is three hours ahead of UTC,
so day 6 rolls over to day 0 at 9pm on Saturday in UTC terms.

Following the header comes the list of satellite cells
giving data about the satellites that sent the observed signals..  
The message ends with a set of signal cells
giving data about the observed signals.
If a constellation sends two types of signal
and signals were observed from 7 satellites,
there will be seven sets
of satellite cells in the message,
followed by up to 14 signal cells.
However, if the base station only received one signal from two of the satellites,
there will only be 12 signal cells.
The three masks in the header
show how to relate the data in the cells
to satellites and signals.

The signal mask is 64 bits long.
The satellites in a constellation are numbered 1-64,
so the standard supports 64 satellites in each constellation.
For each of the satellites observed a bit is
set in the mask, the first bit for satellite 1, the last bit for satellite 64.
(Confusingly, viewed as a 64-bit integer,
the first bit is the top bit, so bit 63 represents satellite 1
and bit 0 represents satellite 64.)
For example, if the satellite mask is

```
0101100000000010000101000000010000000000000000000000000000
```

Bits 2, 4, 5 etc, seven bits in all are set in that mask
so signals were observed from the seven satellites,
with those numbers.
There will be seven satellite cells in the message,
each containing data about one satellite.
The first will be for satellite 2,
the second for satellite 4,
and so on.

The signal mask is 32 bits,
laid out in the same way,
so the standard supports 32 types of signal
numbered 1-32.
Signals can share the same frequency.
The RTCM standard defines the meaning of each signal
type and the frequency that it's broadcast on.
Currently the satellites are dual-band and only send two signals,
one in each frequency band. 


If the 32-bit signal mask is

```
1000000000001000000000000000000
```

Bits 1 and 13 are set
which means that the bas station observed 
signal types 1 from at least one of the satellites that it can see,
and signal 13 from at least one, not necessarily the same one.
It may have observed signal type 1 from one satellite,
signal type 13 from another,
signals of both types from a third, and so on.

The cell mask shows what signals were observed.
It's variable length, nSignals X nSatellites bits long, where nSignals
is the number of signal types observed (2 in the above example) and nSatellites is the number of
satellites (7 in the example).
The cell mask is an array of bits with nSatellite
elements of nSignals each,
so with the satellite and signal masks as in the example it will be 14 bits long. 
For example:

```
01 11 11 10 10 10 10
```

The first pair of bits 01 means that the receiver did not pick up 
signal 1 from satellite 2 but it did pick up signal 13.
The second pair 11 means that it observed
both signals from satellite 4, and so on.
Nine bits in the cll mask are set
so the signal cell list in the message will contain nine cells.

The cell mask is the last field in the header.
All the other fields are fixed length but the cell mask
is variable length,
so the header is variable length.

The header is followed by a list of satellite cells,
then a list of signal cells,
both variable length.

The cells in an MSM7 message are bigger than the ones in an MSM4
and contain extra data.

MSM4 Satellite and Signal Cells
-----

The cells contain the data about satellites and signals,
but the order is not what a programmer might expect.
The bit
stream is divided into fields
starting with all of the values of the first field,
followed by all of the values of the second field,
and so on.

Some fields have a special value indicating that the value is invalid and should be ignored.
In the raw binary number,
the top bit of an invalid value is a one and the rest of the digits in the bit stream are 0.
Onc the binary has been read and converted, the meaning will depend on the data type.
If the top bit of a signed integer is set,
the number is negative.
For example, if the bits of the field represent a 22 bit two's complement signed integer,
the invalid value is 100000000000000000, which is decimal -2097152.

The fields used in MSMs that can contain invalid values are:

```
 8 bits unsigned: 10000000 0x80 255
14 bits signed: -8192 
15 bits signed: -16384
20 bits signed: -524288
22 bits signed: -2097152
24 bits signed: -8388608
```

In the message, the ranges (the distance from the satellite to the receiver)
are given as transit time in milliseconds and in three parts.
The satellite cells contains an approximate range as
whole milliseconds and fractional milliseconds -
The signal cells contain deltas.

If signals were observed from n satellites,
the satellite cell data are:

* n whole milliseconds of approximate range - 8 bit unsigned integers (invalid if 0xff)
* n fractional milliseconds of approximate range - 10 bit unsigned integers

The fractional value is valid if the whole value is valid.

if the total number of signal cells is m
then the signal cell data are:

* m range delta values - 15 bit signed integers (invalid if -16384)
* m phase range values - 22 bit signed integer (invalid if -2097152)
* m lock time values - 4 bit unsigned integer
* m half-cycle ambiguity values - 1 bit boolean
* m CNR values - 6 bit unsigned integer


MSM7 Satellite and Signal Cells
----

After the header comes a list of satellite cells
followed by a list of signal cells.

The MSM7 cells contain all the fields of the MSM4 cells but some
have more bits.
There are also some extra fields in the MSM7 cells,
including the phase range and the phase range rate.
According to Blewitt's paper,
the latter is the velocity at which the satellite is approaching
the receiver (if positive) or moving away from it (if negative). 

If there are nSatellite satellites,
the satellite cell data are:

* nSatellite whole milliseconds of approximate range - 8 bit unsigned integers (invalid if 255)
* nSatellite extended information - 4 bit unsigned integers
* nSatellite fractional milliseconds of approximate range - 10 bit unsigned integers
* nSatellite phase range rate values - 14 bit signed integers (invalid if -8192)

The fractional value and the deltas are  invalid if the value of the whole milliseconds field is invalid.

The signal cells follow.
if the total number of signal cells nSignals X nSatellites is T,
then the signal cell data are:

```
* T range delta values - 20 bit signed integers (invalid if -524288)
* T phase range values - 24 bit signed integers (invalid if -8388608)
* T lock time values - 10 bit unsigned integers
* T half-cycle ambiguity values - 1 bit booleans
* T CNR values - 10 bit unsigned integers
* T phase range rate delta values - 15 bit signed integers (invalid if -16384)
```

Note that the satellite cells contain the approximate range value
and the signal cells contain the delta values. 
That makes sense because the range values for all of the signals from the same satellite will only be a little bit different,
the difference being due to a small amount of distortion as they come
close to the Earth.
That arrangement of data saves a bit of space in the message
compared with having a complete range value in each signal,
especially if future versions of the satellites send more of them.

The signal list is followed by any padding necessary to fill the last byte.
In the example message shown above, the last non-zero byte of the first message is 0x78, in binary 01111000.

The GNSS receiver can then add a few zero bytes on the end of the message if it wishes.
The example message has some of those.

MSM7 Message Format
---------

```
nSatellite whole milliseconds of approximate range - 8 bit unsigned integers (invalid if 0xff)
nSatellite extended satellite info - 4 bit unsigned integers
nSatellite fractional milliseconds of approximate range - 10 bit unsigned integers
nSatellite approximate range rate - 14 bit unsigned integers (invalid if 0x2000)
```

Finally comes the 3-byte
CRC value.

The next message frame starts immediately after,
with no intervening newline byte.
That could be a few NMEA messages,
each separated from the next by a newline,
or it could be another RTCM messages,
signalled by the special 0xd3 byte.


Timestamps
========

To analyse a set of RTCM messages,
the handler needs to know when they were collected.
That's because MSM messages contain a
timestamp.
Except for GLONASS messages it's a single integer value,
milliseconds from the constellation's
epoch, which rolls over every week.
GLONASS
uses a two-part integer timestamp containing a day of the week and
a millisecond offset from the start of day.
The handler displays
all these timestamps as times in UTC, so given a stream of
observations advancing in time, it needs to know in which week the
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