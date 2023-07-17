package main

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	circularQueue "github.com/goblimey/go-ntrip/apps/proxy/circular_queue"
	reportfeed "github.com/goblimey/go-ntrip/apps/proxy/reportfeed"
	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
	"github.com/goblimey/go-tools/dailylogger"
	reporter "github.com/goblimey/go-tools/statusreporter"
)

// Terminology:
// This is a Man In The Middle (MITM) NTRIP proxy intended to go between:
//
//		 an NTRIP Client on the (probably) local machine and
//		 an NTRP  Server on   a (probably) remote machine.
//
// The program variables and functions are named accordingly.
//
// To see the command line argument, run "proxy -h" or "proxy --help".
//
// Logging can be verbose or quiet.  It's verbose by default.  It can be set
// initially by options and at runtime by sending HTTP requests:
//    /status/loglevel/0
//    /status/loglevel/1
//
// The /status/report request displays the timestamp and contents of the last
// input and output buffers.

var reportFeed *reportfeed.ReportFeed

var byteChan chan byte

var rtcmHandler *rtcm.Handler

var messageChan chan rtcm.Message

const maxNumberOfMessagesStored = 20

var recentMessages *circularQueue.CircularQueue

var rtcmLog *dailylogger.Writer

func main() {

	// Handle command line arguments.
	localPortPtr := flag.Int("p", 0, "Local Port to listen on")
	nameOfLocalHostPtr := flag.String("l", "", "Local address to listen on")
	remoteHostPtr := flag.String("r", "", "Remote Server address host:port")
	configFilePtr := flag.String("c", "", "Use a config file (set TLS etc) - Commandline params overwrite config file")
	tlsPtr := flag.Bool("s", false, "Create a TLS Proxy")
	certFilePtr := flag.String("cert", "", "Use a specific certificate file")

	controlHostPtr := flag.String("ca", "", "hostname to listen on for status requests")
	controlPortPtr := flag.Int("cp", 0, "port to listen on for status requests")

	verbose := false
	flag.BoolVar(&verbose, "v", true, "verbose logging (shorthand)")
	flag.BoolVar(&verbose, "verbose", true, "verbose logging")

	quiet := false
	flag.BoolVar(&quiet, "q", false, "quiet logging (shorthand)")
	flag.BoolVar(&quiet, "quiet", false, "quiet logging")

	flag.Parse()

	// Non-zero config values from the command line override any config file.
	configFromCommandLine := Config{
		Localhost:   *nameOfLocalHostPtr,
		Localport:   *localPortPtr,
		Remotehost:  *remoteHostPtr,
		ControlHost: *controlHostPtr,
		ControlPort: *controlPortPtr,
		CertFile:    *certFilePtr,
	}

	configFile := *configFilePtr // Config file for TLS connection.
	isTLS := *tlsPtr             // If true, offer HTTPS, otherwise http.

	fmt.Printf("setting up config\n")
	SetConfig(configFile, &configFromCommandLine)

	// Ensure that the logging directory exists.
	if config.RecordMessages {
		_, err := os.Stat(config.MessageLogDirectory)
		if err != nil {
			if os.IsNotExist(err) {
				// The logging directory does not exist.  create it.
				createError := os.Mkdir(config.MessageLogDirectory, os.ModePerm)
				if createError != nil {
					panic(createError)
				}
			} else {
				// Some other error.
				panic(err)
			}
		}

		// Create the logger.
		rtcmLog = dailylogger.New(config.MessageLogDirectory, "data.", ".rtcm")
	}
	// Set the logging level.  It should be either quiet or verbose.
	if verbose {
		rtcmLog.EnableLogging()
	}
	if quiet {
		rtcmLog.DisableLogging() // quiet trumps verbose.
	}
	byteChan = make(chan byte)
	// Ensure that the byte channel is closed on return.
	defer close(byteChan)

	messageChan = make(chan rtcm.Message)
	// Ensure that the message channel is closed on return.
	defer close(messageChan)

	// Set up an RTCM handler and start it running.  It takes bytes
	// from the byte channel and turns them into messages on the
	// message channel.  The incoming data is sent to the byte channel
	// by handleClientMessages.
	rtcmHandler = rtcm.New(time.Now())
	go rtcmHandler.HandleMessages(byteChan, messageChan)

	// Create a circular queue to hold the recent messages from the message
	// channel and start the goroutine that keeps it up to date. The goroutine
	// reads messages from the message channel and puts them into the queue.
	// If the queue is full, the earliest message is discarded to make way
	// for the new one.  The report feed displays the messages currently in
	// the queue.
	recentMessages = circularQueue.NewCircularQueue(maxNumberOfMessagesStored)
	go keepCircularQueueUpdated(messageChan, recentMessages)

	// Set up the status reporter and the proxy server
	m := fmt.Sprintf("setting up status reporter - %s:%d", config.ControlHost, config.ControlPort)
	rtcmLog.Write([]byte(m))
	SetReportFeed(makeReporter(config.ControlHost, config.ControlPort, recentMessages))

	if config.Remotehost == "" {
		fmt.Fprintf(os.Stderr, "[x] Remote host required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Start the main server for NTRIP traffic.
	StartClientListener(isTLS)
}

// SetReportFeed sets the ReportFeed.
func SetReportFeed(feed *reportfeed.ReportFeed) {
	reportFeed = feed
}

// StartClientListener starts listening for traffic from the client.
func StartClientListener(isTLS bool) {

	client := connectToClient(isTLS)
	defer func() { client.Close() }()

	rtcmLog.Write([]byte("[*] Listening for Client call ...\n"))

	for {
		call, err := client.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to accept call from client: %s\n", err)
			break
		}
		id := ids
		ids++
		trace1 := []byte(fmt.Sprintf("[*][%d]connection Accepted from: client %s\n", id, call.RemoteAddr()))
		rtcmLog.Write(trace1)

		server := connectToServer(isTLS)
		trace2 := fmt.Sprintf("[*][%d] Connected to server: %s\n", id, server.RemoteAddr())
		rtcmLog.Write([]byte(trace2))

		go handleMessages(server, call, isTLS, id)
	}
}

func connectToClient(isTLS bool) (conn net.Listener) {
	var err error

	if isTLS {
		conn, err = tlsListen()
	} else {
		trace := fmt.Sprintf("listening on %s\n", fmt.Sprint(config.Localhost, ":", config.Localport))
		rtcmLog.Write([]byte(trace))
		conn, err = net.Listen("tcp", fmt.Sprint(config.Localhost, ":", config.Localport))
	}

	if err != nil {
		panic("failed to connect to client: " + err.Error())
	}

	return conn
}

func connectToServer(isTLS bool) (conn net.Conn) {
	var err error

	if isTLS {
		conf := tls.Config{InsecureSkipVerify: true}
		conn, err = tls.Dial("tcp", config.Remotehost, &conf)
	} else {
		conn, err = net.Dial("tcp", config.Remotehost)
	}

	if err != nil {
		panic("failed to connect to server: " + err.Error())
	}
	return conn
}

func handleMessages(server, client net.Conn, isTLS bool, id int) {

	// Next bit needs coordination?
	go handleServerMessages(server, client, id)
	handleClientMessages(server, client, id)
	server.Close()
	client.Close()
}

func handleClientMessages(server, client net.Conn, id int) {
	for {
		data := make([]byte, 2048)
		n, err := client.Read(data)
		if n > 0 {
			trace := fmt.Sprintf("From Client [%d]:\n%s\n", id, hex.Dump(data[:n]))
			rtcmLog.Write([]byte(trace))

			// Send contents of the buffer to the RTCM handler.
			for i := 0; i < n; i++ {
				byteChan <- data[i]
			}

			// Hang onto the buffer for reporting until the next one arrives
			reportFeed.RecordClientBuffer(&data, uint64(id), n)
			server.Write(data[:n])
		}
		if err != nil && err == io.EOF { // INCONSISTENT?
			fmt.Println(err)
			return
		}
	}
}

func handleServerMessages(server, client net.Conn, id int) {
	for {
		data := make([]byte, 2048)
		n, err := server.Read(data)
		if n > 0 {
			trace := fmt.Sprintf("From Server [%d]:\n%s\n", id, hex.Dump(data[:n]))
			rtcmLog.Write([]byte(trace))
			//fmt.Fprintf("From Server:\n%s\n",hex.EncodeToString(data[:n]))
			// Hang onto the buffer for reporting until the next one arrives
			reportFeed.RecordServerBuffer(&data, uint64(id), n)
			client.Write(data[:n])
		}
		if err != nil && err != io.EOF { // INCONSISTENT?
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			break
		}
	}
}

// SetConfig sets the proxy config - the server for which it acts as a proxy etc.
func SetConfig(configFile string, configFromCommandLine *Config) {
	if len(configFile) > 0 {
		file, err := os.Open(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] Cannot open config file: %s\n", err.Error())
			os.Exit(1)
		}

		parseError := SetConfigFromReader(file, configFromCommandLine)

		if parseError != nil {
			os.Exit(-1)
		}

	} else {
		config = Config{
			Localhost:  configFromCommandLine.Localhost,
			Localport:  configFromCommandLine.Localport,
			Remotehost: configFromCommandLine.Remotehost,
			TLS:        &TLS{},
		}
	}
	fmt.Printf("remote %s local host %s local port %d\n",
		config.Remotehost, config.Localhost, config.Localport)
}

// SetConfigFrom Reader sets the proxy config from data on a reader.
func SetConfigFromReader(configReader io.Reader, configFromCommandLine *Config) error {

	data := make([]byte, 4096)
	n, err := configReader.Read(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] Error reading config file: %s\n", err.Error())
		return err
	}

	fmt.Printf("data\n%s", string(data))

	parseError := parseConfig(data[:n], &config)
	if parseError != nil {
		fmt.Fprintf(os.Stderr, "[-] Not a valid config file: %s\n", parseError.Error())
		return err
	}

	// Non-Zero command line values override values in the config file.

	if len(configFromCommandLine.CertFile) > 0 {
		config.CertFile = configFromCommandLine.CertFile
	}
	if len(configFromCommandLine.Localhost) > 0 {
		config.Localhost = configFromCommandLine.Localhost
	}
	if configFromCommandLine.Localport != 0 {
		config.Localport = configFromCommandLine.Localport
	}
	if len(configFromCommandLine.Remotehost) > 0 {
		config.Remotehost = configFromCommandLine.Remotehost
	}
	if len(configFromCommandLine.ControlHost) > 0 {
		config.ControlHost = configFromCommandLine.ControlHost
	}
	if configFromCommandLine.ControlPort > 0 {
		config.ControlPort = configFromCommandLine.ControlPort
	}

	// Default settings.
	if config.Localport == 0 {
		config.Localport = 2101
	}
	if config.ControlPort == 0 {
		config.ControlPort = 8080
	}
	if len(config.ControlHost) == 0 {
		config.ControlHost = config.Localhost
	}

	return nil
}

func parseConfig(data []byte, config *Config) error {
	err := json.Unmarshal(data, config)
	if err != nil {
		return err
	}

	return nil
}

func makeReporter(controlHost string, controlPort int, queue *circularQueue.CircularQueue) *reportfeed.ReportFeed {
	rtcmLog.Write([]byte("setting up the status reporter\n"))

	rf := reportfeed.New(rtcmLog, queue)

	proxyReporter := reporter.MakeReporter(rf, controlHost, controlPort)

	proxyReporter.SetUseTextTemplates(true)

	// Start the HTTP server for control requests.
	go proxyReporter.StartService()

	return rf
}

// keepCircularQueueUpdated loops, reading messages from the message channel
// and putting them into the circular queue.  It terminates when the message
// queue is closed.  It can be run in a goroutine.
func keepCircularQueueUpdated(messageChan chan rtcm.Message, cq *circularQueue.CircularQueue) {
	// Fetch the messages and add them to the circular queue.
	for {
		message, more := <-messageChan
		if !more {
			break
		}

		cq.Add(message)

		rtcmLog.Write(message.RawData)
	}
}
