## Go tools to support the NTRIP protocol

This repository contains Go software to support Network Transport
of RTCM over IP (NTRIP) messages.

The RTCM protocol is named after the organisation that defined it, the Radio Technical Commission for Maritime services or RTCM.  It's used to carry observations of global positioning satellites from fixed base stations to moving rovers to allow the rovers to better find their positions.

The NTRIP protocol defines simple wrapper to carry the RTCM 
messages over an Internet connection. 
It's an open-source standard, published for free by the RTCM. 

RTCM data can be carried over radio
or, using NTRIP, over the Internet.
The latter requires the sender and the receiver to be connected to the network.
The data requires little bandwidth - a few kilobytes per second which is
low enough to be carried by a domestic broadband connection or a mobile phone's Internet connection.

There are a number of networks ("constellations") of global positioning satellites,
currently(2023):

* GPS
* Glonass
* Galileo
* Beidou
* SBAS
* QZSS
* Navic/Irnss

The American Global Positioning System (GPS),
the European Galileo, the Chinese Beidou and the Russian GLONASS constellations
each provide a global service - 
some satellites from each of those constellations should be visible from any point on the Earth's surface.

Strictly these constellations are called Global Navigation Satellite Systems (GNSS)
rather than GPS, which is just the first and best known constellation.
GNSS receivers are available that can use all satellites from those constellations,
in any combination.

A GNSS device's position can be expressed in all sorts of ways.
[This paper](https://www.ordnancesurvey.co.uk/documents/resources/guide-coordinate-systems-great-britain.pdf) from the UK's mapping authority the Ordnance Survey
gives an excellent introduction.

Since the satellites are orbiting around the Earth,
ECEF format is a fairly natural way to represent a position.
That uses cartesian coordinates measured in metres from the centre of the Earth.  As shown in the Ordnance Survey paper,
the X axis runs from the North Pole to the South Pole, the Y axis runs from the point where zero latitude meets the Equator
to the centre.  The Z axis is perpendicular to the other two.

The Earth is almost a sphere,
but not quite.
It spins on its axis, which stretches it slightly at the Equator,
making it a slightly irregular ellipsoid.
Geographers have defined a perfect ellipsoid that's close to the Earth's real shape.

There's another solid shape called the Geoid
(which is just "Earth-shaped" in Latin).
According to Wikipedia:
"The geoid is the shape that the ocean surface would take under the influence of the gravity of Earth, including gravitational attraction and Earth's rotation, if other influences such as winds and tides were absent. This surface is extended through the continents."
So the Ellipsoid is a regular solid that approximates to the shape of the Earth
and the Geoid is an irregular solid that also approximates to the shape of the Earth.
[This page](https://www.usgs.gov/faqs/what-geoid-why-do-we-use-it-and-where-does-its-shape-come)
has more details, including a useful diagram.

A position on the earth's surface can be represented as a longitude, a latitude and a height.
The height can be expressed as the geoidal height, the distance above or below the Geoid, the Ellipsoidal height, the distance above or below the Ellipsoid,
or the height above local Mean Sea Level (MSL).
The distance will be different in each case and it's easy to get confused between the three systems.

A GNSS device on the ground receives signals from the satellites and uses something like
trigonometry to find its position.  
Internally it uses ECEF notation,
but that can be converted directly to Longitude, latitude
and any of the three measures of height.
The device needs signals from 4 satellites to do that.  A multi-constellation receiver can use signals from all of the constellation.  In good conditions a receiver can see upwards of twenty satellites at any time.  

(I say "something like trigonometry" because the device uses the signals from the satellites
to find its position and the precise time.
You probably haven't noticed,
but the SatNav in your car always knows the right time
but you never have to set its clock.
Actually, it's probably the most accurate timepiece that you own.)

To do this, the device needs to receive signals from four satellites.
(Four data inputs to calculate four numbers - X,Y,Z and Time.)
On its own, the receiver can figure out its position to within a few metres.
Seeing more satellites allows a faster calculation but doesn't produce more accuracy.
This accuracy is acceptable for vehicle navigation
but other purposes such as land surveying
require greater accuracy.

More accuracy is possible using two receivers within a few kilometers of each other.
The signals from the positioning satellites suffer distortion, particularly as they pass through the ionosphere on their way to the Earth.
The receivers on the ground use these signals to figure out their position,
but the distortions produce inaccuracies.

About twenty years ago, Geoffrey Blewitt of Newcastle University
published his paper "Basics of the GPS Technique: Observation Equations".
He's now Professor at the Nevada Bureau of Mines and geology and you can download his paper from there:

https://nbmg.unr.edu/staff/pdfs/blewitt%20basics%20of%20gps.pdf

or just search for title on Google.

Some readers might find the maths in the paper a bit advanced,
so here is a simplified (actually oversimplified) explanation:

White light is composed of light of different colours,
each at a different frequency.
When a beam of white light passes through a prism,
the different beams are distorted in different ways
and the result is a patch of light broken up into a spectrum of colours.

When a signal from a satellite travels to a GNSS receiver on the ground
it passes through the ionosphere
and is distorted,
like the light passing through a prism.
The receiver uses the transit time of the signal
to calculate the distance to the satellite,
but the signal was distorted,
so the resulting distance is slightly wrong.
Without more information
the receiver can only estimate its position to within a few metres

The current generation of GNSS satellites broadcast signals on two frequency bands.
The ionosphere distorts signals on each frequency band differently,
so two signals sent by the satellite at the same time
arrive at the receiver at slightly different times.
So it ends up with two notions of that distance,
both slightly wrong.

Correcting the errors requires a receiver in a known fixed position
(a base station)
with a dual-band receiver so it can receive signals on both frequency bands.
It scans for signals from all the satellites it can see
and broadcasts
those data, plus its correct position.

Moving receivers (rovers) which are close to the base station receive signals from the same
satellites,
distorted in the same way.
A rover can use the information from the base station to correct its calculation of its own position.

The rover can do this even if its receiver is single-band,
and can only see some of the signals.
That's useful because single-band receivers are cheaper.

Some of the signals are sent in plain text,
others may be encrypted and meant for the owner's police force, military etc.
A receiver that you and I can buy
can't decrypt the encrypted signals but it can still see how the carrier wave is distorted.
Analysing those distortions is enough to better correct the effect of the ionosphere on the plain-text signals
that it can make understand.

In ideal conditions a rover within about 8km of a base station and receiving data from it
can estimate its position to within 2cm.
Beyond 8km, the accuracy falls linearly -
within 16km the rover can attain an accuracy of 4cm,
within 24km an accuracy of 8cm, and so on.
At about 64 km the RTCM data only allows about 2.5m accuracy,
which is close to what the rover can achieve without help.

The base station can send out its observations using Long Range Radio (LoRa)
or over the Internet using NTRIP
(Networked Transmission of RTCM over the Internet Protocol).
A rover used for surveying typically uses the surveyor's mobile phone to act
as an Internet modem.

Unfortunately, we don't live in ideal conditions.
Here in the real world we get less accuracy.
I'm currently trying to figure out how accurate my equipment is,
but that's work in progress.

Accurate GNSS systems are also easier and faster to use than theodolites.
The UK's mapping authority the Ordnance Survey has been using satellite positioning
for many years.
In the early days the equipment was expensive but now
a base and rover set can cost less than $1,000
and one base can support many rovers.

For RTCM correction to work properly, the operator has to figure out the base station's precise position.
If that's wrong by, say,
1.5m to the North,
then each each rover using that base station
will calculate positions that are wrong by the same amount.
The positions in the resulting survey will be accurate
relative to each other
but they will each be shifted from their true position by 1.5m to the North. 

So the problem becomes finding the position of the base station accurately.


The Caster
==========

To avoid the rover or the base station needing a fixed Internet (IP) address,
they communicate via an intermediate device called an NTRIP caster (broadcaster).
The caster is just a web server on the Internet so
it doesn't need to be physically close to the base station or the rover, 
it can be anywhere.

The caster offers a set of named endpoints.
Each rover connects to the caster and subscribes to an NTRIP feed from one endpoint.

In the simple case, each endpoint is fed by a single base station,
(so endpoint equals base station).
The base station reads signals from the satellites that it can see,
encodes them as RTCM messages, packages those up into NTRIP messages
and sends them over the Internet to its assigned endpoint on the caster.
The caster sends those messages on to any rovers that have subscribed to that endpoint.

```
                                                    NTRIP
                                                  ---------> rover
 ------------                       --------     /
| NTRIP base |      NTRIP          | NTRIP  | __/
| station    | ------------------> | caster |   \
 ------------                       --------     \  NTRIP
                                                  ---------> rover
```

(The more complicated case involves taking readings from many base stations spread over a wide area
and merging them together to create a virtual base station feeding a single endpoint.)


NTRIP on a budget
=================

A configuration that I've used is a GNSS receiver such as a Sparkfun
RTK board producing RTCM and other messages, connected via
I2C or serial USB to a host computer running my RTCM filter.  The
host could be a Windows machine but
a Raspberry Pi single board computer is cheaper and quite adequate:

```
 -------------          messages          --------------
| GPS device  | -----------------------> | Raspberry Pi |
 -------------    serial connection       --------------
```

With a bit more free software that can be used to create an NTRIP
base station.
The software for the base station is called an NTRIP server.
This github account contains a ready-made NTRIP server and an NTRIP caster,
derived from open source software
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

My rover is an Emlid Reach M+, which costs about $300 including antenna
and uses my smartphone to provide an Internet connection in the field. 


RTCM
=====

The RTCM protocol is currently at version 3.
RTCM3 messages are in a
compact binary form and not readable by eye.
The format is described by RTCM STANDARD 10403.3
Differential GNSS (Global Navigation Satellite Systems) Services –
Version 3 (RTCM3 10403.3).  This is not an open source standard.
It costs about $300 to buy a copy.  

The standard defines a large number of message types,
each with its own format.  Fortunately most of them are redundant.
A complete NTRIP service can be created using just six or seven types of message.

There is a little bit of useful information scattered around
various public web pages.  
There's also an open source library of C code to handle them, RTKLIB.
This was my main source of information about RTCM
and
I've copied some of
the more relevant RTKLIB source files into this repository as a handy reference
for other programmers.

There are already open-source tools available to convert an RTCM3 data stream into messages
in another format called RINEX.  That's an open standard and the result is human readable. 

To figure out the format of the RTCM message I'm
interested in, I read what I could find, including the RTKLIB source code.
Then I took the RTCM3 messages
that my device produced, used the tools to convert them into RINEX format
and checked that I got the same numbers in both cases.
These data form some of my unit and integration tests.

There are many hundreds of RTCM3 message types,
but they are not all required to find an accurate position,
and some that can be used for that are redundant,
replaced by Multiple Signal Messages (MSMs).
MSMs carry the observations of signals from satellites.
There is an MSM for each constellation, so I'm interested in
these message types:

* 1074 type 4 (low resolution) observations of signals from GPS satellites
* 1077 type 7 (high resolution) observations of signals from GPS satellites
* 1084 type 4 for GLONASS
* 1087 type 7 for GLONASS
* 1094 type 4 for Galileo
* 1097 type 7 for Galileo
* 1124 type 4 for Beidou
* 1127 type 7 for Beidou

A base station should be configured to send out either MSM4 or MSM7 messages.
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

UBlox advises that if a rover based on their technology uses GLONASS MSMs (mssage type 1084 or 1087)
the base station should also send
messages of type 1230
GLONASS code-phase bias values.
Unfortunately
the RTKLIB software doesn't have functionality to decode one of those,
so I have no idea what's in them.
(Mind you, the ones that my equipment receives
seem to contain just a string of zeroes.)


Apart from MSM material, a rover also needs type 1005 or 1006 messages,
which give the position of the base station.
(1005 gives just the position,
1006 gives the position plus the height of the base station above
the ground.)

The UBlox ZED-F9P Integration Manual (2022-02 edition) 
section 3.1.5.5.3 'Base station: RTCM output conﬁguration'
recommends that the base station sends these messages:

* RTCM 1005 Stationary RTK reference station ARP (base position)
* RTCM 1074 GPS MSM4
* RTCM 1084 GLONASS MSM4
* RTCM 1094 Galileo MSM4
* RTCM 1124 BeiDou MSM4
* RTCM 1230 GLONASS code-phase biases

As I said, MSM7 messages can be used instead of MSM4 - message types 1077, 1087, 1097 and 1127.
So a working fixed base station might be configured to send out messages of type 1005, 1074, 1084, 1094, 1124 and 1230.  Alternatively it could use high resolution MSMs, so 1005, 1077, 1087, 1097, 1127 and 1230.

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

The U-Blox Integration Manual also mentions two U-Blox proprietary message types:

* 4072.0 Reference station PVT and
* 4072.1 Additional reference station information,

It appears that these are only required to handle
a moving base station.
Type 4072.1 is only used by old firmware,
so we can ignore it.
Type 4072.0 appears to give the position of a moving base station as it moves
so if you have a fixed base station, it's irrelevant.

UBlox doesn't publish the format of those two message types
so my software doesn't interpret them.

The RTKLIB source code has functionality to decode most RTCM messages,
so if you need to understand other message types, go there.
However, that code can be difficult to follow and comments are scant.

RTCM Format
=======

An RTCM3 message is binary and variable length.  Each message frame
is composed of a three-byte header, an embedded message and 3 bytes of
Cyclic Redundancy Check (CRC) data.  The header starts with a 0xd3 byte and
includes the length of the embedded message.  Each embedded message starts with
a 12-bit message number which defines the type.  Apart from that
message number, each type of message is in a different format.

To avoid confusion between the complete RTCM message (header, embedded message
and trailing CRC) and the embedded message,
I talk about message frames and embedded messages. 

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

The fitst message starts at byte zero.  That byte has the value 0xd3
(hexadecimal number d3),
which
announces the start of the message frame.  The frame is composed of a
3-byte header, an embedded message and 3 bytes of Cyclic Redundancy
Check (CRC) data.

Byte 0 of an RTCM frame is always d3.  The top few bits of byte 1 are
always zero.  
The lower two bits of byte 1 and the bits of byte 2 form the 10-bit
message length, in this case hex 0aa, decimal 176.  So the embedded
message is 176 bytes long.  With the header and CRC
the whole message frame is 182
bytes long.

The first twelve bits of the embedded message are the message type.

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
message.  One clue is that it's not followed by a few zero bits.  To extract
a message frame from a stream of data and decode it, you need to read the
header and the next two bytes, check the header, find the message length,
read the whole message frame and check the CRC value.  This matters
particularly when you start to receive a stream of data from a device.  You
may come into the data stream part-way through and blunder into a d3 byte.
You can't assume that it's the start of a message.

The CRC data is also used to check that the message has not been corrupted in
transit.  If the CRC check fails, the message must be discarded.

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
and receive up to two signals from each satellite.  They typically see signals from 6-8 satellites
from each of the four constellations in each scan. 


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
on for GPS, one for Galileo, etc. 

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
so in UTC terms the timestamp rolls over on Saturday at 23:59:42,
18 seconds before midnight.
An extra leap
second may be added every four years.  The start of 2021 was a
candidate for adding another leap second but it was not necessary.
One may be added in 2025.

Galileo time is the same as GPS time.

At present (2022) the Beidou timestamp rolls over in UTC terms on Saturday at 23:59:56,
4 seconds before 
midnight.

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
start of week.
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
timestamp value will be quite large.  At 21:00 the week rolls
over and the timestamps will start again at (zero, zero).  Meanwhile
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
