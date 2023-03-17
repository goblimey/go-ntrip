// The satellite package contains code to handle MSM4 satellite data.  The
// satellite cells follow the header in a type 4 Multiple Signal Message (MSM).
// Each cell contains the data about one satellite: the approximate (rough)
// range in terms of light milliseconds, ie the approximate transit time of
// the signals from the satellite to the GPS device in whole milliseconds and
// fractional milliseconds.  The real transit time of each signal can be
// slightly different due to factors such as ionospheric distortion, and each
// signal cell contains a small delta which is added to the approximate values
// given here to give the transit time of that signal.
package satellite

import (
	"errors"
	"fmt"

	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// lenWholeMillis is the length of the Whole Millis part of the cell.
const lenWholeMillis = 8

// lenFractionalMillis is the length of the FractionalMillis part of the cell.
const lenFractionalMillis = 10

// CellLengthInBits is the number of bits in each cell.
const CellLengthInBits = lenWholeMillis + lenFractionalMillis

// Cell holds the data for one satellite from an MSM message,
// type MSM4 (message type 1074, 1084 ...).
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

	// RangeFractionalMillis - unit10.  The fractional part of the range
	// in milliseconds.
	RangeFractionalMillis uint
}

// New creates an MSM4 satellite cell from the given values.
func New(id, wholeMillis, fractionalMillis uint) *Cell {

	cell := Cell{
		SatelliteID:           id,
		RangeWholeMillis:      wholeMillis,
		RangeFractionalMillis: fractionalMillis,
	}

	return &cell
}

func (cell *Cell) String() string {

	var approxRange string
	if cell.RangeWholeMillis == utils.InvalidRange {
		approxRange = "invalid"
	} else {
		// The range values are valid.
		rangeMillis := utils.GetApproxRangeMetres(cell.RangeWholeMillis, cell.RangeFractionalMillis)
		approxRange = fmt.Sprintf("%.3f", rangeMillis)
	}

	return fmt.Sprintf("%2d {%s}", cell.SatelliteID, approxRange)
}

// GetSatelliteCells extracts the satellite cell data from an MSM4 message.
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
	// by three fractional range values.
	//
	// It's more convenient to represent these data as a list of cells, one cell per
	// satellite, so we gather all the values and then create the cells.

	bitsLeft := len(bitStream)*8 - int(startOfSatelliteData)
	minBits := len(Satellites) * (lenWholeMillis + lenFractionalMillis)

	if (((len(bitStream)) * 8) - int(startOfSatelliteData)) <= minBits {
		message := fmt.Sprintf("overrun - not enough data for %d MSM4 satellite cells - need %d bits, got %d",
			len(Satellites), minBits, bitsLeft)
		return nil, errors.New(message)
	}

	// Set the bit position to the start of the satellite data in the message.
	pos := startOfSatelliteData

	// Get the rough range values (whole milliseconds).
	wholeMillis := make([]uint, 0)
	for range Satellites {
		millis := uint(utils.GetBitsAsUint64(bitStream, pos, lenWholeMillis))
		pos += lenWholeMillis
		wholeMillis = append(wholeMillis, millis)
	}

	// Get the fractional millis values (fractions of a millisecond).
	fractionalMillis := make([]uint, 0)
	for range Satellites {
		fraction := utils.GetBitsAsUint64(bitStream, pos, lenFractionalMillis)
		pos += lenFractionalMillis
		fractionalMillis = append(fractionalMillis, uint(fraction))
	}

	// Create a slice of satellite cells initialised from those data.
	satData := make([]Cell, 0)
	for i := range Satellites {
		satCell := New(Satellites[i], wholeMillis[i], fractionalMillis[i])
		satData = append(satData, *satCell)
	}

	return satData, nil
}
