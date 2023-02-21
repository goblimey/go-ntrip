package message

import (
	"fmt"
	"testing"
	"time"

	"github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/msm4/signal"
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

// createmessage is a helper function.  It creates a message with known contents.
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

	if gotMessage.Header.EpochTime != wantEpochTime {
		t.Errorf("want %d got %d", wantEpochTime, gotMessage.Header.EpochTime)
	}

	if !gotMessage.Header.MultipleMessage {
		t.Errorf("want %v got %v", wantMultipleMessage, gotMessage.Header.MultipleMessage)
	}

	if gotMessage.Header.IssueOfDataStation != wantIssue {
		t.Errorf("want %d got %d", wantIssue, gotMessage.Header.IssueOfDataStation)
	}
	if gotMessage.Header.EpochTime != wantEpochTime {
		t.Errorf("want %d got %d", wantEpochTime, gotMessage.Header.EpochTime)
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

func TestString(t *testing.T) {
	const resultTemplateComplete = `type 1074 GPS Full Pseudoranges and PhaseRanges plus CNR
time 2023-02-14 01:02:03.004 +0000 UTC (epoch time 2)
stationID 1, multiple message, sequence number 3, session transmit time 4
clock steering 5, external clock 6
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
time 2023-02-14 01:02:03.004 +0000 UTC (epoch time 2)
stationID 1, multiple message, sequence number 3, session transmit time 4
clock steering 5, external clock 6
divergence free smoothing true, smoothing interval 7
2 satellites, 3 signal types, 6 signals
No Satellites
No Signals
`

	locationUTC, _ := time.LoadLocation("UTC")
	timestamp :=
		time.Date(2023, time.February, 14, 1, 2, 3, int(4*time.Millisecond), locationUTC)

	// MesageComplete has a header, UTCTime, satellites and Signals.
	messageComplete := createMessage()
	messageComplete.Header.UTCTime = timestamp

	// messageIncomplete has just a header and UTCTime
	messageIncomplete := createMessage()
	messageIncomplete.Satellites = nil
	messageIncomplete.Signals = nil
	messageIncomplete.Header.UTCTime = timestamp

	// The expected approximate range given by the satellite cell.
	approxRange := utils.GetApproxRange(wantRangeWhole, wantRangeFractional)

	rangefromSignal := messageComplete.Signals[0][0].RangeInMetres()

	phaseRangefromSignal := messageComplete.Signals[0][0].PhaseRange()

	wantComplete :=
		fmt.Sprintf(resultTemplateComplete, approxRange, rangefromSignal, phaseRangefromSignal)

	var testData = []struct {
		description string
		message     *Message
		want        string
	}{
		{"complete", messageComplete, wantComplete},
		{"incomplete", messageIncomplete, wantIncomplete},
	}

	for _, td := range testData {
		got := td.message.String()

		if td.want != got {
			t.Error(diff.Diff(td.want, got))
		}
	}
}

func TestGetMessage(t *testing.T) {

	bitStream := []byte{
		// The header is 185 bits long, with 2 cell mask bits.
		// The type is 1074, Station ID is 1:
		// 0: 1000 0110 010|0000 0000 0001
		0x43, 0x20, 0x01, 0x00, 0x00, 0x00, 0x04, 0x00,
		//                   64 bit satellite mask with satellite 4 marked.
		// 64: 000|0 0|0|00   0|000 1000   0000 0000   0000 0000 ...
		0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		//               32 bit signal mask with signals 2 and 16 marked.
		// 0000 0000   0|010 0000   0000 0000   1000 0000
		/* 128 */ 0x00, 0x20, 0x00, 0x80,
		//
		//                    2 bit cell mask
		//                       Satellite cell - whole 1, frac 0.25
		//                                                   Signal cell
		// 160: 0000 0000   0|11|0 0000  001|0 1000   0000 0|000
		0x00, 0x60, 0x28, 0x00,
		// 192: 0100 0000   0000|0001   0000 0000   000|0 0010
		0x40, 0x01, 0x00, 0x02,
		// 224: 0000 0000   0000 0000  0|100 0000   0000 0000
		0x00, 0x00, 0x40, 0x00,
		// 276: 0000 000|0   011|0 100|0|  1|000 111|0  1000 0|000
		0x00, 0x68, 0x8e, 0x80,
	}

	const wantWholeMillis = 1
	const wantFractionalMillis = 0x100 // 0001 0000 0000

	message, err := GetMessage(bitStream)

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
