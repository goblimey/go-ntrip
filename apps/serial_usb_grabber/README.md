# Serial USB Grabber

The serial_usb_grabber connects to a USB serial line,
reads data from it
and writes
that data to the standard output channel.
If the connection to the serial line is lost,
it tries repeatedly to reconnect and carry on.
Once started, it runs until it's
forcibly stopped. 

Many GNSS devices send data along a serial USB connection
at various speeds.
The standard Go file reader
can handle aserial USB connection, 
but only at 9600 baud.
If the port runs at any other speed,
the read consumes data from the port but misinterprets it,
and produces junk.

All other applications,
such as my rtcmfilter,
read a stream of data from their standard input channel,
so you can use this application to grab the data coming in 
and send it along a pipe for processing, like so: 
```
        Serial
        USB
        connection                   pipe          pipe
GNSS  |----------> serial_usb_grabber -> rtcmfilter -> |NTRIP
device|    data                                        |Client
```

The -c or --config options specify a JSON config file.
When the application starts up it looks for a JSON config file
containing something like:

```
{
    "speed": 115200,
    "read_timeout_milliseconds": 3000,
    "sleep_time_after_failed_open_milliseconds": 1000,
    "sleep_time_on_EOF_millis": 1000,
    "filenames": [
        "/dev/ttyACM0",
        "/dev/ttyACM1",
        "/dev/ttyACM2",
        "/dev/ttyACM3"
    ]
}
```

The read timeout and sleep_time_after_failed_open_milliseconds values relate to retries when attempting
to open files.
The sleep_time_on_EOF_millis
value relates to the handling of end of file while reading.

The input should be a device such as a
GNSS device emitting NTRIP and other messages.
The filename list may be the device name of a single
device or a list as above.

A host running MS Windows uses device names com2, com8
etc to represent the serial USB connection.  
A Debian Linux machine uses device
names /dev/ttyACM0, /dev/ttyACM1.
On both systems,
the file representing
the device doesn't exist until a real device is connected.

If the host computer and the GNSS source lose contact briefly (for
example because the source has lost power) the file connection may
break and need to be re-opened.

When the connection is a serial USB channel, that process is a  little
complicated.
Neither Windows nor Linux use one
device name per physical port.  The device file is created when the
GNSS device is plugged into one of the host's USB ports.  If the
connection is lost later, the device file representing it disappears.
If the connection is then restored on the same port, the system may
(or may not)
use one of the other device names.

During my testing on a Debian Linux system with four physical USB
sockets, the system would create one of four devices to handle the
traffic, /dev/ttyACM[0-4].  The first time I connected the GNSS
device, the Linux system created /dev/ttyACM0 to handle the traffic.
I disconnected the device and reconnected it on the same USB socket.
The host created /dev/ttyACM1.  On each disconnection and
reconnection, it cycled around the four device names.

However, a different device consistently used /dev/ttyACM0 over
many disconnections and reconnections.
I suspect that the person writing the device's firmware may have some control over what 
device name is used.

Similar experiments with a Windows host machine produced similar
behaviour - the operating system cycled round a list of device
names: com1, com8 and so on.  If you use a Windows host you will
need to experiment to figure out 
what to put into your configuration file.
Note also
that plugging in another USB device later, such as a mouse, may
disturb the sequence, so set up all your USB devices first and
leave them that way.

When I ran experiments with a Raspberry Pi as the host and a
SparkFun GNSS board as the source I found that the connection
was dropped and
reconnected fairly regularly.  At the time, My Sparkfun board drew
its power from the Pi via the USB line and my guess was that the
Pi couldn't always supply sufficient power.  That would cause the
Sparkfun board to shut down briefly until the Pi restored the
power.  Now I connect the two via a powered USB hub, which should
make the setup more stable. Time will tell.

If your host machine has other devices that use a serial USB
connection it may be difficult or impossible to predict which
device filename will get used when the GNSS device connects.
That's one reason why I recommend
using a Raspberry Pi as a host machine - 
it's cheap enough that you can
dedicate it to handling your GPS device.


A GNSS base station should be configured to send a batch of messages
every second,so there should only be a delay of a fraction of a
second between each batch.  If there is a longer delay, then the
connection between the host machine running the grabber and the GNSS
device has probably died.  The grabber will close the input channel
and attempt (silently)
to reopen it and continue. The timeout and retry values
in the JSON control this behavior.  You should tune the values to
suit your equipment.

(The BKG NTRIP client software
can be connected directly to a GNSS device
and it uses a read timeout of three seconds.
If no data arrives within three seconds,
it closes the connection and reopens it.)

## Why Write This?

My original implementation of the go-ntrip applications
used the standard Go file reader
to read from a USB connection.
This worked at first
because all my devices transmitted
at 9600 baud.
I hit problems with a GNSS device that transmitted data at
115200 baud. 

The [bug.st serial package](https://github.com/bugst/go-serial) 
provides a reader that seems to handle serial USB connections 
properly at all speeds.
It avoids any dependency on C libraries and CGO
so it can be cross-compiled to most operating systems,
except for the Apple Mac.
(To run it on a Mac you need to build it on a Mac.)

The problem with this package is testing.
The author's README admits that his testing is insufficient.
Also, it's not practical to use unit tests for most of the functionality,
as you need some sort of real device transmitting
data on a port.
Testing edge cases such as what happens on EOF
is also difficult.

Having tested the bug.st package manually
as best I can, I'm happy to use it
but I don't want to embed it in lots of applications.
Instead, I've written this one
which just handles the serial connection and
nothing more.
If I encounter any serious problems, 
I can look round for another solution
and reimplement this application using that.
