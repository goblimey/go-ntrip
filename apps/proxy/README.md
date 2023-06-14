# proxy

A Man In The Middle (MITM) Proxy with Status Reporting, written in Go

This is a rework by Mark Rafter of an original version by Staaldraad.
The original is available [here](https://github.com/staaldraad/tcpprox)
and described [here](https://staaldraad.github.io/2016/12/11/tcpprox/).

This version uses the status-reporter to control logging levels and provide status reports.

## Installation

    git clone https://github.com/goblimey/go-ntrip.git
    cd go-ntrip/apps/proxy 
    go install


This produces a proxy program.


## Running the proxy

To run on the server named example.com, receiving HTTP requests on port 2102 and passing them onto a server on localhost port 2101.  Taking control requests on port 4001 and logging to ./proxy.log, initially quiet (log level 0):

    proxy -p -2102 -r localhost:2101 -l example.com -ca example.com -cp 4001 -q >proxy.log 2>&1 &


## Log Level

Set the log level to 1 on the proxy running on port 4001 on the server example.com:

    curl -X POST example.com:4001/status/loglevel/1

The log level value is 0-255.  0 turns logging off.  Any value above 0 turns logging on.


## Status Report

To produce a status report from the proxy running on example.com port 4001:

    curl example.com:4001/status/report

The report shows the data from the last request received and forwarded to the upstream server,  the response
from the upstream server, then a list of recent RTCM messages,
broken out into plain text where possible.

There may be more messages in the list than are in the last request.  Also, the report can only interpret complete messages, so if the data in the request includes part of a message at the end, that will not be shown in the list this time around.

