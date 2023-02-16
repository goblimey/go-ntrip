// The satellite package contains code to handle the satellite cells of a
// type 7 Multiple Signal Message.  The satellite cells follow the header in
// the message.  Each cell contains the data about one satellite: the
// approximate (rough) range, the extended satellite information and the
// rough phase range rate. The rough range is expressed in light milliseconds,
// ie the  approximate transit time of the signals from the satellite to the
// GPS device in whole milliseconds and fractional milliseconds.  The real
// transit time of each signal can be slightly different due to factors such as
// ionospheric distortion.  Each signal cell contains a small delta which is
// added to the rough value given here to give the transit time of that signal.
// The signal data also contains a phase range rate delta value which is used
// to correct the rough phase rang rate value.
//
package satellite

import (
	"errors"
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// InvalidRange is the invalid value for the whole millis range.
const InvalidRange = 0xff

// invalidPhaseRangeRate is the invalid value for the phase range rate.
// 14 bit two's complement 10 0000 0000 0000
const InvalidPhaseRangeRate = -8192

// Lengths of the fields in the bitStream.
const lenWholeMillis = 8
const lenExtendedInfo = 4
const lenFractionalMillis = 10
const lenPhaseRangeRate = 14

// CellLengthInBits is the total length of the cell
const CellLengthInBits = lenWholeMillis + lenExtendedInfo + lenFractionalMillis + lenPhaseRangeRate

// Cell holds the data from one satellite cell from a type 7 Multiple Signal Message.
// (Message type 1077, 1087 ...).
//
type Cell struct {
	// The field names, types and sizes and invalid values are shown in comments
	// in rtklib rtcm3.c - see the function decode_msm7().

	// SatelliteID is the satellite ID, 1-64.
	SatelliteID uint

	// RangeWholeMillis - uint8 - the number of integer milliseconds in the
	// GNSS Satellite range (ie the transit time of the signals).  0xff
	// indicates an invalid value.  See also the RangeFractionalMillis value
	// here and the delta value in the signal cell.
	RangeWholeMillis uint

	// ExtendedInfo - uint4.  Extended Satellite Information.
	ExtendedInfo uint

	// RangeFractionalMillis - unit10.  The fractional part of the range
	// in milliseconds.
	RangeFractionalMillis uint

	// PhaseRangeRate - int14.  The approximate phase range rate for all signals
	// that come later in this MSM7 message.  Invalid if the top bit is set and all
	// the others are zero (InvalidPhaseRangeRate).  The value is signed, so the
	// invalid value is a negative number.  If the value is valid, the true phase
	// range rate for a signal is derived by merging this (positive or negative)
	// scaled value with the signal's PhaseRangeRateDelta value.
	PhaseRangeRate int
}

// New creates an MSM7 satellite cell from the given values.
func New(id, wholeMillis, fractionalMillis, extendedInfo uint, phaseRangeRate int) *Cell {

	cell := Cell{
		SatelliteID:           id,
		RangeWholeMillis:      wholeMillis,
		RangeFractionalMillis: fractionalMillis,
		ExtendedInfo:          extendedInfo,
		PhaseRangeRate:        phaseRangeRate,
	}

	return &cell
}

func (cell *Cell) String() string {

	var approxRange string
	if cell.RangeWholeMillis == InvalidRange {
		approxRange = "invalid"
	} else {
		// The range values are valid.
		rangeMillis := utils.GetApproxRange(cell.RangeWholeMillis, cell.RangeFractionalMillis)
		approxRange = fmt.Sprintf("%.3f", rangeMillis)
	}

	var phaseRangeRate string
	if cell.PhaseRangeRate == InvalidPhaseRangeRate {
		phaseRangeRate = "invalid"
	} else {
		// The phase range rate value is valid.
		phaseRangeRate = fmt.Sprintf("%d", cell.PhaseRangeRate)
	}
	return fmt.Sprintf("%2d {%s, %d, %s}",
		cell.SatelliteID, approxRange, cell.ExtendedInfo, phaseRangeRate)
}

// GetSatelliteCells extracts the satellite cell data from an MSM7 message.
// It returns a slice of cell data.  If the bitstream is not long enough to
// contain the message, it returns an error.
//
func GetSatelliteCells(bitStream []byte, startOfSatelliteData uint, Satellites []uint) ([]Cell, error) {
	// The bitStream contains the variable length header, the satellite cells and
	// then the signal cells.  startOfSatelliteData gives the bit position of the
	// start of the satellite cells.  Satellites gives the number of satellites
	// from which signals were observed and their IDs.  If signals were observed
	// from satellites 2, 3 and 15, Satellites will contain those three IDs
	// and the bitStream will contain a list of three rough range values, followed
	// by a list of three extended info values, followed by three fractional range
	// values, and so on.
	//
	// It's more convenient to represent these data as a list of cells, one cell per
	// satellite, so we gather all the values and then create the cells.

	// bitsLeft is the number of bits in the bitStream left to consume.
	bitsLeft := len(bitStream)*8 - int(startOfSatelliteData)
	// minBits is the minimum number of bits needed to hold the satellite cells.
	// (There must be at least this many bits left.)
	minBits := len(Satellites) * CellLengthInBits

	if ((len(bitStream) * 8) - int(startOfSatelliteData)) < minBits {
		message :=
			fmt.Sprintf("overrun - not enough data for %d MSM7 satellite cells - need %d bits, got %d",
				len(Satellites), minBits, bitsLeft)
		return nil, errors.New(message)
	}

	// Set the bit position to the start of the satellite data in the message.
	pos := startOfSatelliteData

	// Gather the values:

	// Get the rough range values (whole milliseconds).
	wholeMillis := make([]uint, 0)
	for range Satellites {
		millis := uint(utils.GetBitsAsUint64(bitStream, pos, lenWholeMillis))
		pos += lenWholeMillis
		wholeMillis = append(wholeMillis, millis)
	}

	// Aet the extended info values.
	extendedInfo := make([]uint, 0)
	for range Satellites {
		info := utils.GetBitsAsUint64(bitStream, pos, lenExtendedInfo)
		pos += lenExtendedInfo
		extendedInfo = append(extendedInfo, uint(info))
	}

	// Get the fractional millis values (fractions of a millisecond).
	fractionalMillis := make([]uint, 0)
	for range Satellites {
		fraction := utils.GetBitsAsUint64(bitStream, pos, lenFractionalMillis)
		pos += lenFractionalMillis
		fractionalMillis = append(fractionalMillis, uint(fraction))
	}

	// Get the phase range rate field.
	phaseRangeRate := make([]int, 0)
	for range Satellites {
		rate := utils.GetBitsAsInt64(bitStream, pos, lenPhaseRangeRate)
		pos += lenPhaseRangeRate
		phaseRangeRate = append(phaseRangeRate, int(rate))
	}

	// Create a slice of satellite cells using the data that we just gathered.
	satData := make([]Cell, 0)
	for i := range Satellites {
		satCell := New(Satellites[i], wholeMillis[i],
			fractionalMillis[i], extendedInfo[i], phaseRangeRate[i])
		satData = append(satData, *satCell)
	}

	return satData, nil
}
