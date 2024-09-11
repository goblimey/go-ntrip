# proxy

A Man In The Middle (MITM) Proxy with Status Reporting, written in Go

This is based on a rework by Mark Rafter of an original version by Staaldraad.
The original is available [here](https://github.com/staaldraad/tcpprox)
and described [here](https://staaldraad.github.io/2016/12/11/tcpprox/).

This version uses the status-reporter to control logging levels and provide status reports.

## Installation

    git clone https://github.com/goblimey/go-ntrip.git
    cd go-ntrip
    cd apps
    cd proxy 
    go install

This produces a proxy program.


## Running the proxy

The proxy takes quite a lot of config.
This is best handled by packaging up the config in a file.
For example, create a file proxy.json containing:

```
{
	"remote_host": "localhost:2101",
	"proxy_host": "example.com",
	"proxy_port": 2102,
    "control_port": 4001,
	"record_messages": true,
	"message_log_directory": "./logs"
}
```

Run the proxy from the same directory as the config, like so:

    proxy -c proxy.json -q

With this config the proxy receives HTTP requests on port 2102 and passes them onto a caster running on the same machine as the proxy on port 2101.  It accepts control requests on port 4001 and makes a verbatim copy of the data that passes through in the file data.{date}.rtcm in the local directory logs:

    proxy -l example.com -p 2102 -r localhost:2101 -ca example.com -cp 4001 -q &

Alternatively, the proxy could be run on a local machine
such as
a laptop connected to a local WiFi network and the Internet via a mobile phone and sending traffic to a remote NTRIP caster.
In the example below the caster is running on port 2101 of
the server example.com.
The laptop has no domain name, just the IP address 172.20.10.6.  The proxy runs on port 2101 of the laptop and the reporting interface runs on port 4001:

```
{
	"remote_host": "example.com:2101",
	"proxy_host": "172.20.10.6",
	"proxy_port": 2101,
    "control_port": 4001,
	"record_messages": true,
	"message_log_directory": "./logs"
}
```

## Log Level

Set the log level to 1 on the proxy running on port 4001 on the server example.com:

    curl -X POST example.com:4001/status/loglevel/1

The log level value is 0-255.  0 turns logging off.  Any value above 0 turns logging on.


## Status Report

To produce a status report from the proxy running on port 4001 of the server example.com,
navigate your web browser to http://example.com:4001/status/report/.

To produce a status report from the proxy running on port 4001 of this computer,
navigate to http://localhost:4001/status/report/.

The report shows:

* the data from the last request received and forwarded to the upstream server
* the response from the upstream server
* a list of recent RTCM messages, broken out into plain text where possible.

There may be more messages in the list than are in the last request.  Also, the report can only interpret complete messages, so if the data in the request includes part of a message at the end, that will not be shown in the list of messages.

