package jsonconfig

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goblimey/go-tools/switchwriter"
	"github.com/goblimey/go-tools/testsupport"
)

var logger *log.Logger

func init() {
	writer := switchwriter.New()
	logger = log.New(writer, "rtcm_test", 0)
}

// TestWaitAndConnectToInput tests that waitAndConnectToInput returns a
// reader connected to the correct file when the file does not exist
// initially.
//
func TestWaitAndConnectToInput(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	// The filename list in the config contains "a", "b" and "c"
	filenames := make([]string, 0)
	filenames = append(filenames, "a")
	filenames = append(filenames, "b")
	filenames = append(filenames, "c")
	config := Config{Filenames: filenames, LostInputConnectionTimeout: 1,
		LostInputConnectionSleepTime: 1}
	config.SystemLog = logger

	// Wait for a short time and then create file "b" with some contents.
	const expectedContents = "Hello world"
	creator := func() {
		time.Sleep(2 * time.Second)
		// To avoid a race while writing, create "t", write to it and
		// then rename it.  The test won't notice it until it's renamed.
		writer, err := os.Create("t")
		if err != nil {
			log.Fatal(err)
		}
		writer.Write([]byte(expectedContents))
		err = os.Rename("t", "b")
		if err != nil {
			log.Fatal(err)
		}

	}
	go creator()

	// File b doesn't exist at first when this is called.  It should spin
	// and, once file "b" appears, open it for reading.
	ctx := context.Background()
	reader := WaitAndConnectToInput(ctx, &config)
	if reader == nil {
		log.Fatalf("findInputDevice returns nil, should open \"b\" for reading")
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	// The contents read should match the expectedContents that was written.
	if expectedContents != string(contents) {
		t.Fatalf("expected %s, got %s", expectedContents, string(contents))
	}
}

// TestFindInputDevice tests that findInputDevice returns a
// reader connected to the correct file.
func TestFindInputDevice(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	configReader := strings.NewReader(`{
		"input": ["a", "b", "c"]
	}`)

	w := switchwriter.New()
	logger := log.New(w, "jsonconfig_test", 0)

	config, err := getJSONConfig(configReader, logger)
	if err != nil {
		t.Fatal(err)
	}

	if config == nil {
		t.Fatal("parsing json failed - nil")
	}

	// Create file "b" with some contents.
	writer, err := os.Create("b")
	if err != nil {
		log.Fatal(err)
	}
	const expectedContents = "Hello world"
	writer.Write([]byte(expectedContents))

	// This should open file "b" for reading.
	ctx := context.Background()
	reader := findInputDevice(ctx, config)
	if reader == nil {
		log.Fatalf("findInputDevice returns nil, should open \"b\" for reading")
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	// The contents read should match the expectedContents that was written.
	if expectedContents != string(contents) {
		t.Fatalf("expected %s, got %s", expectedContents, string(contents))
	}
}

// TestGetInputFile tests that TestGetInputFile scans
// correctly for the files in its list.
func TestGetInputFile(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	// The filename list contains "a", "b" and "c"
	configReader := strings.NewReader(`{
		"input": ["a", "b", "c"]
	}`)

	w := switchwriter.New()
	logger := log.New(w, "jsonconfig_test", 0)

	config, err := getJSONConfig(configReader, logger)
	if err != nil {
		t.Fatal(err)
	}

	if config == nil {
		t.Fatal("parsing json failed - nil")
	}

	// Create files "a", "b", "c" and "d".
	os.Create("a")
	os.Create("b")
	os.Create("c")
	os.Create("d")

	// This call should return file "a".
	file := getInputFile(config)
	if file.Name() != "a" {
		t.Errorf("expected file a, got %s", file.Name())
	}

	// Remove "a" and the call should return file "b".
	os.Remove("a")
	file = getInputFile(config)
	if file.Name() != "b" {
		t.Errorf("expected file b, got %s", file.Name())
	}

	// Remove "b" and the call should return file "c".
	os.Remove("b")
	file = getInputFile(config)
	if file.Name() != "c" {
		t.Errorf("expected file c, got %s", file.Name())
	}

	// Remove "c" and with no matching files, this call should
	// return nil, not "d".
	os.Remove("c")
	file = getInputFile(config)
	if file != nil {
		t.Errorf("expected nil, got file %s", file.Name())
	}
}

// TestGetJSONConfigFromFile tests that getJSONConfigFromFile
// reads a config file correctly.
func TestGetJSONConfigFromFile(t *testing.T) {

	workingDirectory, err := testsupport.CreateWorkingDirectory()
	if err != nil {
		t.Errorf("createWorkingDirectory failed - %v", err)
	}
	defer testsupport.RemoveWorkingDirectory(workingDirectory)

	// Create the JSON control file.
	fileContents := `{
		"input": ["a", "b"],
		"sendtocaster": true,
		"writeoutputlog": true,
		"writereadablelog": false,
		"casterhostname": "caster.example.com",
		"casterport": 2101,
		"casterUserName": "user",
		"casterPassword": "password",
		"timeout": 1,
		"sleeptime": 2
	}`
	controlFileName := "config.json"

	controlFile, err := os.Create(controlFileName)
	if err != nil {
		t.Fatal(err)
	}

	_, err = controlFile.Write([]byte(fileContents))
	if err != nil {
		t.Fatal(err)
	}

	config, err := GetJSONConfigFromFile(controlFileName, logger)
	if err != nil {
		t.Fatal(err)
	}

	if config == nil {
		t.Fatal("parsing json failed - nil")
	}

	numFiles := len(config.Filenames)
	if numFiles != 2 {
		t.Fatalf("parsing json, expected 2 files, got %d", numFiles)
	}

	if config.Filenames[0] != "a" {
		t.Errorf("parsing json, expected file 0 to be a, got %s",
			config.Filenames[0])
	}

	if config.Filenames[1] != "b" {
		t.Errorf("parsing json, expected file 1 to be b, got %s",
			config.Filenames[1])

	}
	if config.CasterUserName != "user" {
		t.Errorf("parsing json, expected caster username to be user, got %s",
			config.CasterUserName)
	}
	if config.CasterPassword != "password" {
		t.Errorf("parsing json, expected caster password to be password, got %s",
			config.CasterPassword)
	}

	if !config.SendToCaster {
		t.Error("parsing json, expected sendToCaster to be true, got false")
	}

	if !config.WriteOutputLog {
		t.Error("parsing json, expected outputLog to be true, got false")
	}

	if config.WriteReadableLog {
		t.Error("parsing json, expected readableLog to be false, got true")
	}
}
