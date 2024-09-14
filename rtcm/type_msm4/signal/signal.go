// package signal contains code to handle the data from a signal cell from a
// Multiple Signal Message type 4 (message type 1074, 1084 etc.).  Various
// values are defined from values in the signal cell and its associated satellite
// cell.  For convenience the ones from the satellite cell are copied here.
package signal

import (
	"errors"
	"fmt"
	"log/slog"

	msmHeader "github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm4/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Cell holds the data from an MSM4 message for one signal
// from one satellite, plus values copied from the satellite.
type Cell struct {
	// Field names, sizes, invalid values etc are derived from rtklib rtcm3.c
	// (decode_msm4 function) plus other clues from the igs BNC application.

	// SatelliteID is the ID of the satellite from which this signal was received: 1-64.
	// SatelliteID uint

	// ID is the ID of the signal that was observed: 1-32.
	ID uint

	// Wavelength is the wavelength of the signal
	Wavelength float64

	// RangeDelta - int15.  A scaled value representing a small signed delta to be added to
	// the range values from the satellite to get the range as the transit time of the
	// signal.  The value is in units of (two to the power of -24) milliseconds.  To get
	// the range in metres, multiply the result by one light millisecond (the distance
	// light travels in a millisecond).  The function RangeInMetres combines the 3 values
	// and returns th result in metres.
	RangeDelta int

	// PhaseRangeDelta - int22.  Invalid if the top bit is set and the others are all zero
	// (-2097152).  The true phase range for the signal is derived by scaling this and adding
	// it to the approximate value in the satellite cell.  If this value is invalid, use just
	// the approximate value.
	PhaseRangeDelta int

	// LockTimeIndicator - uint4.
	LockTimeIndicator uint

	// HalfCycleAmbiguity flag - 1 bit.
	HalfCycleAmbiguity bool

	// CarrierToNoiseRatio - uint6.
	CarrierToNoiseRatio uint

	// The satellite that sent the signal.
	Satellite *satellite.Cell

	// LogLevel controls the data output by String.
	LogLevel slog.Level
}

// New creates an MSM Signal Cell.
func New(
	signalID uint,
	satelliteCell *satellite.Cell,
	rangeDelta,
	phaseRangeDelta int,
	lockTimeIndicator uint,
	halfCycleAmbiguity bool,
	cnr uint,
	wavelength float64,
	logLevel slog.Level,
) *Cell {

	cell := Cell{
		ID:                  signalID,
		Wavelength:          wavelength,
		RangeDelta:          rangeDelta,
		PhaseRangeDelta:     phaseRangeDelta,
		LockTimeIndicator:   lockTimeIndicator,
		HalfCycleAmbiguity:  halfCycleAmbiguity,
		CarrierToNoiseRatio: cnr,
		Satellite:           satelliteCell,
		LogLevel:            logLevel,
	}

	return &cell
}

// String returns a readable version of a signal cell.
func (cell *Cell) String() string {

	var satID string
	if cell.Satellite == nil {
		satID = "<nil>"
	} else {
		satID = fmt.Sprintf("%2d", cell.Satellite.ID)
	}
	var rangeM string
	if cell.Satellite == nil || cell.Satellite.RangeWholeMillis == utils.InvalidRange {
		rangeM = "invalid"
	} else {
		// Convert the delta to float and divide by two to the power 24 to restore
		// the scale.  This gives the delta in milliseconds.
		rangeDeltaInMillis := float64(cell.RangeDelta) / float64(utils.TwoToThePower24)

		rangeDeltaInMetres := rangeDeltaInMillis * utils.OneLightMillisecond

		rangeM = fmt.Sprintf("(%d, %.3f, %.3f)",
			cell.RangeDelta, rangeDeltaInMetres, cell.RangeInMetres())
	}
	var phaseRange string
	if cell.Satellite == nil || cell.Satellite.RangeWholeMillis == utils.InvalidRange {
		phaseRange = "invalid"
	} else {
		phaseRange = fmt.Sprintf("(%d, %.3f)",
			cell.PhaseRangeDelta, cell.PhaseRange())
	}
	return fmt.Sprintf("%s %2d {%s, %s, %d, %v, %d, %.3f}",
		satID, cell.ID, rangeM, phaseRange,
		cell.LockTimeIndicator, cell.HalfCycleAmbiguity,
		cell.CarrierToNoiseRatio, cell.Wavelength)
}

// GetSignalCells gets the data from the signal cells of an MSM4 message.
func GetSignalCells(
	bitStream []byte,
	startOfSignalCells uint,
	header *msmHeader.Header,
	satCells []satellite.Cell,
	logLevel slog.Level,
) ([][]Cell, error) {
	// The third part of the message bit stream is the signal data.  Each satellite can
	// send many signals, each on a different frequency.  For example, if we observe one
	// signal from satellite 2, two from satellite 3 and 2 from satellite 15, there will
	// be five sets of signal data.  Irritatingly they are not laid out in a convenient
	// way.  First, we get the pseudo range delta values for each of the five signals,
	// followed by all of the phase range delta values, and so on. The trailing bytes of
	// the message may be all zeros.
	//
	// It's more convenient to present these as a slice of slices of fields, one outer
	// slice for each satellite and one inner slice for each observed signal.
	//
	// If the multiple message flag is set in the header, the message is one of a set
	// with the same timestamp and station ID.  Each message in the set contains some
	// of the signals.  If the multiple message flag is not set then we expect the
	// message to contain all the signals.

	// Define the lengths of the fields
	const lenRangeDelta uint = 15
	const lenPhaseRangeDelta uint = 22
	const lenLockTimeIndicator uint = 4
	const lenHalfCycleAmbiguity uint = 1
	const lenCNR uint = 6

	const bitsPerCell = lenRangeDelta + lenPhaseRangeDelta +
		lenLockTimeIndicator + lenHalfCycleAmbiguity + lenCNR

	// The frame contain the 24-bit leader, the embedded message and the 24-bit CRC.
	// startOfSignalCells is the number of bits of the FRAME consumed so far.
	bitsLeftInFrame := uint(len(bitStream)*8 - int(startOfSignalCells))
	bitsLeftInMessage := bitsLeftInFrame - utils.CRCLengthBits

	// Pos is the position within the bitstream.
	pos := startOfSignalCells

	// Find the number of signal cells, ignoring any padding.

	numSignalCells := utils.GetNumberOfSignalCells(bitStream, pos, bitsPerCell)

	if header.MultipleMessage {
		// The message doesn't contain all the signal cells but there should be
		// at least one.
		if bitsLeftInMessage < bitsPerCell {
			message := fmt.Sprintf("overrun - want at least one %d-bit signal cell when multiple message flag is set, got only %d bits left",
				bitsPerCell, bitsLeftInMessage)
			return nil, errors.New(message)
		}
	} else {
		// This message should contain all the signal cells.  Check that
		// there are the expected number.
		if numSignalCells < header.NumSignalCells {
			message := fmt.Sprintf("overrun - want %d MSM4 signals, got %d",
				header.NumSignalCells, numSignalCells)
			return nil, errors.New(message)
		}
	}

	// Capture the signal fields into a set of slices, one per field.

	// Get the range deltas.
	rangeDelta := make([]int, 0)
	for i := 0; i < numSignalCells; i++ {
		rd := int(utils.GetBitsAsInt64(bitStream, pos, lenRangeDelta))
		pos += lenRangeDelta
		rangeDelta = append(rangeDelta, rd)
	}

	// Get the phase range deltas.
	phaseRangeDelta := make([]int, 0)
	for i := 0; i < numSignalCells; i++ {
		prd := int(utils.GetBitsAsInt64(bitStream, pos, lenPhaseRangeDelta))
		pos += lenPhaseRangeDelta
		phaseRangeDelta = append(phaseRangeDelta, prd)
	}

	// Get the lock time indicators.
	lockTimeIndicator := make([]uint, 0)
	for i := 0; i < numSignalCells; i++ {
		lti := uint(utils.GetBitsAsUint64(bitStream, pos, lenLockTimeIndicator))
		pos += lenLockTimeIndicator
		lockTimeIndicator = append(lockTimeIndicator, lti)
	}

	// Get the half-cycle ambiguity indicator bits.
	halfCycleAmbiguity := make([]bool, 0)
	for i := 0; i < numSignalCells; i++ {
		hca := (utils.GetBitsAsUint64(bitStream, pos, lenHalfCycleAmbiguity) == 1)
		pos += lenHalfCycleAmbiguity
		halfCycleAmbiguity = append(halfCycleAmbiguity, hca)
	}

	// Get the Carrier to Noise Ratio values.
	cnr := make([]uint, 0)
	for i := 0; i < numSignalCells; i++ {
		c := uint(utils.GetBitsAsUint64(bitStream, pos, lenCNR))
		pos += lenCNR
		cnr = append(cnr, c)
	}

	// Create and return a slice of slices of signal cells.
	// For example if the satellite mask in the header contains {3, 5, 8} and the
	// cell mask contains {{1,5},{1},{5}} then we received signals 1 and 5 from
	// satellite 3, signal 1 from satellite 5 and signal 5 from satellite 8.
	// We would return this slice of slices:
	//     0: a slice of two signal cells with satellite ID 3, signal IDs 1 and 5
	//     1: a slice of one signal cell with satellite ID 5, signal ID 1
	//     2: a slice of one signal cell with satellite ID 8, signal ID 5
	//
	// Figuring this out is a bit messy, because the order information
	// is distributed over the satellite, signal and cell masks.

	// c is the index into the slices of signal fields captured above.
	c := 0

	// Create a slice of slices of cells ...
	signalCells := make([][]Cell, 0)
	// ... and populate it.
	for i := range header.Cells {
		cellSlice := make([]Cell, 0)
		signalCells = append(signalCells, cellSlice)
		for j := range header.Cells[i] {
			// Beware!  We are consuming cells in a loop controlled by cranking through the
			// 1 bits in the cell mask.  If the multiple message flag is set, only some of
			// those cells are in this message.
			if c < numSignalCells {

				if header.Cells[i][j] {

					signalID := header.Signals[j]

					wavelength := utils.GetSignalWavelength(header.Constellation, signalID)

					cell := New(signalID, &satCells[i], rangeDelta[c],
						phaseRangeDelta[c], lockTimeIndicator[c],
						halfCycleAmbiguity[c], cnr[c], wavelength,
						logLevel,
					)

					signalCells[i] = append(signalCells[i], *cell)

					// Prepare to process the next set of signal fields.
					c++
				}
			}
		}
	}

	return signalCells, nil
}

// GetAggregateRange takes the range values from an MSM4 signal cell (including some
// copied from the MSM4satellite cell) and returns the range as a 37-bit scaled unsigned
// integer with 8 bits whole part and 29 bits fractional part.  This is the transit time
// of the signal in milliseconds.  Use RangeInMetres to convert it to a distance in
// metres.  The whole millis value and/or the delta value can indicate that the
// measurement was invalid.  If the approximate range value in the satellite cell is
// invalid, the result is 0.  If the delta in the signal cell is invalid, the result is
// the approximate range.
//
// This is a helper function for RangeInMetres, exposed for unit testing.
func (cell *Cell) GetAggregateRange() uint64 {

	if cell.Satellite == nil {
		return 0
	}

	if cell.Satellite.RangeWholeMillis == utils.InvalidRange {
		return 0
	}

	if cell.RangeDelta == utils.InvalidRangeDelta {
		// The range is valid but the delta is not.
		return utils.GetScaledRange(cell.Satellite.RangeWholeMillis,
			cell.Satellite.RangeFractionalMillis, 0)
	}

	// The delta value is valid.

	// The range delta value in an MSM4 signal is 15 bits signed.  In an MSM7 signal
	// it's 20 bits signed, the bottom 5 bits being extra precision.  The calculation
	// assumes the MSM7 form, so for an MSM4, normalise the value to 20 bits before
	// using it.  The value may be negative, so multiply rather than shifting bits.
	delta := cell.RangeDelta * 32

	return utils.GetScaledRange(cell.Satellite.RangeWholeMillis,
		cell.Satellite.RangeFractionalMillis, delta)
}

// GetAggregatePhaseRange takes a header, satellite cell and signal cell, extracts
// the phase range values, aggregates them and returns them as a 41-bit scaled unsigned
// unsigned integer, 8 bits whole part and 33 bits fractional part.  Use
// getPhaseRangeCycles to convert this to the phase range in cycles.
func (cell *Cell) GetAggregatePhaseRange() uint64 {

	// This is similar to getAggregateRange  but for the phase range.  The phase
	// range value in the signal cell is merged with the range values from the
	// satellite cell.

	if cell.Satellite == nil {
		return 0
	}

	if cell.Satellite.RangeWholeMillis == utils.InvalidRange {
		return 0
	}

	var delta int

	if cell.PhaseRangeDelta == utils.InvalidPhaseRangeDelta {
		// The range is valid but the delta is not.  Use just the range.
		delta = 0
	} else {
		// The range and the delta are valid.  Use both.
		// The value is 22 bits signed.  In an MSM7 signal cell it's 24 bits signed,
		// the lowest two bits being extra precision.  The calculation assumes the MSM7
		// form, so normalise the MSM4 value to 24 bits before using it.  It may be
		// negative, so multiply it rather than shifting bits.
		//
		delta = cell.PhaseRangeDelta * 4
	}

	scaledPhaseRange := utils.GetScaledPhaseRange(
		cell.Satellite.RangeWholeMillis,
		cell.Satellite.RangeFractionalMillis,
		delta,
	)

	return scaledPhaseRange
}

// RangeInMillis gives the distance from the satellite to the GPS device derived from
// the satellite and signal cell as the transit time in milliseconds.
func (cell *Cell) RangeInMillis() float64 {
	// Get the range as a 37-bit scaled integer, 8 bits whole, 29 bits fractional
	// representing the transit time in milliseconds.
	scaledRange := cell.GetAggregateRange()

	// Convert to float and divide by two to the power 29 to restore the scale.
	// scaleFactor is two to the power of 29:
	// 10 0000 0000 0000 0000 0000 0000 0000
	const scaleFactor = 0x20000000
	// Restore the scale to give the range in milliseconds.
	rangeInMillis := float64(scaledRange) / float64(scaleFactor)

	return rangeInMillis
}

// RangeInMetres gives the distance from the satellite to the GPS device derived from
// the satellite and signal cell, in metres.
func (cell *Cell) RangeInMetres() float64 {

	rangeInMillis := cell.RangeInMillis()

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	rangeInMetres := rangeInMillis * utils.OneLightMillisecond

	return rangeInMetres
}

// PhaseRange combines the range and the phase range from an MSM4
// message and returns the result in cycles. It returns zero if the input
// measurements are invalid and an error if the signal is not in use.
func (cell *Cell) PhaseRange() float64 {

	// In the RTKLIB, the decode_msm4 function uses the range from the
	// satellite and the phase range from the signal cell to derive the
	// carrier phase:
	//
	// /* carrier-phase (cycle) */
	// if (r[i]!=0.0&&cp[j]>-1E12&&wl>0.0) {
	//    rtcm->obs.data[index].L[ind[k]]=(r[i]+cp[j])/wl;
	// }

	// This is similar to RangeInMetres.  The phase range delta is aggregated
	// with the range values from the satellite cell and converted to cycles.

	aggregatePhaseRange := cell.GetAggregatePhaseRange()

	// Restore the scale of the aggregate value.
	phaseRangeMilliSeconds := utils.GetPhaseRangeMilliseconds(aggregatePhaseRange)

	// Convert to light milliseconds
	phaseRangeLMS := utils.GetPhaseRangeLightMilliseconds(phaseRangeMilliSeconds)

	// and divide by the wavelength to get cycles.
	phaseRangeCycles := phaseRangeLMS / cell.Wavelength

	return phaseRangeCycles
}
