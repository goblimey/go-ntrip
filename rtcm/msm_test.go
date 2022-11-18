package rtcm

import (
	"fmt"
	"testing"
	"time"
)

// These tests check the analysis of MSM messages.

func TestCreateMSMSatelliteCell(t *testing.T) {

	const rangeWhole uint = 1
	const rangeFractional uint = 2
	const extendedInfo uint = 3
	const phaseRangeRate = 4
	const invalidRange = 0xff

	invalidPhaseRangeRateBytes := []byte{0x80, 00} // 14 bits plus filler: 1000 0000 0000 00 filler 00
	invalidPhaseRangeRate := int(GetbitsAsInt64(invalidPhaseRangeRateBytes, 0, 14))

	var testData = []struct {
		Description      string
		MessageType      int
		ID               uint
		WholeMillis      uint
		FractionalMillis uint
		ExtendedInfo     uint
		PhaseRangeRate   int
		Want             MSMSatelliteCell // expected result
	}{
		{"MSM4, all valid", 1074, 1, rangeWhole, rangeFractional, 0, 0,
			MSMSatelliteCell{MessageType: 1074, SatelliteID: 1,
				RangeWholeMillis: 1, RangeFractionalMillis: 2}},
		{"MSM4 with invalid range", 1084, 2, invalidRange, rangeFractional, 0, 0,
			MSMSatelliteCell{MessageType: 1084, SatelliteID: 2,
				RangeWholeMillis: invalidRange, RangeFractionalMillis: rangeFractional}},
		{"MSM7 all valid", 1077, 3, rangeWhole, rangeFractional, extendedInfo, phaseRangeRate,
			MSMSatelliteCell{MessageType: 1077, SatelliteID: 3,
				RangeWholeMillis: rangeWhole, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: phaseRangeRate}},
		{"MSM7 with invalid range", 1087, 4, invalidRange, rangeFractional, extendedInfo, phaseRangeRate,
			MSMSatelliteCell{MessageType: 1087, SatelliteID: 4,
				RangeWholeMillis: invalidRange, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: phaseRangeRate}},
		{"MSM7 with invalid range and phase range rate", 1097, 5, invalidRange, rangeFractional, extendedInfo, invalidPhaseRangeRate,
			MSMSatelliteCell{MessageType: 1097, SatelliteID: 5,
				RangeWholeMillis: invalidRange, RangeFractionalMillis: rangeFractional,
				ExtendedInfo: extendedInfo, PhaseRangeRate: invalidPhaseRangeRate}},
	}
	for _, td := range testData {
		got := createSatelliteCell(
			td.MessageType, td.ID, td.WholeMillis, td.FractionalMillis, td.ExtendedInfo, td.PhaseRangeRate)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Description, td.Want, got)
		}
	}
}

// TestGetSatelliteCellsMSM4 checks that getSatelliteCells correctly interprets a
// bit stream from an MSM4 message containing two satellite cells.
func TestGetSatelliteCellsMSM4(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	satellites := []uint{satelliteID1, satelliteID2}
	headerMSM4 := MSMHeader{MessageType: 1074, Satellites: satellites}

	// The bit stream contains two satellite cells - two 8-bit whole millis
	// followed by two 10-bit fractional millis set like so:
	// 1000 0001| 0000 0001| 1000 0000  01|00 0000  0010|0000- 36 long with 4 bits padding.
	bitStream := []byte{0x81, 1, 0x80, 0x40, 0x20}
	const wantBitPosition = 36 // 2 X 8 + 2 X 10

	handler := New(time.Now(), nil)

	want0 := MSMSatelliteCell{MessageType: 1074, SatelliteID: satelliteID1,
		RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201}
	want1 := MSMSatelliteCell{MessageType: 1074, SatelliteID: satelliteID2,
		RangeWholeMillis: 1, RangeFractionalMillis: 2}

	got, gotBitPosition, satError := handler.getSatelliteCells(bitStream, &headerMSM4, 0)
	if satError != nil {
		t.Error(satError)
		return
	}

	if gotBitPosition != wantBitPosition {
		t.Errorf("got final position %d expected %d", gotBitPosition, wantBitPosition)
	}

	if len(got) != 2 {
		t.Errorf("got %d cells, expected 2", len(got))
	}

	if got[0] != want0 {
		t.Errorf("got %v expected %v", got[0], want0)
	}
	if got[1] != want1 {
		t.Errorf("got %v expected %v", got[1], want1)
	}
}

// TestGetSatelliteCellsMSM7 checks that getSatelliteCells correctly interprets a
// bit stream from an MSM7 message containing two satellite cells.
func TestGetSatelliteCellsMSM7(t *testing.T) {
	const satelliteID1 = 42
	const satelliteID2 = 43
	satellites := []uint{satelliteID1, satelliteID2}
	header := MSMHeader{MessageType: 1077, Satellites: satellites}

	// The bit stream contains two satellite cells - two 8-bit whole millis
	// followed by two 4-bit extended info fields, two 10-bit fractional millis
	// and two 14-bit signed phase range rate values set like so:
	// 1000 0001|  0000 0001|  1001|0110|  1000 0000  01|00 0000
	// 0001|1111   1111 1111   11|01 0000  0000 0001
	bitstream := []byte{0x81, 0x01, 0x96, 0x80, 0x40,
		0x1f, 0xff, 0xd0, 0x01}
	const wantBitPosition = 72 // 2 X 8 + 2 X 4 + 2 X 10 + 2 X 14

	handler := New(time.Now(), nil)

	want0 := MSMSatelliteCell{MessageType: 1077, SatelliteID: satelliteID1,
		RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201,
		ExtendedInfo: 9, PhaseRangeRate: -1}
	want1 := MSMSatelliteCell{MessageType: 1077, SatelliteID: satelliteID2,
		RangeWholeMillis: 1, RangeFractionalMillis: 1,
		ExtendedInfo: 6, PhaseRangeRate: 4097}

	got, gotBitPosition, satError := handler.getSatelliteCells(bitstream, &header, 0)

	if satError != nil {
		t.Error(satError)
		return
	}

	if gotBitPosition != wantBitPosition {
		t.Errorf("got final position %d expected %d", gotBitPosition, wantBitPosition)
	}

	if len(got) != 2 {
		t.Errorf("got %d cells, expected 2", len(got))
	}

	if got[0] != want0 {
		t.Errorf("got %v expected %v", got[0], want0)
	}
	if got[1] != want1 {
		t.Errorf("got %v expected %v", got[1], want1)
	}
}

// TestCreateMSMSignalCell checks that createMSMSignalCell correctly
// creates an MSM4 and MSM7 signal cell, including when some values
// are invalid.
func TestCreateMSMSignalCell(t *testing.T) {

	const rangeDelta = 5
	const phaseRangeDelta = 6
	const lockTimeIndicator = 7
	const halfCycleAmbiguity = true
	const cnr = 8
	const phaseRangeRateDelta = 9

	invalidRangeDeltaMSM4Bytes := []byte{0x80, 0} // 15 bits plus filler: 1000 0000 0000 000 filler 0
	invalidRangeDeltaMSM4 := int(GetbitsAsInt64(invalidRangeDeltaMSM4Bytes, 0, 15))
	invalidRangeDeltaMSM7Bytes := []byte{0x80, 0, 0} // 20 bits: 1000 0000 0000 0000 0000
	invalidRangeDeltaMSM7 := int(GetbitsAsInt64(invalidRangeDeltaMSM7Bytes, 0, 20))
	invalidPhaseRangeDeltaMSM4Bytes := []byte{0x80, 0, 0} // 22 bits plus filler: 1000 0000 0000 0000 0000 00 filler 00
	invalidPhaseRangeDeltaMSM4 := int(GetbitsAsInt64(invalidPhaseRangeDeltaMSM4Bytes, 0, 22))
	invalidPhaseRangeDeltaMSM7Bytes := []byte{0x80, 0, 0} // 24 bits: 1000 0000 0000 0000 0000 0000
	invalidPhaseRangeDeltaMSM7 := int(GetbitsAsInt64(invalidPhaseRangeDeltaMSM7Bytes, 0, 24))
	invalidPhaseRangeRateDeltaBytes := []byte{0x80, 0} // 15 bits plus filler: 1000 0000 0000 000 filler 0
	invalidPhaseRangeRateDelta := int(GetbitsAsInt64(invalidPhaseRangeRateDeltaBytes, 0, 15))

	var testData = []struct {
		Comment             string
		Type4               bool
		ID                  uint
		RangeDelta          int
		PhaseRangeDelta     int
		LockTimeIndicator   uint
		HalfCycleAmbiguity  bool
		CNR                 uint
		PhaseRangeRateDelta int
		Want                MSMSignalCell // expected result
	}{
		{"MSM4, all valid", true, 1, rangeDelta, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, 0,
			MSMSignalCell{SignalID: 1,
				RangeDelta:      rangeDelta,
				PhaseRangeDelta: phaseRangeDelta, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr}},
		{"MSM4, invalid range", true, 2, invalidRangeDeltaMSM4, phaseRangeDelta, lockTimeIndicator, halfCycleAmbiguity, cnr, phaseRangeRateDelta,
			MSMSignalCell{SignalID: 2,
				RangeDelta:      invalidRangeDeltaMSM4,
				PhaseRangeDelta: phaseRangeDelta, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr}},
		{"MSM4, invalid phase range delta", true, 3, rangeDelta, invalidPhaseRangeDeltaMSM4,
			lockTimeIndicator, halfCycleAmbiguity, cnr, 0,
			MSMSignalCell{SignalID: 3,
				RangeDelta:      rangeDelta,
				PhaseRangeDelta: invalidPhaseRangeDeltaMSM4, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
			}},
		{"MSM4, both invalid", true, 4, invalidRangeDeltaMSM4, invalidPhaseRangeDeltaMSM4,
			lockTimeIndicator, halfCycleAmbiguity, cnr, 0,
			MSMSignalCell{SignalID: 4,
				RangeDelta:      invalidRangeDeltaMSM4,
				PhaseRangeDelta: invalidPhaseRangeDeltaMSM4, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr}},

		{"MSM7, all valid", false, 5, rangeDelta, phaseRangeDelta, lockTimeIndicator,
			halfCycleAmbiguity, cnr, phaseRangeRateDelta,
			MSMSignalCell{SignalID: 5,
				RangeDelta:      rangeDelta,
				PhaseRangeDelta: phaseRangeDelta, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
				PhaseRangeRateDelta: phaseRangeRateDelta}},
		{"MSM7, invalid range delta", false, 6, invalidRangeDeltaMSM7, phaseRangeDelta, lockTimeIndicator,
			halfCycleAmbiguity, cnr, phaseRangeRateDelta,
			MSMSignalCell{SignalID: 6,
				RangeDelta:      invalidRangeDeltaMSM7,
				PhaseRangeDelta: phaseRangeDelta, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
				PhaseRangeRateDelta: phaseRangeRateDelta}},
		{"MSM7, invalid phase range delta", false, 5, rangeDelta, invalidPhaseRangeDeltaMSM7,
			lockTimeIndicator, halfCycleAmbiguity,
			cnr, phaseRangeRateDelta,
			MSMSignalCell{SignalID: 5,
				RangeDelta:      rangeDelta,
				PhaseRangeDelta: invalidPhaseRangeDeltaMSM7, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
				PhaseRangeRateDelta: phaseRangeRateDelta}},
		{"MSM7, invalid phase range rate", false, 5, rangeDelta, phaseRangeDelta, lockTimeIndicator,
			halfCycleAmbiguity, cnr,
			invalidPhaseRangeRateDelta,
			MSMSignalCell{SignalID: 5,
				RangeDelta:      rangeDelta,
				PhaseRangeDelta: phaseRangeDelta, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
				PhaseRangeRateDelta: invalidPhaseRangeRateDelta}},
		{"MSM7, all invalid", false, 5, invalidRangeDeltaMSM7, invalidPhaseRangeDeltaMSM7, lockTimeIndicator,
			halfCycleAmbiguity, cnr, invalidPhaseRangeRateDelta,
			MSMSignalCell{SignalID: 5,
				RangeDelta:      invalidRangeDeltaMSM7,
				PhaseRangeDelta: invalidPhaseRangeDeltaMSM7, LockTimeIndicator: lockTimeIndicator,
				HalfCycleAmbiguity: halfCycleAmbiguity, CNR: cnr,
				PhaseRangeRateDelta: invalidPhaseRangeRateDelta}},
	}
	for _, td := range testData {
		got := createSignalCell(
			td.Type4, td.ID, td.RangeDelta, td.PhaseRangeDelta, td.LockTimeIndicator,
			td.HalfCycleAmbiguity, td.CNR, td.PhaseRangeRateDelta)
		if got != td.Want {
			t.Errorf("%s: want %v, got %v", td.Comment, td.Want, got)
		}
	}
}

// TestGetSignalCellsMSM4 checks that getSignalCells correctly interprets a
// bit stream from an MSM4 message containing three signal cells.
func TestGetSignalCellsMSM4(t *testing.T) {
	const signalID1 = 3
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 8

	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	header := MSMHeader{MessageType: 1074, Constellation: "GPS",
		NumSignalCells: 3, Satellites: satellites, Signals: signals,
		CellMask: cellMask}
	satData := []MSMSatelliteCell{
		MSMSatelliteCell{MessageType: 1074, SatelliteID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		MSMSatelliteCell{MessageType: 1074, SatelliteID: satelliteID1,
			RangeWholeMillis: 1, RangeFractionalMillis: 2},
	}

	// The bit stream contains three signal cells - three 15-bit signed range
	// delta (8193, -1, 0), followed by three 22-bit signed phase range delta
	// (-1, 0, 1), three 4-bit unsigned phase lock time indicators (0xf, 0, 1),
	// three single bit half-cycle ambiguity indicators (t, f, t), three 6-bit
	// unsigned GNSS Signal Carrier to Noise Ratio (CNR) (33, 0, 32).
	// (48 bits per signal, so 144 bits in all) set like so:
	// 0100 0000   0000 001|1  1111 1111   1111 11|00   0000 0000   0000 0|111
	// 1111 1111   1111 1111   111|00000   0000 0000    0000 0000   0|000 0000
	// 0000 0000   0000 001|1  111|0000|0   001|1|0|1|100001|0000   00|10 0000|
	bitStream := []byte{
		0x40, 0x03, 0xff, 0xfc, 0x00, 0x07,
		0xff, 0xff, 0xe0, 0x00, 0x00, 0x00,
		0x00, 0x03, 0xe0, 0x36, 0x10, 0x20,
	}

	handler := New(time.Now(), nil)

	want0 := MSMSignalCell{
		Header: &header, Satellite: &satData[0], SignalID: signalID0,
		RangeDelta: 8193, PhaseRangeDelta: -1,
		LockTimeIndicator: 15, HalfCycleAmbiguity: true, CNR: 33,
	}

	want1 := MSMSignalCell{
		Header: &header, Satellite: &satData[0], SignalID: signalID1,
		RangeDelta: -1, PhaseRangeDelta: 0,
		LockTimeIndicator: 0, HalfCycleAmbiguity: false, CNR: 0,
	}

	want2 := MSMSignalCell{
		Header: &header, Satellite: &satData[1], SignalID: signalID0,
		RangeDelta:        0,
		PhaseRangeDelta:   1,
		LockTimeIndicator: 1, HalfCycleAmbiguity: true, CNR: 32,
	}

	got, err := handler.getSignalCells(bitStream, &header, satData, 0)

	if err != nil {
		t.Errorf(err.Error())
	}

	if len(got) != 2 {
		t.Errorf("got %d outer slices, expected 2", len(got))
	}

	if len(got[0]) != 2 {
		t.Errorf("got[0] contains %d cells, expected 2", len(got[0]))
	}
	if len(got[1]) != 1 {
		t.Errorf("got[1] contains %d cells, expected 1", len(got[1]))
	}

	if got[0][0] != want0 {
		t.Errorf("expected [0][0]\n%v got\n%v", want0, got[0][0])
	}

	if got[0][1] != want1 {
		t.Errorf("expected [0][1]\n%v got\n%v", want1, got[0][1])
	}

	if got[1][0] != want2 {
		t.Errorf("expected [1][0]\n%v got\n%v", want2, got[1][0])
	}
}

// TestGetSignalCellsMSM4WithShortBitStream checks that getSignalCells produces
// the correct error message if the bitstream is too short.
func TestGetSignalCellsMSM4WithShortBitStream(t *testing.T) {
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 5
	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	header := MSMHeader{MessageType: 1074, NumSignalCells: 3,
		Satellites: satellites, Signals: signals, CellMask: cellMask}
	satData := []MSMSatelliteCell{
		MSMSatelliteCell{SatelliteID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		MSMSatelliteCell{SatelliteID: satelliteID1,
			RangeWholeMillis: 1, RangeFractionalMillis: 2}}

	// The bit stream is taken from a working example, then one byte is removed.
	// The original contains three signal cells - three 15-bit signed range
	// delta, followed by three 22-bit signed phase range delta, three 4-bit
	// unsigned phase lock time indicators, three single bit half-cycle ambiguity
	// indicators, three 6-bit unsigned GNSS Signal Carrier to Noise Ratio (CNR)
	// values (48 bits per signal, so 144 bits in all) set like so:
	// 0100 0000  0000 0011  1111 1111  1111 1100  0000 0000  0000 0111
	// 1111 1111  1111 1111  1110 0000  0000 0000  0000 0000  0000 0000
	// 0000 0000  0000 0011  1110 0000  0011 0110  0001 0000  0010 0000
	bitStream := []byte{
		0x40, 0x03, 0xff, 0xfc, 0x00, 0x07,
		0xff, 0xff, 0xe0, 0x00, 0x00, 0x00,
		0x00, 0x03, 0xe0, 0x36, 0x10, // 0x20,
	}

	want := "overrun - not enough data for 3 MSM4 signals - 144 136"

	handler := New(time.Now(), nil)

	// Expect an error.
	_, got := handler.getSignalCells(bitStream, &header, satData, 0)

	// Check the error.
	if got == nil {
		t.Error("expected an overrun error")
		return
	}

	if got.Error() != want {
		t.Errorf("expected the error\n\"%s\"\ngot \"%s\"",
			want, got.Error())
		return
	}
}

// TestGetSignalCellsMSM7 checks that getSignalCells correctly interprets a
// bit stream from an MSM7 message containing two signal cells.
func TestGetSignalCellsMSM7(t *testing.T) {
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 5
	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	header := MSMHeader{MessageType: 1077, NumSignalCells: 3,
		Satellites: satellites, Signals: signals, CellMask: cellMask}
	satData := []MSMSatelliteCell{
		MSMSatelliteCell{SatelliteID: satelliteID0},
		MSMSatelliteCell{SatelliteID: satelliteID1},
	}

	// The bit stream contains three signal cells - three 20-bit signed range
	// delta, followed by three 24-bit signed phase range delta, three 10-bit
	// unsigned phase lock time indicators, three single bit half-cycle ambiguity
	// indicators, three 10-bit unsigned GNSS Signal Carrier to Noise Ratio (CNR)
	// values and three 15-bit signed phase range rate delta values. (80 bits per
	// signal, so 240 bits in all) set like so:
	// 0000 0000   0000 0000   0000|1111    1111 1111   1111 1111|  0100 0000     48
	// 0000 0000   0001|1111   1111 1111    1111 1111   1111|0000   0000 0000     96
	// 0000 0000   0000|0000   0000 0000    0000 0000   0101| 1111  1111 11|00   144
	// 0000 0000|  0000 0000   01|011|000   0000 000|1  1111  1111  1|0000001    172
	// 010|11111   1111 1111   11|00 0000   0000 0000   0|000 0000  0000 1101|   240
	bitStream := []byte{
		0x00, 0x00, 0x0f, 0xff, 0xff, 0x40,
		0x00, 0x1f, 0xff, 0xff, 0xf0, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x5f, 0xfc,
		0x00, 0x00, 0x58, 0x01, 0xff, 0x81,
		0x5f, 0xff, 0xc0, 0x00, 0x00, 0x0d,
	}

	handler := New(time.Now(), nil)

	want0 := MSMSignalCell{
		Header: &header, Satellite: &satData[0], SignalID: signalID0,
		RangeDelta: 0, PhaseRangeDelta: -1,
		LockTimeIndicator: 1023, HalfCycleAmbiguity: false, CNR: 0,
		PhaseRangeRateDelta: -1,
	}

	want1 := MSMSignalCell{
		Header: &header, Satellite: &satData[0], SignalID: signalID1,
		RangeDelta: -1, PhaseRangeDelta: 0,
		LockTimeIndicator: 0, HalfCycleAmbiguity: true, CNR: 1023,
		PhaseRangeRateDelta: 0,
	}

	want2 := MSMSignalCell{
		Header: &header, Satellite: &satData[1], SignalID: signalID0,
		RangeDelta: 262145, PhaseRangeDelta: 5,
		LockTimeIndicator: 1, HalfCycleAmbiguity: true, CNR: 10,
		PhaseRangeRateDelta: 13,
	}

	got, err := handler.getSignalCells(bitStream, &header, satData, 0)

	if err != nil {
		t.Errorf(err.Error())
	}

	if len(got) != 2 {
		t.Errorf("got %d outer slices, expected 2", len(got))
	}

	if len(got[0]) != 2 {
		t.Errorf("got[0] contains %d cells, expected 2", len(got[0]))
	}
	if len(got[1]) != 1 {
		t.Errorf("got[1] contains %d cells, expected 1", len(got[1]))
	}

	if got[0][0] != want0 {
		t.Errorf("expected [0][0]\n%v got\n%v", want0, got[0][0])
	}

	if got[0][1] != want1 {
		t.Errorf("expected [0][1]\n%v got\n%v", want1, got[0][1])
	}

	if got[1][0] != want2 {
		t.Errorf("expected [1][0]\n%v got\n%v", want2, got[1][0])
	}
}

// TestGetSignalCellsMSM7WithShortBitStream checks that getSignalCells produces
// the correct error message if the bitstream is too short.
func TestGetSignalCellsMSM7WithShortBitStream(t *testing.T) {
	const signalID1 = 7
	const satelliteID0 = 42
	const satelliteID1 = 43
	satellites := []uint{satelliteID0, satelliteID1}
	const signalID0 = 5
	signals := []uint{signalID0, signalID1}
	// Satellite 42 received signals 5 and 7, satellite 43 received signal 5 only.
	cellMask := [][]bool{{true, true}, {true, false}}
	msm4Header := MSMHeader{MessageType: 1074, NumSignalCells: 3,
		Satellites: satellites, Signals: signals, CellMask: cellMask}
	satData := []MSMSatelliteCell{
		MSMSatelliteCell{SatelliteID: satelliteID0,
			RangeWholeMillis: 0x81, RangeFractionalMillis: 0x201},
		MSMSatelliteCell{SatelliteID: satelliteID1,
			RangeWholeMillis: 1, RangeFractionalMillis: 2}}

	// The bit stream is taken from a working example, then one byte is removed.
	// The original contains three signal cells - three 15-bit signed range
	// delta, followed by three 22-bit signed phase range delta, three 4-bit
	// unsigned phase lock time indicators, three single bit half-cycle ambiguity
	// indicators, three 6-bit unsigned GNSS Signal Carrier to Noise Ratio (CNR)
	// values (48 bits per signal, so 144 bits in all) set like so:
	// 0100 0000  0000 0011  1111 1111  1111 1100  0000 0000  0000 0111
	// 1111 1111  1111 1111  1110 0000  0000 0000  0000 0000  0000 0000
	// 0000 0000  0000 0011  1110 0000  0011 0110  0001 0000  0010 0000
	bitStream := []byte{
		0x40, 0x03, 0xff, 0xfc, 0x00, 0x07,
		0xff, 0xff, 0xe0, 0x00, 0x00, 0x00,
		0x00, 0x03, 0xe0, 0x36, 0x10, // 0x20,
	}

	want := "overrun - not enough data for 3 MSM4 signals - 144 136"

	handler := New(time.Now(), nil)

	// Expect an error.
	_, got := handler.getSignalCells(bitStream, &msm4Header, satData, 0)

	// Check the error.
	if got == nil {
		t.Error("expected an overrun error")
		return
	}

	if got.Error() != want {
		t.Errorf("expected the error\n\"%s\"\ngot \"%s\"",
			want, got.Error())
		return
	}
}

// TestGetMSMType checks that a bitstream containing the start of an
// MSM Header (containing a message type) is correctly interpreted.
func TestGetMSMType(t *testing.T) {
	// getMSMHeaderWithType is a helper function for GetMSMHeader.  The task
	// of figuring out the type values is messy, and needs careful testing.
	// Testing GetMSMHeader involves hand-crafting long bit streams.  If it
	// handled the work that getMSMHeaderType does, we would need to
	// hand-craft lots of them.

	const maxMessageType = 4095 // the message type is a 12-bit unsigned quantity.
	const errorForMaxMessageType = "message type 4095 is not an MSM4 or an MSM7"
	const wantFinalBitPosition = 40 // position after consuming 5 bytes.

	var testData = []struct {
		MessageType       int
		WantError         string // "" means the error is nil
		WantPosition      int
		WantMSM4          bool
		WantConstellation string
	}{
		{1074, "", 40, true, "GPS"},
		{1084, "", 40, true, "GLONASS"},
		{1094, "", 40, true, "Galileo"},
		{1104, "", 40, true, "SBAS"},
		{1114, "", 40, true, "QZSS"},
		{1124, "", 40, true, "BeiDou"},
		{1134, "", 40, true, "NavIC/IRNSS"},
		{1077, "", 40, true, "GPS"},
		{1087, "", 40, true, "GLONASS"},
		{1097, "", 40, true, "Galileo"},
		{1107, "", 40, false, "SBAS"},
		{1117, "", 40, false, "QZSS"},
		{1127, "", 40, false, "BeiDou"},
		{1137, "", 40, false, "NavIC/IRNSS"},

		// These message numbers are not for MSM messages
		{0, "message type 0 is not an MSM4 or an MSM7", 0, false, ""},
		{1, "message type 1 is not an MSM4 or an MSM7", 0, false, ""},
		{1073, "message type 1073 is not an MSM4 or an MSM7", 0, false, ""},
		{1075, "message type 1075 is not an MSM4 or an MSM7", 0, false, ""},
		{1076, "message type 1076 is not an MSM4 or an MSM7", 0, false, ""},
		{1138, "message type 1138 is not an MSM4 or an MSM7", 0, false, ""},
		{1023, "message type 1023 is not an MSM4 or an MSM7", 0, false, ""},
		{maxMessageType, errorForMaxMessageType, 40, false, ""},
	}
	for _, td := range testData {
		// Create a 5-byte (40-bit) bitstream containing a 3-byte leader,
		// the 12-bit message type and four trailing zero bits.
		// example 1074: |0100 0001 1110|0000

		// Shift the type to give 16 bits with 4 trailing bits.
		tp := td.MessageType << 4

		bitStream := []byte{
			0xff, 0xff, 0xff, byte(tp >> 8), byte(tp & 0xff),
		}

		msmHeader, position, err := getMSMType(bitStream, 0)

		if td.WantError != "" {
			// The call is expected to fail and return an error.
			if err == nil {
				t.Errorf("%d: want error %s",
					td.MessageType, td.WantError)
			} else {
				if td.WantError != err.Error() {
					t.Errorf("%d: want error %s, got %s",
						td.MessageType, td.WantError, err.Error())
				}
			}

			// The checks below only make sense for valid calls.
			continue
		}

		// The call is expected to work.
		if err != nil {
			t.Errorf("%d: want no error, got %s",
				td.MessageType, err.Error())
			continue
		}

		if msmHeader.MessageType != td.MessageType {
			t.Errorf("want type %d, got %d",
				td.MessageType, msmHeader.MessageType)
			continue
		}

		if msmHeader.Constellation != td.WantConstellation {
			t.Errorf("%d: want constellation %s, got %s",
				td.MessageType, td.WantConstellation, msmHeader.Constellation)
			continue
		}

		if td.WantPosition != wantFinalBitPosition {
			t.Errorf("%d: want %d bits to be consumed, got %d",
				td.MessageType, wantFinalBitPosition, position)
			continue
		}
	}
}

// TestGetMSMTypeWithShortBitstream checks that getMSMHeaderWithType
// returns the correct error message when given a bit stream which is too short.
func TestGetMSMTypeWithShortBitstream(t *testing.T) {
	tp := 1074
	mType := tp << 4
	// A bitstream containing a 3-byte leader and an incomplete message type
	bitStream := []byte{0xff, 0xff, 0xff, byte(mType >> 8)}
	wantError :=
		"cannot extract the header from a bitstream 32 bits long, expected at least 36 bits"

	_, _, gotError := getMSMType(bitStream, 0)

	if gotError == nil {
		t.Errorf("type %d not MSM, expected an error", mType)
		return
	}

	if gotError.Error() != wantError {
		em := fmt.Sprintf("want \"%s\", got \"%s\"", wantError, gotError)
		t.Error(em)
		return
	}
}

// TestGetMSMHeader checks that a bitstream containing an MSMHeader is
// correctly interpreted.
func TestGetMSMHeader(t *testing.T) {

	// The bitstream read by the function under test contains a 3-byte message header,
	// followed by the MSM Header.  The message header has already been processed and
	// is ignored at this point.
	//
	// The MSMHeader contains:
	//    a 12-bit unsigned message type (tested in detail separately)
	//    a 12-bit unsigned station ID
	//    a 30-bit unsigned timestamp
	//    a boolean multiple message flag
	//    a 3-bit unsigned sequence number
	//    a 7-bit unsigned session transmission time value
	//    a 2-bit unsigned clock steering indicator
	//    a 2-bit unsigned external clock indicator
	//    a boolean GNSS Divergence Free Smoothing Indicator
	//    a 3-bit GNSS Smoothing Interval
	//    a 64-bit satellite mask (one bit set per satellite observed)
	//    a 32-bit signal mask (one bit set per signal type observed)
	//    (169 bits up to this point)
	//    a cell mask (nSatellites X nSignals) bits long
	//
	// The function returns the broken out header and the bit position
	// of the start of the next part of the message. (In the real world
	// that comes immediately after this cell mask, so the position is
	// just the length of this bit stream, ignoring the final three
	// padding bits.)

	// 1111 1111   1111 1111   1111 1111  |0100 0011   0101|0000
	// 0000 0001|  0000 0000   0000 0000   0000 0000   0000 10|1|0
	// 11|00 0010  0|10|01|1|111|0000000   0000 0000   0000 0000
	// 0000 0000   0000 0000   0000 0000   0000 0000   0000 1110
	// 1|0000000   0000 0000   0000 0000   0001 0101   0|1111100
	// 0000 1|000

	bitStream := []byte{
		0xff, 0xff, 0xff, 0x43, 0x50,
		0x01, 0x00, 0x00, 0x00, 0x0a,
		0xc2, 0x4f, 0x80, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x0e,
		0x80, 0x00, 0x00, 0x15, 0x7c,
		0x08,
	}

	// The length of the fixed-size fields of the cell mask.
	const lenHeaderWithoutCellMask = 169
	// The length of the cell mask (4 satellites, 3 signals).
	const lenCellMask = 12
	// The expected satellite IDs
	satellites := []uint{60, 61, 62, 64}
	// The signal types we expect to observe
	signals := []uint{27, 29, 31}
	// The expected broken-out cell mask values.
	cellMask := [][]bool{
		{true, true, true}, {true, true, false},
		{false, false, false}, {false, false, true},
	}
	// Expect to be at this position of the bit stream after reading the header.
	const wantPos = lenHeaderWithoutCellMask + lenCellMask + leaderLengthInBits
	// Expect this error message if the bit stream is short by two bytes
	// (so not all the fixed-length items are there).
	wantMinLengthError :=
		fmt.Sprintf("bitstream is too short for an MSM header - got %d bits, expected at least %d",
			(len(bitStream)-2)*8, lenHeaderWithoutCellMask)
	// Expect this error if the bit stream is short by one byte
	// (so it includes part of the cell mask but is incomplete).
	wantLengthError :=
		fmt.Sprintf("bitstream is too short for an MSM header with %d cell mask bits - got %d bits, expected at least %d",
			lenCellMask, (len(bitStream)-1)*8, wantPos)

	wantHeader := MSMHeader{
		MessageType:                          1077,
		Constellation:                        "GPS",
		StationID:                            1,
		EpochTime:                            2,
		MultipleMessage:                      true,
		SequenceNumber:                       3,
		SessionTransmissionTime:              4,
		ClockSteeringIndicator:               2,
		ExternalClockIndicator:               1,
		GNSSDivergenceFreeSmoothingIndicator: true,
		GNSSSmoothingInterval:                7,
		SatelliteMaskBits:                    0x001d,
		SignalMaskBits:                       0x2a,
		CellMaskBits:                         0xf81,
		NumSignalCells:                       6,
	}

	handler := New(time.Now(), nil)

	// This bitstream is below the minimum length for an MSM header.
	_, _, minLengthError := handler.GetMSMHeader(bitStream[:len(bitStream)-2])

	if minLengthError == nil {
		t.Errorf("expected an error")
	} else {

		if minLengthError.Error() != wantMinLengthError {
			t.Errorf("expected error \"%s\", got \"%s\"",
				wantMinLengthError, minLengthError.Error())

		}
	}

	// This bitstream is above the minimum length but still short.
	_, _, lengthError := handler.GetMSMHeader(bitStream[:len(bitStream)-1])

	if lengthError == nil {
		t.Errorf("expected an error")
	} else {
		if lengthError.Error() != wantLengthError {
			t.Errorf("expected error \"%s\", got \"%s\"",
				wantLengthError, lengthError.Error())

		}
	}

	header, gotPos, err := handler.GetMSMHeader(bitStream)

	if err != nil {
		t.Error(err)
		return
	}

	if gotPos != uint(wantPos) {
		t.Errorf("got position %d, want %d", gotPos, wantPos)
	}

	if header.Constellation != wantHeader.Constellation {
		t.Errorf("got type %s, want %s",
			header.Constellation, wantHeader.Constellation)
	}

	if header.StationID != wantHeader.StationID {
		t.Errorf("got type %d, want %d",
			header.StationID, wantHeader.StationID)
	}

	if header.EpochTime != wantHeader.EpochTime {
		t.Errorf("got epoch time %d, want %d",
			header.EpochTime, wantHeader.EpochTime)
	}

	// UTCTime time.Time

	if header.MultipleMessage != wantHeader.MultipleMessage {
		t.Errorf("got multiple message %v, want %v",
			header.MultipleMessage, wantHeader.MultipleMessage)
	}

	if header.SequenceNumber != wantHeader.SequenceNumber {
		t.Errorf("got sequence number %d, want %d",
			header.SequenceNumber, wantHeader.SequenceNumber)
	}

	if header.SessionTransmissionTime != wantHeader.SessionTransmissionTime {
		t.Errorf("got trans time %d, want %d",
			header.SessionTransmissionTime, wantHeader.SessionTransmissionTime)
	}

	if header.ClockSteeringIndicator != wantHeader.ClockSteeringIndicator {
		t.Errorf("got clock steering ind %d, want %d",
			header.ClockSteeringIndicator, wantHeader.ClockSteeringIndicator)
	}

	if header.GNSSDivergenceFreeSmoothingIndicator !=
		wantHeader.GNSSDivergenceFreeSmoothingIndicator {

		t.Errorf("got smoothing ind %v, want %v",
			header.GNSSDivergenceFreeSmoothingIndicator,
			wantHeader.GNSSDivergenceFreeSmoothingIndicator)
	}

	if header.GNSSSmoothingInterval != wantHeader.GNSSSmoothingInterval {
		t.Errorf("got smoothing interval %d, want %d",
			header.GNSSSmoothingInterval, wantHeader.GNSSSmoothingInterval)
	}

	if header.ExternalClockIndicator != wantHeader.ExternalClockIndicator {
		t.Errorf("got external CLI %d, want %d",
			header.ExternalClockIndicator, wantHeader.ExternalClockIndicator)
	}

	if header.SatelliteMaskBits != wantHeader.SatelliteMaskBits {
		t.Errorf("got satellite mask 0x%x, want 0x%x",
			header.SatelliteMaskBits, wantHeader.SatelliteMaskBits)
	}

	if header.SignalMaskBits != wantHeader.SignalMaskBits {
		t.Errorf("got signal mask 0x%x, want 0x%x",
			header.SignalMaskBits, wantHeader.SignalMaskBits)
	}

	if header.CellMaskBits != wantHeader.CellMaskBits {
		t.Errorf("got cell mask 0x%x, want 0x%x",
			header.CellMaskBits, wantHeader.CellMaskBits)
	}

	if header.NumSignalCells != wantHeader.NumSignalCells {
		t.Errorf("got number of signal cells %d, want %d",
			header.NumSignalCells, wantHeader.NumSignalCells)
	}

	if len(satellites) == len(header.Satellites) {
		for i := range satellites {
			if satellites[i] != header.Satellites[i] {
				t.Errorf("satellite %d want %d got %d",
					i, satellites[i], header.Satellites[i])
			}
		}
	} else {
		t.Errorf("want %d satellites, got %d", len(satellites), len(header.Satellites))
		return
	}

	if len(signals) == len(header.Signals) {
		for i := range signals {
			if signals[i] != header.Signals[i] {
				t.Errorf("signal %d want %d got %d",
					i, signals[i], header.Signals[i])
			}
		}
	} else {
		t.Errorf("want %d signals, got %d", len(signals), len(header.Signals))
		return
	}

	// Check the cell mask.
	if len(cellMask) == len(header.CellMask) {
		for i := range cellMask {
			if len(cellMask[i]) == len(header.CellMask[i]) {
				for j := range cellMask[i] {
					if cellMask[i][j] != (header.CellMask[i][j]) {
						t.Errorf("cellMask[%d][%d]: want %v, got %v",
							i, j, cellMask[i][j], header.CellMask[i][j])
					}
				}
			} else {
				t.Errorf("cellMask[%d] want %d items, got %d",
					i, len(cellMask), len(header.CellMask))
			}
		}
	} else {
		t.Errorf("cellMask: want %d items, got %d",
			len(cellMask), len(header.CellMask))
	}
}

// TestGetMSMScaledRange checks getMSMScaledRange.
func TestGetMSMScaledRange(t *testing.T) {

	// getMSMScaledRange combine the 8-bit unsigned whole range, the 10-bit
	// unsigned fractional range and the 20-bit signed delta to get a
	// scaled integer with 18 bits whole and 19 bits fractional.  The whole
	// and the delta may both have values indicating that they are invalid
	// BUT the function under test assumes that the incoming values have
	// already been checked and it assumes that it will only receive valid
	// values.

	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// The maximum delta value is (2 to the power 19) -1
	// (111 1111 1111 1111 1111 - 0x7ffff)
	const maxDelta = 0x7ffff

	// The maximum result is a 37-bit unsigned with all components at their
	// maximum value: 1 1111 1101 1111 1111 1111 1111 1111 1111 1111
	const maxScaledRange = 0x1fdfffffff

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 or invalid delta:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0000
	const maxNoDelta = 0x1fdff80000

	// wholeFracSmall is the result of combining a whole and fractional part,
	// both 1, with a zero or invalid delta:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0000
	const wholeFracSmall = 0x20080000

	// allOne is the result of combining three values, all 1:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0001
	const allOne = 0x20080001

	// deltaMinusOne is the result of combining 1, 1 and -1:
	// 0 0000 0010 0000 0000 0111 1111 1111 1111 1111
	const deltaMinusOne = 0x2007ffff

	var testData = []struct {
		WholeMillis      uint
		FractionalMillis uint
		Delta            int
		Want             uint64 // Expected result.
	}{
		{0, 0, 0, 0},
		{1, 1, 0, wholeFracSmall},
		{1, 1, 1, allOne},
		{1, 1, -1, deltaMinusOne},
		// Maximum approx range, zero delta.
		{maxWhole, maxFractional, 0, maxNoDelta},
		// All maximum values.
		{maxWhole, maxFractional, maxDelta, maxScaledRange},
	}

	for _, td := range testData {
		got := GetScaledRange(td.WholeMillis,
			td.FractionalMillis, td.Delta)
		if got != td.Want {
			if td.Delta < 0 {
				t.Errorf("(0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.WholeMillis, td.FractionalMillis, td.Delta, td.Want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.WholeMillis, td.FractionalMillis, td.Delta, td.Want, got)
			}
		}
	}
}

// TestGetAggregateRange checks getAggregateRange.
func TestGetAggregateRange(t *testing.T) {
	// getAggregateRange takes a header, satellite cell and signal cell
	// from an MSMMessage object containing the data from an MSM4 or an MSM7
	// message.  It combines those values and returns the range as a floating
	// point value in metres per second.  The data values can be marked as
	// invalid.

	const invalidWhole = 0xff   // 1111 1111
	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// The invalid range value fr an MSM7 is 20 bits: 1000 0000 0000 0000 0000 filler 0000.
	invalidDeltaBytes := []byte{0x80, 0, 0, 0}
	invalidDelta20 := int(GetbitsAsInt64(invalidDeltaBytes, 0, 20))
	// the invalid value for an MSM4 is just the first 15 bits of the MSM7 pattern.
	invalidDelta15 := int(GetbitsAsInt64(invalidDeltaBytes, 0, 15))

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 delta:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0000
	const maxNoDelta = 0x1fdff80000

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of 1:
	// 1 1111 1101 1111 1111 1000 0000 0000 0000 0001
	const maxDeltaOne = 0x1fdff80001

	/// maxDelta16 is the result of combining the maximum whole and fractional
	// parts with a 15-bit delta of 1, normalised to 20 bits:
	// 1 1111 1101 1111 1111 1000 0000 0000 0001 0000
	const maxDelta16 = 0x1fdff80010

	// allOne is the result of combining three values, all 1:
	// 0 0000 0010 0000 0000 1000 0000 0000 0000 0001
	const allOne = 0x20080001

	msm4Header := MSMHeader{MessageType: 1074}
	msm7Header := MSMHeader{MessageType: 1077}

	satCellWithInvalidWhole :=
		MSMSatelliteCell{RangeWholeMillis: invalidWhole, RangeFractionalMillis: 1}
	satCellWithMaxValues :=
		MSMSatelliteCell{RangeWholeMillis: maxWhole, RangeFractionalMillis: maxFractional}
	satCellBothOne := MSMSatelliteCell{RangeWholeMillis: 1, RangeFractionalMillis: 1}

	sigCell4WithInvalidRangeDelta := MSMSignalCell{
		Header:     &msm4Header,
		RangeDelta: invalidDelta15,
	}

	// Each signal cell needs a pointer to a satellite cell, but the cell
	// is different on each iteration of the test so it's supplied later.
	sigCell7WithInvalidRangeDelta := MSMSignalCell{
		Header:     &msm7Header,
		RangeDelta: invalidDelta20,
	}

	sigCell4WithRangeDeltaOne := MSMSignalCell{
		Header:     &msm4Header,
		RangeDelta: 1,
	}

	sigCell7WithRangeDeltaOne := MSMSignalCell{
		Header:     &msm7Header,
		RangeDelta: 1,
	}

	var testData = []struct {
		Header    MSMHeader
		Satellite MSMSatelliteCell
		Signal    MSMSignalCell
		Want      uint64 // Expected result.
	}{

		// If the whole milliseconds value is invalid, the result is always zero.
		{msm7Header, satCellWithInvalidWhole, sigCell7WithRangeDeltaOne, 0},
		{msm4Header, satCellWithInvalidWhole, sigCell4WithRangeDeltaOne, 0},
		// If the delta is invalid, the result is the approximate range.
		{msm4Header, satCellWithMaxValues, sigCell4WithInvalidRangeDelta, maxNoDelta},
		{msm7Header, satCellWithMaxValues, sigCell7WithInvalidRangeDelta, maxNoDelta},
		// For an MSM4 message, the delta (if valid) is 16 times the value given.
		{msm4Header, satCellWithMaxValues, sigCell4WithInvalidRangeDelta, maxNoDelta},
		{msm7Header, satCellBothOne, sigCell7WithRangeDeltaOne, allOne},
		{msm4Header, satCellWithMaxValues, sigCell4WithRangeDeltaOne, maxDelta16},
		// For an MSM7 message, the delta is as given.
		{msm7Header, satCellWithMaxValues, sigCell7WithRangeDeltaOne, maxDeltaOne},
		{msm7Header, satCellBothOne, sigCell7WithRangeDeltaOne, allOne},
	}

	for _, td := range testData {

		// Set the correct satellite in the signal.
		td.Signal.Satellite = &td.Satellite

		got := td.Signal.GetAggregateRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("(0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.Satellite.RangeWholeMillis,
					td.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("(0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.Satellite.RangeWholeMillis,
					td.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			}
		}
	}
}

// TestRangeInMetresMSM7 checks that the correct range is calculated for an MSM7.
func TestRangeInMetresMSM7(t *testing.T) {

	const maxWhole = 0xfe                                         // 1111 1110
	const maxFractional = 0x3ff                                   // 11 1111 1111
	var bigRangeMillisWhole uint = 0x80                           // 1000 0000
	var bigRangeMillisFractional uint = 0x200                     // 10 bits 1000 ...
	const wantBig float64 = (128.5 + P2_11) * oneLightMillisecond // 38523477.236036
	bigDelta := int(0x40000)                                      // 20 bits 0100 ...
	const twoToPower29 = 0x20000000                               // 10 0000 0000 0000 0000 0000 0000 0000
	const twoToPower19 = 0x40000                                  // 1000 0000 0000 0000 0000

	header := MSMHeader{MessageType: 1097}

	satCellBothOne := MSMSatelliteCell{RangeWholeMillis: 1, RangeFractionalMillis: 1}

	satCellWithMillisOne := MSMSatelliteCell{RangeWholeMillis: 1, RangeFractionalMillis: 0}

	satCellWithBigValues :=
		MSMSatelliteCell{RangeWholeMillis: bigRangeMillisWhole, RangeFractionalMillis: bigRangeMillisFractional}

	satCellWithMaxValues :=
		MSMSatelliteCell{RangeWholeMillis: maxWhole, RangeFractionalMillis: maxFractional}

	sigCellWithRangeZero := MSMSignalCell{
		Header:     &header,
		RangeDelta: 0,
	}

	sigCellWithRangeDeltaOne := MSMSignalCell{
		Header:     &header,
		RangeDelta: 1,
	}

	sigCellWithSmallNegativeDelta := MSMSignalCell{
		Header:     &header,
		RangeDelta: -1,
	}

	sigCellWithBigDelta := MSMSignalCell{
		Header:     &header,
		RangeDelta: bigDelta,
	}

	// These values are taken from real data - an MSM7
	// converted to RINEX format to give the range.
	// {81, 435, -26835, 24410527.355},

	satCellWithRealValues :=
		MSMSatelliteCell{RangeWholeMillis: 81, RangeFractionalMillis: 435}

	sigCellWithRealDelta := MSMSignalCell{
		Header:     &header,
		RangeDelta: -26835,
	}

	const wantResultFromReal = 24410527.355

	// The range delta is minus (half of a range fractional value of 1).
	sigCellWithLargeNegativeDelta := MSMSignalCell{
		Header:     &header,
		RangeDelta: -1 * twoToPower19,
	}

	var testData = []struct {
		Description string
		Header      MSMHeader
		Satellite   MSMSatelliteCell
		Signal      MSMSignalCell
		Want        float64 // Expected result.
	}{

		// For an MSM4 message, the delta (if valid) is 16 times the value given.
		{"1,0,0", header, satCellWithMillisOne, sigCellWithRangeZero, oneLightMillisecond},
		{"1,1,1", header, satCellBothOne, sigCellWithRangeDeltaOne,
			(float64(1) + (float64(1) / 1024) + (float64(1) / twoToPower29)) * oneLightMillisecond},
		{"1,1,small neg", header, satCellBothOne, sigCellWithSmallNegativeDelta,
			(float64(1) + (float64(1)/1024 - (float64(1) / twoToPower29))) * oneLightMillisecond},
		{"1,1,large neg", header, satCellBothOne, sigCellWithLargeNegativeDelta,
			(float64(1) + (float64(1) / 2048)) * oneLightMillisecond},
		{"big data", header, satCellWithBigValues, sigCellWithBigDelta, wantBig},
		{"max,max,1", header, satCellWithMaxValues, sigCellWithRangeDeltaOne,
			(float64(maxWhole) + (float64(maxFractional) / 1024) + (float64(1) / twoToPower29)) * oneLightMillisecond},
		{"max,max,0", header, satCellWithMaxValues, sigCellWithRangeZero,
			(float64(maxWhole) + (float64(maxFractional) / 1024)) * oneLightMillisecond},
		{"real data", header, satCellWithRealValues, sigCellWithRealDelta, wantResultFromReal},
	}

	for _, td := range testData {

		td.Signal.Satellite = &td.Satellite

		got, rangeError := td.Signal.RangeInMetres()
		if rangeError != nil {
			t.Error(rangeError)
		}
		if !equalWithin(3, td.Want, got) {
			t.Errorf("%s: want %f got %f", td.Description, td.Want, got)
		}

	}
}

// TestGetAggregatePhaseRange checks getAggregateRange.
func TestGetAggregatePhaseRange(t *testing.T) {
	// getAggregateRange takes a header, satellite cell and signal cell
	// from an MSMMessage object containing the data from an MSM4 or an MSM7
	// message.  It combines those values and returns the range as a floating
	// point value in cycles.  The data values can be marked as invalid
	// invalid.

	const invalidWhole = 0xff   // 1111 1111
	const maxWhole = 0xfe       // 1111 1110
	const maxFractional = 0x3ff // 11 1111 1111

	// 24 bits: 1000 0000 0000 0000 0000 0000
	invalidDeltaBytes := []byte{0x80, 0, 0}
	invalidDelta24 := int(GetbitsAsInt64(invalidDeltaBytes, 0, 24))
	// The invalid value for an MSM4 is just the first 22 bits of MSM7 pattern.
	invalidDelta22 := int(GetbitsAsInt64(invalidDeltaBytes, 0, 22))

	// The incoming delta value is 24 bits signed and the delta and the fractional
	// part share 3 bits, producing a 39-bit result.
	//
	//     ------ Range -------
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range rate delta.

	// maxNoDelta is the result of combining the maximum whole and fractional
	// parts with a 0 24-bit delta:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0000
	const maxNoDelta = 0x7f7fe00000

	// maxDeltaOne is the result of combining the maximum whole and fractional
	// parts with a delta of 1:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0001
	const maxDeltaOne = 0x7f7fe00001

	/// maxDelta4 is the result of combining the maximum whole and fractional
	// parts with a 22-bit delta of 1, normalised to 24 bits:
	//     111 1111 0|111 1111 111|0
	//                         000 0000 0000 0000 0000 0100
	const maxDelta4 = 0x7f7fe00004

	// allOne is the result of combining three values, all 1:
	//     000 0000 1|000 0000 001|0
	//                         000 0000 0000 0000 0000 0001
	const allOne = 0x80200001

	msm4Header := MSMHeader{MessageType: 1074}
	msm7Header := MSMHeader{MessageType: 1077}

	satCellWithInvalidWhole :=
		MSMSatelliteCell{RangeWholeMillis: invalidWhole, RangeFractionalMillis: 1}
	satCellWithMaxValues :=
		MSMSatelliteCell{RangeWholeMillis: maxWhole, RangeFractionalMillis: maxFractional}
	satCellBothOne := MSMSatelliteCell{RangeWholeMillis: 1, RangeFractionalMillis: 1}

	sigCell4WithInvalidPhaseRangeDelta := MSMSignalCell{PhaseRangeDelta: invalidDelta22}

	sigCell7WithInvalidPhaseRangeDelta := MSMSignalCell{PhaseRangeDelta: invalidDelta24}

	sigCell4WithPhaseRangeDeltaOne := MSMSignalCell{PhaseRangeDelta: 1}

	sigCell7WithPhaseRangeDeltaOne := MSMSignalCell{PhaseRangeDelta: 1}

	var testData = []struct {
		ID        int
		Header    MSMHeader
		Satellite MSMSatelliteCell
		Signal    MSMSignalCell
		Want      uint64 // Expected result.
	}{
		{3, msm4Header, satCellWithMaxValues, sigCell4WithInvalidPhaseRangeDelta, maxNoDelta},
		// If the whole milliseconds value is invalid, the result is always zero.
		{1, msm7Header, satCellWithInvalidWhole, sigCell7WithPhaseRangeDeltaOne, 0},
		{2, msm4Header, satCellWithInvalidWhole, sigCell4WithPhaseRangeDeltaOne, 0},
		// If the delta is invalid, the result is the approximate range.
		{3, msm4Header, satCellWithMaxValues, sigCell4WithInvalidPhaseRangeDelta, maxNoDelta},
		{4, msm7Header, satCellWithMaxValues, sigCell7WithInvalidPhaseRangeDelta, maxNoDelta},
		// For an MSM4 message, the delta (if valid) is 4 times the value given.
		{5, msm4Header, satCellWithMaxValues, sigCell4WithInvalidPhaseRangeDelta, maxNoDelta},
		{6, msm7Header, satCellBothOne, sigCell7WithPhaseRangeDeltaOne, allOne},
		{7, msm4Header, satCellWithMaxValues, sigCell4WithPhaseRangeDeltaOne, maxDelta4},
		// For an MSM7 message, the delta is as given.
		{8, msm7Header, satCellWithMaxValues, sigCell7WithPhaseRangeDeltaOne, maxDeltaOne},
		{9, msm7Header, satCellBothOne, sigCell7WithPhaseRangeDeltaOne, allOne},
	}

	for _, td := range testData {

		// Set the correct values in the satellite and signal.
		td.Satellite.MessageType = td.Header.MessageType
		td.Signal.Header = &td.Header
		td.Signal.Satellite = &td.Satellite

		got := td.Signal.GetAggregatePhaseRange()
		if got != td.Want {
			if td.Signal.RangeDelta < 0 {
				t.Errorf("(%d 0x%x,0x%x,%d) want 0x%x, got 0x%x",
					td.ID,
					td.Satellite.RangeWholeMillis,
					td.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			} else {
				t.Errorf("%d (0x%x,0x%x,0x%x) want 0x%x, got 0x%x",
					td.ID,
					td.Satellite.RangeWholeMillis,
					td.Satellite.RangeFractionalMillis,
					td.Signal.RangeDelta, td.Want, got)
			}
		}
	}
}

func TestGetPhaseRange(t *testing.T) {
	const rangeMillisWhole uint = 0x80       // 1000 0000
	const rangeMillisFractional uint = 0x200 // 10 0000 0000
	const phaseRangeDelta int = 1

	// The 39-bit aggregate values works like so:
	//     whole     fractional
	//     876 5432 1098 7654 3210 9876 5432 1098 7654 3210
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd <- phase range rate delta.
	//   0|100 0000 0100 0000 000|
	//                        0000 0000 0000 0000 0000 0001
	const wantAggregate = 0x4040000001

	const twoToPower31 = 0x80000000 // 1000 0000 0000 0000 0000 0000 0000 0000
	const twoToPowerMinus31 = 1 / float64(twoToPower31)
	const rangeMilliseconds = 128.5 + float64(twoToPowerMinus31)
	const wantWavelength = cLight / freq2

	rangeLM := rangeMilliseconds * oneLightMillisecond
	var signalID uint = 16

	wantPhaseRange := rangeLM / wantWavelength

	header := MSMHeader{
		MessageType:   1077,
		Constellation: "GPS",
	}

	satelliteCell := MSMSatelliteCell{
		RangeWholeMillis:      rangeMillisWhole,
		RangeFractionalMillis: rangeMillisFractional}

	signalCell := MSMSignalCell{
		Header:          &header,
		Satellite:       &satelliteCell,
		SignalID:        signalID,
		PhaseRangeDelta: phaseRangeDelta,
	}

	agg := signalCell.GetAggregatePhaseRange()

	if agg != wantAggregate {
		t.Errorf("want aggregate 0x%x got 0x%x", wantAggregate, agg)
		return
	}

	wavelength, wavelengthError := signalCell.GetWavelength()
	if wavelengthError != nil {
		t.Error(wavelengthError)
		return
	}

	if wavelength != wantWavelength {
		if wantWavelength != wavelength {
			t.Errorf("want wavelength %f got %f", wavelength, wantWavelength)
			return
		}
	}

	r := getPhaseRangeMilliseconds(agg)
	if !equalWithin(6, r, rangeMilliseconds) {
		t.Errorf("want range %f got %f", rangeMilliseconds, r)
		return
	}

	rlm := getPhaseRangeLightMilliseconds(r)
	if !equalWithin(3, rangeLM, rlm) {
		t.Errorf("want range %f got %f", rangeLM, rlm)
		return
	}

	if wantWavelength != wavelength {
		t.Errorf("want wavelength %f got %f", wantWavelength, wavelength)
		return
	}

	gotPhaseRange, phaseRangError := signalCell.PhaseRange()

	if phaseRangError != nil {
		t.Error(phaseRangError)
		return
	}

	if !equalWithin(3, wantPhaseRange, gotPhaseRange) {
		t.Errorf("expected %f got %f", wantPhaseRange, gotPhaseRange)
		return
	}

	// Try the biggest positive delta: 0100 0000 0000 0000 0000 0000
	const biggestDelta int = 0x400000

	const biggestDeltaRangeMilliseconds = 128.5 + float64(biggestDelta)*float64(twoToPowerMinus31)

	const biggestDeltaRangeLM = biggestDeltaRangeMilliseconds * oneLightMillisecond

	wantBiggestPhaseRange := biggestDeltaRangeLM / wantWavelength

	signalCell.PhaseRangeDelta = biggestDelta

	gotPhaseRange2, phaseRangError2 := signalCell.PhaseRange()

	if phaseRangError2 != nil {
		t.Error(phaseRangError2)
		return
	}

	if !equalWithin(3, wantBiggestPhaseRange, gotPhaseRange2) {
		t.Errorf("expected %f got %f", wantBiggestPhaseRange, gotPhaseRange2)
		return
	}
}

// TestGetPhaseRangeRealValues tests getPhaseRange with real data.
func TestGetPhaseRangeRealValues(t *testing.T) {

	// These data were captured from equipment and converted to RINEX format.
	const signalID = 2
	// wantPhaseRange is taken from the resulting RINEX file.
	const wantPhaseRange = 128278179.264

	header := MSMHeader{
		MessageType:   1077,
		Constellation: "GPS",
	}

	satellite := MSMSatelliteCell{
		RangeWholeMillis:      81,
		RangeFractionalMillis: 435}

	signalCell := MSMSignalCell{
		Header:          &header,
		Satellite:       &satellite,
		SignalID:        signalID,
		PhaseRangeDelta: -117960}

	gotCycles, err := signalCell.PhaseRange()

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if !equalWithin(3, wantPhaseRange, gotCycles) {
		t.Errorf("expected %f got %f", wantPhaseRange, gotCycles)
		return
	}
}

// TestMSM7DopplerWithRealData checks that getMSM7Doppler works using real data.
func TestMSM7DopplerWithRealData(t *testing.T) {
	// The input data were collected from a UBlox device.
	// The want value is from a RINEX file created from those data.

	const want = float64(709.992)

	header := MSMHeader{MessageType: 1077, Constellation: "GPS"}

	satellite := MSMSatelliteCell{
		PhaseRangeRate: -135}

	sigCell := MSMSignalCell{
		Header:              &header,
		Satellite:           &satellite,
		SignalID:            2,
		PhaseRangeRateDelta: -1070}

	got, err := sigCell.GetMSM7Doppler()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	if !equalWithin(3, want, got) {
		t.Errorf("expected %f got %f", want, got)
		return
	}
}
