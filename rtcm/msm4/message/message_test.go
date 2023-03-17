package message

import (
	"fmt"
	"testing"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/msm4/signal"
	"github.com/goblimey/go-ntrip/rtcm/testdata"
	"github.com/goblimey/go-ntrip/rtcm/utils"

	"github.com/kylelemons/godebug/diff"
)

const wantSatelliteMask = 3
const wantSignalMask = 7
const wantCellMask = 1
const wantMessageType = 1074
const wantStationID = 1
const wantEpochTime = 2
const wantMultipleMessage = true
const wantIssue = 3
const wantTransTime = 4
const wantClockSteeringIndicator = 5
const wantExternalClockSteeringIndicator = 6
const wantSmoothing = true
const wantSmoothingInterval = 7

const wantSatelliteID = 8
const wantRangeWhole uint = 9
const wantRangeFractional uint = 10

const wantSignalID = 11
const wantRangeDelta = 12
const wantPhaseRangeDelta = 13
const wantLockTimeIndicator = 14
const wantHalfCycleAmbiguity = true
const wantCNR = 15
const wantWavelength = 16.0

// createMessage is a helper function.  It creates a message with known contents.
func createMessage() *Message {
	h := header.New(wantMessageType, wantStationID, wantEpochTime, wantMultipleMessage,
		wantIssue, wantTransTime, wantClockSteeringIndicator,
		wantExternalClockSteeringIndicator, true, wantSmoothingInterval,
		wantSatelliteMask, wantSignalMask, wantCellMask)

	sat := satellite.New(wantSatelliteID, wantRangeWhole, wantRangeFractional)
	sats := []satellite.Cell{*sat}

	sig := signal.New(wantSignalID, sat, wantRangeDelta, wantPhaseRangeDelta,
		wantLockTimeIndicator, wantHalfCycleAmbiguity, wantCNR, wantWavelength)
	sigs := [][]signal.Cell{{*sig}}

	return New(h, sats, sigs)
}
func TestNew(t *testing.T) {

	gotMessage := createMessage()

	if gotMessage.Header.MessageType != wantMessageType {
		t.Errorf("want %d got %d", wantMessageType, gotMessage.Header.MessageType)
	}

	if gotMessage.Header.StationID != wantStationID {
		t.Errorf("want %d got %d", wantStationID, gotMessage.Header.StationID)
	}

	if gotMessage.Header.Timestamp != wantEpochTime {
		t.Errorf("want %d got %d", wantEpochTime, gotMessage.Header.Timestamp)
	}

	if !gotMessage.Header.MultipleMessage {
		t.Errorf("want %v got %v", wantMultipleMessage, gotMessage.Header.MultipleMessage)
	}

	if gotMessage.Header.IssueOfDataStation != wantIssue {
		t.Errorf("want %d got %d", wantIssue, gotMessage.Header.IssueOfDataStation)
	}
	if gotMessage.Header.Timestamp != wantEpochTime {
		t.Errorf("want %d got %d", wantEpochTime, gotMessage.Header.Timestamp)
	}

	if gotMessage.Header.ClockSteeringIndicator != wantClockSteeringIndicator {
		t.Errorf("want %d got %d", wantClockSteeringIndicator, gotMessage.Header.ClockSteeringIndicator)
	}
	if gotMessage.Header.ExternalClockSteeringIndicator != wantExternalClockSteeringIndicator {
		t.Errorf("want %d got %d", wantExternalClockSteeringIndicator, gotMessage.Header.ExternalClockSteeringIndicator)
	}
	if !gotMessage.Header.GNSSDivergenceFreeSmoothingIndicator {
		t.Errorf("want %v got %v", wantSmoothing, gotMessage.Header.GNSSDivergenceFreeSmoothingIndicator)
	}
	if gotMessage.Header.GNSSSmoothingInterval != wantSmoothingInterval {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotMessage.Header.GNSSSmoothingInterval)
	}
	if gotMessage.Header.GNSSSmoothingInterval != wantSmoothingInterval {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotMessage.Header.GNSSSmoothingInterval)
	}
	if gotMessage.Header.SatelliteMask != wantSatelliteMask {
		t.Errorf("want %d got %d", wantSatelliteMask, gotMessage.Header.SatelliteMask)
	}
	if gotMessage.Header.SignalMask != wantSignalMask {
		t.Errorf("want %d got %d", wantSignalMask, gotMessage.Header.SignalMask)
	}
	if gotMessage.Header.CellMask != wantCellMask {
		t.Errorf("want %d got %d", wantSmoothingInterval, gotMessage.Header.CellMask)
	}

	// Check satellite slice
	if len(gotMessage.Satellites) != 1 {
		t.Errorf("want 1 satellite, got %d", len(gotMessage.Satellites))
	}

	if gotMessage.Satellites[0].SatelliteID != wantSatelliteID {
		t.Errorf("want satelliteID %d, got %d",
			wantSatelliteID, gotMessage.Satellites[0].SatelliteID)
	}

	if gotMessage.Satellites[0].RangeWholeMillis != wantRangeWhole {
		t.Errorf("want satellite range whole %d, got %d",
			wantRangeWhole, gotMessage.Satellites[0].RangeWholeMillis)
	}

	if gotMessage.Satellites[0].RangeFractionalMillis != wantRangeFractional {
		t.Errorf("want satellite range fractional %d, got %d",
			wantRangeFractional, gotMessage.Satellites[0].RangeFractionalMillis)
	}

	// Check signal Slice

	if len(gotMessage.Signals) != 1 {
		t.Errorf("want 1 signal slice, got %d", len(gotMessage.Signals))
	}

	if len(gotMessage.Signals[0]) != 1 {
		t.Errorf("want 1 signal, got %d", len(gotMessage.Signals[0]))
	}

	if gotMessage.Signals[0][0].SatelliteID != wantSatelliteID {
		t.Errorf("want satelliteID %d, got %d",
			wantSatelliteID, gotMessage.Signals[0][0].SatelliteID)
	}

	if gotMessage.Signals[0][0].SignalID != wantSignalID {
		t.Errorf("want signalID %d, got %d",
			wantSignalID, gotMessage.Signals[0][0].SignalID)
	}

	if gotMessage.Signals[0][0].SignalID != wantSignalID {
		t.Errorf("want signalID %d, got %d",
			wantSignalID, gotMessage.Signals[0][0].SignalID)
	}

	if gotMessage.Signals[0][0].RangeWholeMillisFromSatelliteCell != wantRangeWhole {
		t.Errorf("want range whole %d, got %d",
			wantRangeWhole, gotMessage.Signals[0][0].RangeWholeMillisFromSatelliteCell)
	}

	if int(gotMessage.Signals[0][0].RangeFractionalMillisFromSatelliteCell) != int(wantRangeFractional) {
		t.Errorf("want range fractional %d, got %d",
			wantRangeFractional, gotMessage.Signals[0][0].RangeFractionalMillisFromSatelliteCell)
	}

	if gotMessage.Signals[0][0].RangeDelta != wantRangeDelta {
		t.Errorf("want range delta %d, got %d",
			wantRangeDelta, gotMessage.Signals[0][0].RangeDelta)
	}
	if gotMessage.Signals[0][0].PhaseRangeDelta != wantPhaseRangeDelta {
		t.Errorf("want phase range delta %d, got %d",
			wantPhaseRangeDelta, gotMessage.Signals[0][0].PhaseRangeDelta)
	}
	if gotMessage.Signals[0][0].LockTimeIndicator != wantLockTimeIndicator {
		t.Errorf("want lock tome ind %d, got %d",
			wantLockTimeIndicator, gotMessage.Signals[0][0].LockTimeIndicator)
	}
	if gotMessage.Signals[0][0].HalfCycleAmbiguity != wantHalfCycleAmbiguity {
		t.Errorf("want half cycle ambiguity %v, got %v",
			wantHalfCycleAmbiguity, gotMessage.Signals[0][0].HalfCycleAmbiguity)
	}
	if gotMessage.Signals[0][0].CarrierToNoiseRatio != wantCNR {
		t.Errorf("want CNR %d, got %d",
			wantCNR, gotMessage.Signals[0][0].CarrierToNoiseRatio)
	}
	if gotMessage.Signals[0][0].Wavelength != wantWavelength {
		t.Errorf("want wavelength %f, got %f",
			wantWavelength, gotMessage.Signals[0][0].Wavelength)
	}
}

// TstString checks the String method.
func TestString(t *testing.T) {
	const resultTemplateComplete = `type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
stationID 1, timestamp 2, multiple message, sequence number 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
1 Satellites
Satellite ID {range ms}
 8 {%.3f}
1 Signals
Sat ID Sig ID {range (delta), lock time ind, half cycle ambiguity, Carrier Noise Ratio}
 8 11 {%.3f, %.3f, 14, true, 15}
`

	const wantIncomplete = `type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
stationID 1, timestamp 2, multiple message, sequence number 3
session transmit time 4, clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
No Satellites
No Signals
`
	// completeMessage has a header, satellites and Signals.
	completeMessage := createMessage()

	// incompleteMessage has just a header
	incompleteMessage := createMessage()
	incompleteMessage.Satellites = nil
	incompleteMessage.Signals = nil

	// The expected approximate range given by the satellite cell.
	approxRange := utils.GetApproxRangeMetres(wantRangeWhole, wantRangeFractional)

	rangefromSignal := completeMessage.Signals[0][0].RangeInMetres()

	phaseRangefromSignal := completeMessage.Signals[0][0].PhaseRange()

	wantComplete :=
		fmt.Sprintf(resultTemplateComplete, approxRange, rangefromSignal, phaseRangefromSignal)

	var testData = []struct {
		description string
		message     *Message
		want        string
	}{
		{"complete", completeMessage, wantComplete},
		{"incomplete", incompleteMessage, wantIncomplete},
	}

	for _, td := range testData {
		got := td.message.String()

		if td.want != got {
			t.Error(diff.Diff(td.want, got))
		}
	}
}

func TestGetMessage(t *testing.T) {

	const wantWholeMillis = 1
	const wantFractionalMillis = 0x100 // 0001 0000 0000

	message, err := GetMessage(testdata.MessageType1074)

	if err != nil {
		t.Error(err)
	}

	// Check the header.

	if message.Header.MessageType != 1074 {
		t.Errorf("want type 1074 got %d", message.Header.MessageType)
	}

	if message.Header.Constellation != "GPS" {
		t.Errorf("want GPS got %s", message.Header.Constellation)
	}

	if message.Header.StationID != 1 {
		t.Errorf("want station ID 1 got %d", message.Header.StationID)
	}

	if message.Header.MultipleMessage {
		t.Error("want single message got multiple")
	}

	if message.Header.IssueOfDataStation != 0 {
		t.Errorf("want IODS 0 got %d", message.Header.IssueOfDataStation)
	}

	if message.Header.SessionTransmissionTime != 0 {
		t.Errorf("want SessionTransmissionTime 0 got %d",
			message.Header.SessionTransmissionTime)
	}

	if message.Header.ClockSteeringIndicator != 0 {
		t.Errorf("want ClockSteeringIndicator 0 got %d",
			message.Header.ClockSteeringIndicator)
	}

	if message.Header.ExternalClockSteeringIndicator != 0 {
		t.Errorf("want ExternalClockSteeringIndicator 0 got %d",
			message.Header.ExternalClockSteeringIndicator)
	}

	if message.Header.GNSSDivergenceFreeSmoothingIndicator {
		t.Error("want GNSSDivergenceFreeSmoothingIndicator false got true")
	}

	if message.Header.GNSSSmoothingInterval != 0 {
		t.Errorf("want GNSSSmoothingInterval 0 got %d",
			message.Header.GNSSSmoothingInterval)
	}

	// Check the satellite cells.
	if len(message.Satellites) != 1 {
		t.Errorf("want 1 satellite bit got %d", len(message.Satellites))
	}

	if message.Satellites[0].SatelliteID != 4 {
		t.Errorf("want satellite ID 4 got %d",
			message.Satellites[0].SatelliteID)
	}

	if message.Satellites[0].RangeWholeMillis != wantWholeMillis {
		t.Errorf("want %d got %d",
			wantWholeMillis, message.Satellites[0].RangeWholeMillis)
	}

	if message.Satellites[0].RangeFractionalMillis != wantFractionalMillis {
		t.Errorf("want 0x%x got 0x%x",
			wantFractionalMillis, message.Satellites[0].RangeFractionalMillis)
	}

	// Check the signal cells.
	if len(message.Signals) != 1 {
		t.Errorf("want 1 got %d", len(message.Signals))
	}

	if len(message.Signals[0]) != 2 {
		t.Errorf("want 2 got %d", len(message.Signals[0]))
	}

	if message.Signals[0][0].SatelliteID != 4 {
		t.Errorf("want 4 got %d", message.Signals[0][0].SatelliteID)
	}

	if message.Signals[0][0].SignalID != 2 {
		t.Errorf("want 2 got %d", message.Signals[0][0].SignalID)
	}

	if message.Signals[0][1].SatelliteID != 4 {
		t.Errorf("want 4 got %d", message.Signals[0][1].SatelliteID)
	}

	if message.Signals[0][1].SignalID != 16 {
		t.Errorf("want 16 got %d", message.Signals[0][1].SignalID)
	}

	if message.Signals[0][0].RangeWholeMillisFromSatelliteCell != wantWholeMillis {
		t.Errorf("want %d got %d",
			wantWholeMillis, message.Signals[0][0].RangeWholeMillisFromSatelliteCell)
	}

	if message.Signals[0][0].RangeFractionalMillisFromSatelliteCell != wantFractionalMillis {
		t.Errorf("want %d got %d",
			wantFractionalMillis, message.Signals[0][0].RangeFractionalMillisFromSatelliteCell)
	}

	if message.Signals[0][1].RangeWholeMillisFromSatelliteCell != wantWholeMillis {
		t.Errorf("want 0x%x got 0x%x",
			wantWholeMillis, message.Signals[0][0].RangeWholeMillisFromSatelliteCell)
	}

	if message.Signals[0][1].RangeFractionalMillisFromSatelliteCell != wantFractionalMillis {
		t.Errorf("want %d got %d",
			wantFractionalMillis, message.Signals[0][1].RangeFractionalMillisFromSatelliteCell)
	}

	if message.Signals[0][0].RangeDelta != 1024 {
		t.Errorf("want 1024 got %d", message.Signals[0][0].RangeDelta)
	}

	if message.Signals[0][1].RangeDelta != 2048 {
		t.Errorf("want 2048 got %d", message.Signals[0][1].RangeDelta)
	}

	if message.Signals[0][0].PhaseRangeDelta != 0x40000 {
		t.Errorf("want 1024 got %d", message.Signals[0][0].PhaseRangeDelta)
	}

	if message.Signals[0][1].PhaseRangeDelta != utils.InvalidPhaseRangeDelta {
		t.Errorf("want %d got %d",
			utils.InvalidPhaseRangeDelta, message.Signals[0][1].PhaseRangeDelta)
	}

	if message.Signals[0][0].LockTimeIndicator != 3 {
		t.Errorf("want 3 got %d", message.Signals[0][0].PhaseRangeDelta)
	}

	if message.Signals[0][1].LockTimeIndicator != 4 {
		t.Errorf("want 4 got %d",
			message.Signals[0][1].LockTimeIndicator)
	}

	if message.Signals[0][0].HalfCycleAmbiguity {
		t.Errorf("want false got %v", message.Signals[0][0].HalfCycleAmbiguity)
	}

	if !message.Signals[0][1].HalfCycleAmbiguity {
		t.Errorf("want true got %v", message.Signals[0][1].HalfCycleAmbiguity)
	}

	if message.Signals[0][0].CarrierToNoiseRatio != 7 {
		t.Errorf("want 7 got %d", message.Signals[0][0].CarrierToNoiseRatio)
	}

	if message.Signals[0][1].CarrierToNoiseRatio != 16 {
		t.Errorf("want 16 got %d",
			message.Signals[0][1].CarrierToNoiseRatio)
	}
}

// TestGetMessageWithErrors checks that GetMessage handles errors correctly.
func TestGetMessageWithErrors(t *testing.T) {
	// GetMessage responds with an error message at various points.

	var testData = []struct {
		Description string
		BitStream   []byte
		Want        string
	}{
		{
			"header too short", testdata.MessageType1074[:21],
			"bitstream is too short for an MSM header - got 168 bits, expected at least 169",
		},
		{
			"satellite cells too short", testdata.MessageType1074[:23],
			"overrun - not enough data for 1 MSM4 satellite cells - need 18 bits, got 13",
		},
		{
			"Signal cells too short", testdata.MessageType1074[:30],
			"overrun - want 2 MSM4 signals, got 1",
		},
		{
			"not MSM4", testdata.Message1077,
			"message type 1077 is not an MSM4",
		},
	}
	for _, td := range testData {
		gotMessage, gotError := GetMessage(td.BitStream)
		if gotMessage != nil {
			t.Error("On error, the message should be nil")
		}
		if gotError == nil {
			t.Error("expected an error")
		} else {
			if gotError.Error() != td.Want {
				t.Errorf("%s:\nwant %s\n got %s", td.Description, td.Want, gotError.Error())
			}
		}
	}
}
