// package signal contains code to handle the data from a signal cell from a
// Multiple Signal Message type 7 (message type 1077, 1087 etc.).  Various
// values are defined from values in the signal cell and its associated satellite
// cell.  For convenience the ones from the satellite cell are copied here.
package signal

import (
	"errors"
	"fmt"

	msmHeader "github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// invalidRange is the invalid value for the whole millis range in an MSM
// satellite cell.
const invalidRange = 0xff

// InvalidRangeDelta is the invalid value for the range delta in an MSM7
// signal cell. 20 bit two's complement 1000 0000 0000 0000 0000
const InvalidRangeDelta = -524288

// InvalidPhaseRangeDelta is the invalid value for the phase range delta
// in an MSM7 signal cell.  24 bit two's complement: 1000 0000 0000 0000 0000 0000
const InvalidPhaseRangeDelta = -8388608

// InvalidPhaseRangeRate is the invalid value for the phase range rate in an
// MSM7 Satellite cell.  14 bit two's complement 10 0000 0000 0000
const InvalidPhaseRangeRate = -8192

// invalidPhaseRangeRateDelta is the invalid value for the delta in an MSM7
// signal cell. 15 bit two's complement 100 0000 0000 0000
const InvalidPhaseRangeRateDelta = -16384

// Cell holds the data from a Multiple Signal Message type 7 for one signal
// from one satellite, plus values copied from the satellite cell.
// RangeInMetres gives the distance from the satellite to the GPS device derived from
// the values in the satellite and signal cell, converted to metres.
type Cell struct {
	// Field names, sizes, invalid values etc are derived from rtklib rtcm3.c
	// (decode_msm7 function) plus other clues from the igs BNC application.

	// SatelliteID is the satellite ID, 1-64.
	SatelliteID uint

	// SignalID is the ID of the signal, 1-32.
	SignalID uint

	// Wavelength is the wavelength of the signal
	Wavelength float64

	// RangeWholeMillisFromSatelliteCell is the RangeWholeMillis from the satellite cell -
	// Whole milliseconds of range 0-255. It's used to calculate the range and the phase
	// range.  0xff indicates an invalid value, meaning that all range values should be
	// ignored.
	RangeWholeMillisFromSatelliteCell uint

	// RangeFractionalMillisFromSatelliteCell is the RangeFractionalMillis value for the
	// satellite cell.  The units are 1/1024 milliseconds.  The values is used to
	// calculate the range and phase range.
	RangeFractionalMillisFromSatelliteCell uint

	// PhaseRangeRateFromSatelliteCell is the PhaseRangeRate value copied from the satellite
	// cell.  An int14 value in units of one millisecond.  Invalid if the top bit is set and
	// the other bits are zero (-8192).  Combining this with the signal's PhaseRangeRateDelta
	// value gives the phase range rate in light milliseconds.  According to Blewitt's paper,
	// the phase range rate is the velocity at which the satellite is moving towards (if
	// positive) or away from (if negative) the receiving GPS device.
	PhaseRangeRateFromSatelliteCell int

	// RangeDelta - int20.  A scaled value representing a small signed delta to be added to
	// the range values from the satellite to get the range as the transit time of the signal.
	// To get the range in metres, multiply the result by one light millisecond, the distance
	// light travels in a millisecond.
	RangeDelta int

	// PhaseRangeDelta - int24.  Invalid if the top bit is set and the others are all zero
	// (-2097152).  The true phase range for the signal is derived by scaling this and adding
	// it to the approximate value in the satellite cell.  If this value is invalid, use just
	// the approximate value.
	PhaseRangeDelta int

	// LockTimeIndicator - uint10.
	LockTimeIndicator uint

	// HalfCycleAmbiguity flag - 1 bit.
	HalfCycleAmbiguity bool

	// CarrierToNoiseRatio - uint10.
	CarrierToNoiseRatio uint

	// PhaseRangeRateDelta - int15 - invalid if the top bit is set and the others are all
	// zero (-16384).  The value is in ten thousands of a millisecond. The true value of the
	// signal's phase range rate is derived by scaling this (positive or negative) delta and
	// adding it to the approximate value from the satellite cell.
	PhaseRangeRateDelta int
}

// New creates an MSM7 Signal Cell.
func New(signalID uint, satelliteCell *satellite.Cell, rangeDelta, phaseRangeDelta int, lockTimeIndicator uint, halfCycleAmbiguity bool, cnr uint, phaseRangeRateDelta int, wavelength float64) *Cell {

	var satelliteID uint
	var rangeWhole uint
	var rangeFractional uint
	var phaseRangeRate int

	if satelliteCell != nil {
		satelliteID = satelliteCell.SatelliteID
		rangeWhole = satelliteCell.RangeWholeMillis
		rangeFractional = satelliteCell.RangeFractionalMillis
		phaseRangeRate = satelliteCell.PhaseRangeRate
	}

	cell := Cell{
		SatelliteID:                            satelliteID,
		SignalID:                               signalID,
		Wavelength:                             wavelength,
		RangeWholeMillisFromSatelliteCell:      rangeWhole,
		RangeFractionalMillisFromSatelliteCell: rangeFractional,
		PhaseRangeRateFromSatelliteCell:        phaseRangeRate,
		RangeDelta:                             rangeDelta,
		PhaseRangeDelta:                        phaseRangeDelta,
		LockTimeIndicator:                      lockTimeIndicator,
		HalfCycleAmbiguity:                     halfCycleAmbiguity,
		CarrierToNoiseRatio:                    cnr,
		PhaseRangeRateDelta:                    phaseRangeRateDelta,
	}

	return &cell
}

// String returns a readable version of a signal cell.
func (cell *Cell) String() string {
	var rangeM string
	r, rangeError := cell.RangeInMetres()
	if rangeError == nil {
		rangeM = fmt.Sprintf("%f", r)
	} else {
		rangeM = rangeError.Error()
	}

	var phaseRange string
	pr, prError := cell.PhaseRange()
	if prError == nil {
		phaseRange = fmt.Sprintf("%f", pr)
	} else {
		phaseRange = prError.Error()
	}

	var phaseRangeRate string
	prr, prrError :=
		cell.PhaseRangeRate()
	if prrError == nil {
		phaseRangeRate = fmt.Sprintf("%f", prr)
	} else {
		phaseRangeRate = prrError.Error()
	}

	return fmt.Sprintf("%2d %2d {%s %s %d, %v, %d, %s}",
		cell.SatelliteID, cell.SignalID, rangeM, phaseRange,
		cell.LockTimeIndicator, cell.HalfCycleAmbiguity,
		cell.CarrierToNoiseRatio, phaseRangeRate)
}

// GetAggregateRange takes the range values from an MSM7 signal cell (including some
// copied from the MSM7satellite cell) and returns the range as a 37-bit scaled unsigned
// integer with 8 bits whole part and 29 bits fractional part.  This is the transit time
// of the signal in milliseconds.  Use GetRangeInMetres to convert it to a distance in
// metres.  The whole millis value and/or the delta value can indicate that the
// measurement was invalid.  If the approximate range value in the satellite cell is
// invalid, the result is 0.  If the delta in the signal cell is invalid, the result is
// the approximate range.
//
// This is a helper function for RangeInMetres, exposed for unit testing.
func (cell *Cell) GetAggregateRange() uint64 {

	if cell.RangeWholeMillisFromSatelliteCell == invalidRange {
		return 0
	}

	if cell.RangeDelta == InvalidRangeDelta {
		// The range is valid but the delta is not.
		return utils.GetScaledRange(cell.RangeWholeMillisFromSatelliteCell,
			cell.RangeFractionalMillisFromSatelliteCell, 0)
	}

	// The delta value is valid.
	return utils.GetScaledRange(cell.RangeWholeMillisFromSatelliteCell,
		cell.RangeFractionalMillisFromSatelliteCell, cell.RangeDelta)
}

// RangeInMetres gives the distance from the satellite to the GPS device derived from
// the values in the satellite and signal cell, converted to metres.
func (cell *Cell) RangeInMetres() (float64, error) {

	// Get the range as a 37-bit scaled integer, 8 bits whole, 29 bits fractional
	// representing the transit time in milliseconds.
	scaledRange := cell.GetAggregateRange()

	// Convert to float and divide by two to the power 29 to restore the scale.
	// scaleFactor is two to the power of 29:
	// 10 0000 0000 0000 0000 0000 0000 0000
	const scaleFactor = 0x20000000
	rangeInMillis := float64(scaledRange) / float64(scaleFactor)

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	rangeInMetres := rangeInMillis * utils.OneLightMillisecond

	return rangeInMetres, nil
}

// GetPhaseRangeLightMilliseconds gets the phase range of the signal in light milliseconds.
func (cell *Cell) GetPhaseRangeLightMilliseconds(rangeMetres float64) float64 {
	return rangeMetres * utils.OneLightMillisecond
}

// PhaseRange combines the range and the phase range from an MSM7
// message and returns the result in cycles. It returns zero if the input
// measurements are invalid and an error if the signal is not in use.
//
func (cell *Cell) PhaseRange() (float64, error) {

	// In the RTKLIB, the decode_msm7 function uses the range from the
	// satellite and the phase range from the signal cell to derive the
	// carrier phase:
	//
	// /* carrier-phase (cycle) */
	// if (r[i]!=0.0&&cp[j]>-1E12&&wl>0.0) {
	//    rtcm->obs.data[index].L[ind[k]]=(r[i]+cp[j])/wl;
	// }

	// This is similar to RangeInMetres.  The phase range is in cycles and
	// derived from the range values from the satellite cell shifted up
	// 31 bits, plus the 24 bit signed phase range delta.
	//
	//     ------ Range -------
	//     whole     fractional
	//     www wwww wfff ffff fff0 0000 0000 0000 0000 0000
	//     + or -             dddd dddd dddd dddd dddd dddd

	aggregatePhaseRange := cell.GetAggregatePhaseRange()

	// Restore the scale of the aggregate value.
	phaseRangeMilliSeconds := utils.GetPhaseRangeMilliseconds(aggregatePhaseRange)

	// Convert to light milliseconds
	phaseRangeLMS := utils.GetPhaseRangeLightMilliseconds(phaseRangeMilliSeconds)

	// and divide by the wavelength to get cycles.
	phaseRangeCycles := phaseRangeLMS / cell.Wavelength

	return phaseRangeCycles, nil
}

// PhaseRangeRate combines the components of the phase range rate
// in an MSM7 message and returns the result in milliseconds.  If the rate
// value in the satellite cell is invalid, the result is zero.  If the delta
// in the signal cell is invalid, the result is based on the rate value in the
// satellite.
//
func (cell *Cell) PhaseRangeRate() (float64, error) {

	aggregatePhaseRangeRate := cell.GetAggregatePhaseRangeRate()

	// The aggregate is milliseconds scaled up by 10,000.
	phaseRangeRateMillis := float64(aggregatePhaseRangeRate) / 10000

	// Blewitt's paper says that the phase range rate is the rate at which the
	// which the satellite is approaching or (if negative) receding from
	// the GPS device.

	return phaseRangeRateMillis, nil
}

// GetMSM7Doppler gets the doppler value in Hz from the phase
// range rate fields of a satellite and signal cell from an MSM7.
func (cell *Cell) GetMSM7Doppler() (float64, error) {
	// RTKLIB save_msm_obs calculates the phase range, multiplies it by
	// the wavelength of the signal, reverses the sign of the result and
	// calls it the Doppler:
	//
	// /* doppler (hz) */
	// if (rr&&rrf&&rrf[j]>-1E12&&wl>0.0) {
	//     rtcm->obs.data[index].D[ind[k]]=(float)(-(rr[i]+rrf[j])/wl);
	// }
	//
	// When an MSM7 is converted to RINEX format, this value appears in one of
	// the fields, so we can test the handling using data collected from a real
	// device.

	phaseRangeRateMillis, err := cell.PhaseRangeRate()

	if err != nil {
		return 0.0, err
	}

	return (phaseRangeRateMillis / cell.Wavelength) * -1, nil
}

// GetAggregatePhaseRangeRate returns the phase range rate as an int, scaled up
// by 10,000
func (cell *Cell) GetAggregatePhaseRangeRate() int64 {

	// This is similar to getAggregateRange  but for the phase range rate.

	if cell.PhaseRangeRateFromSatelliteCell == InvalidPhaseRangeRate {
		return 0
	}

	var delta int

	if cell.PhaseRangeDelta != InvalidPhaseRangeRateDelta {
		delta = cell.PhaseRangeRateDelta
	}

	return utils.GetScaledPhaseRangeRate(cell.PhaseRangeRateFromSatelliteCell, delta)
}

// GetSignalCells gets the data from the signal cells of an MSM7 message.
func GetSignalCells(bitStream []byte, startOfSignalCells uint, header *msmHeader.Header, satCells []satellite.Cell) ([][]Cell, error) {
	// The third part of the message bit stream is the signal data.  Each satellite can
	// send many signals, each on a different frequency.  For example, if we observe one
	// signal from satellite 2, two from satellite 3 and 2 from satellite 15, there will
	// be five sets of signal data.  Irritatingly they are not laid out in a convenient
	// way.  First, we get the pseudo range delta values for each of the five signals,
	// followed by all of the phase range delta values, and so on.
	//
	// It's more convenient to present these as a slice of slices of fields, one outer
	// slice for each satellite and one inner slice for each observed signal.
	//
	// If the multiple message flag is set in the header, the message is one of a set
	// with the same epoch time and station ID.  Each message in the set contains some
	// of the signals.  If the multiple message flag is not set then we expect the
	// message to contain all the signals.

	// Define the lengths of the fields
	const lenRangeDelta uint = 20
	const lenPhaseRangeDelta uint = 24
	const lenLockTimeIndicator uint = 10
	const lenHalfCycleAmbiguity uint = 1
	const lenCNR uint = 10
	const lenPhaseRangeRateDelta uint = 15

	const bitsPerCell = lenRangeDelta + lenPhaseRangeDelta +
		lenLockTimeIndicator + lenHalfCycleAmbiguity + lenCNR + lenPhaseRangeRateDelta

	// Pos is the position within the message bitstream.
	pos := startOfSignalCells

	// Find the number of signal cells, ignoring any padding.

	numSignalCells := utils.GetNumberOfSignalCells(bitStream, pos, bitsPerCell)

	if !header.MultipleMessage {
		// This message should contain all the signal cells.  Check that
		// there are the expected number.
		if numSignalCells < header.NumSignalCells {
			message := fmt.Sprintf("overrun - want %d MSM7 signals, got %d",
				header.NumSignalCells, numSignalCells)
			return nil, errors.New(message)
		}
	}

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

	// Get the CNRs.
	cnr := make([]uint, 0)
	for i := 0; i < numSignalCells; i++ {
		c := uint(utils.GetBitsAsUint64(bitStream, pos, lenCNR))
		pos += lenCNR
		cnr = append(cnr, c)
	}

	// Get the phase range rate deltas (MSM7 only)
	phaseRangeRateDelta := make([]int, 0)
	for i := 0; i < numSignalCells; i++ {
		delta := int(utils.GetBitsAsInt64(bitStream, pos, lenPhaseRangeRateDelta))
		pos += lenPhaseRangeRateDelta
		phaseRangeRateDelta = append(phaseRangeRateDelta, delta)
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

	signalCells := make([][]Cell, len(header.Satellites))

	// c is the index into the slices of signal fields captured above.
	c := 0

	for i := range header.Cells {
		signalCells[i] = make([]Cell, 0)
		for j := range header.Cells[i] {
			if header.Cells[i][j] {

				signalID := header.Signals[j]

				wavelength := utils.GetWavelength(header.Constellation, signalID)

				cell := New(signalID, &satCells[i], rangeDelta[c], phaseRangeDelta[c],
					lockTimeIndicator[c], halfCycleAmbiguity[c], cnr[c],
					phaseRangeRateDelta[c], wavelength)

				signalCells[i] = append(signalCells[i], *cell)

				// Prepare to process the next set of signal fields.
				c++
			}
		}
	}

	return signalCells, nil
}

// GetAggregatePhaseRange takes a header, satellite cell and signal cell, extracts
// the phase range values, aggregates them and returns them as a 41-bit scaled unsigned
// unsigned integer, 8 bits whole part and 33 bits fractional part.  Use
// getPhaseRangeCycles to convert this to the phase range in cycles.
func (cell *Cell) GetAggregatePhaseRange() uint64 {

	// This is similar to GetAggregateRange  but for the phase range.  The phase
	// range value in the signal cell is merged with the range values ifromthe
	// satellite cell.

	if cell.RangeWholeMillisFromSatelliteCell == invalidRange {
		return 0
	}

	var delta int

	if cell.PhaseRangeDelta == InvalidPhaseRangeDelta {
		// The range is valid but the delta is not.  Use just the range.
		delta = 0
	} else {
		// The range and the delta are valid.  Use both.
		delta = cell.PhaseRangeDelta
	}

	scaledPhaseRange := utils.GetScaledPhaseRange(
		cell.RangeWholeMillisFromSatelliteCell,
		cell.RangeFractionalMillisFromSatelliteCell,
		delta,
	)

	return scaledPhaseRange
}
