// package signal contains code to handle the data from a signal cell from a
// Multiple Signal Message type 7 (message type 1077, 1087 etc.).  Various
// values are defined from values in the signal cell and its associated satellite
// cell.  For convenience the ones from the satellite cell are copied here.
package signal

import (
	"errors"
	"fmt"
	"log/slog"

	msmHeader "github.com/goblimey/go-ntrip/rtcm/header"
	"github.com/goblimey/go-ntrip/rtcm/type_msm7/satellite"
	"github.com/goblimey/go-ntrip/rtcm/utils"
)

// Define the lengths of the fields in the signal cell of an MSM7 bitstream.
const lenRangeDelta uint = 20
const lenPhaseRangeDelta uint = 24
const lenLockTimeIndicator uint = 10
const lenHalfCycleAmbiguity uint = 1
const lenCNR uint = 10
const lenPhaseRangeRateDelta uint = 15

const bitsPerCell = lenRangeDelta + lenPhaseRangeDelta +
	lenLockTimeIndicator + lenHalfCycleAmbiguity + lenCNR + lenPhaseRangeRateDelta

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

	// ID is the ID of the signal, 1-32.
	ID uint

	// Wavelength is the wavelength of the signal
	Wavelength float64

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
	// zero (-16384).  The value is in tenth millimetres per second. The true value of the
	// signal's phase range rate is derived by scaling this (positive or negative) delta and
	// adding it to the approximate value from the satellite cell.
	PhaseRangeRateDelta int

	Satellite *satellite.Cell

	// LogLevel controls the data output by String.
	LogLevel slog.Level
}

// New creates an MSM7 Signal Cell.
func New(
	signalID uint,
	satelliteCell *satellite.Cell,
	rangeDelta int,
	phaseRangeDelta int,
	lockTimeIndicator uint,
	halfCycleAmbiguity bool,
	cnr uint,
	phaseRangeRateDelta int,
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
		PhaseRangeRateDelta: phaseRangeRateDelta,
		Satellite:           satelliteCell,
		LogLevel:            logLevel,
	}

	return &cell
}

// String returns a readable version of a signal cell.
func (cell *Cell) String() string {
	if cell.LogLevel == slog.LevelDebug {
		var rangeMillisecs string
		if cell.Satellite.RangeWholeMillis == utils.InvalidRange {
			rangeMillisecs = "invalid"
		} else {
			// Convert the delta to float and divide by two to the power 29 to restore
			// the scale.  That gives the delta in milliseconds.
			rangeDeltaInMillis := float64(cell.RangeDelta) / float64(utils.TwoToThePower29)

			rangeDeltaInMetres := rangeDeltaInMillis * utils.OneLightMillisecond

			rangeMillisecs = fmt.Sprintf("(%d, %.3f, %.3f)",
				cell.RangeDelta, rangeDeltaInMetres, cell.RangeInMetres())
		}

		var phaseRangeMillisecs string
		switch {
		case cell.Satellite.RangeWholeMillis == utils.InvalidRange:
			phaseRangeMillisecs = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeMillisecs = "no wavelength"
		default:
			phaseRangeMillisecs = fmt.Sprintf("(%d, %.3f)",
				cell.PhaseRangeDelta, cell.PhaseRange())
		}

		var phaseRangeRateMetresPerSecond string
		switch {
		case cell.Satellite.PhaseRangeRate == InvalidPhaseRangeRate:
			phaseRangeRateMetresPerSecond = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeRateMetresPerSecond = "no wavelength"
		default:
			scaledDelta := utils.GetScaledPhaseRangeRate(0, cell.PhaseRangeRateDelta)
			// The delta is metres per second  scaled up by 10,000.
			deltaMPerSec := float64(scaledDelta) / 10000
			// The phase range rate value is rendered as the "doppler"
			// value - the rate in metres per second divided by the wavelength.
			phaseRangeRateMetresPerSecond = fmt.Sprintf("(%d, %.3f, %.3f)",
				cell.PhaseRangeRateDelta, deltaMPerSec, cell.PhaseRangeRate())
		}

		// The phase range rate doppler matches the doppler value in Rinex format.
		var phaseRangeRateDoppler string
		switch {
		case cell.Satellite.PhaseRangeRate == InvalidPhaseRangeRate:
			phaseRangeRateDoppler = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeRateDoppler = "no wavelength"
		default:
			phaseRangeRateDoppler = fmt.Sprintf("%.3f", cell.PhaseRangeRateDoppler())
		}

		return fmt.Sprintf("%2d %2d {%s, %s, %s, %s, %d, %v, %d, %.3f}",
			cell.Satellite.ID, cell.ID, rangeMillisecs, phaseRangeMillisecs,
			phaseRangeRateDoppler, phaseRangeRateMetresPerSecond,
			cell.LockTimeIndicator, cell.HalfCycleAmbiguity,
			cell.CarrierToNoiseRatio, cell.Wavelength)
	} else {

		var rangeMetres string
		if cell.Satellite.RangeWholeMillis == utils.InvalidRange {
			rangeMetres = "invalid"
		} else {
			rangeMetres = fmt.Sprintf("%12.3f", cell.RangeInMetres())
		}

		var phaseRangeMillisecs string
		switch {
		case cell.Satellite.RangeWholeMillis == utils.InvalidRange:
			phaseRangeMillisecs = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeMillisecs = "no wavelength"
		default:
			phaseRangeMillisecs = fmt.Sprintf("%13.3f", cell.PhaseRange())
		}

		// The phase range rate doppler matches the doppler value in Rinex format.
		var phaseRangeRateDoppler string
		switch {
		case cell.Satellite.PhaseRangeRate == InvalidPhaseRangeRate:
			phaseRangeRateDoppler = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeRateDoppler = "no wavelength"
		default:
			phaseRangeRateDoppler = fmt.Sprintf("%9.3f", cell.PhaseRangeRateDoppler())
		}

		var phaseRangeRateMetresPerSecond string
		switch {
		case cell.Satellite.PhaseRangeRate == InvalidPhaseRangeRate:
			phaseRangeRateMetresPerSecond = "invalid"
		case cell.Wavelength == 0:
			// The calculation involves dividing by the frequency
			// so that must be non-zero.
			phaseRangeRateMetresPerSecond = "no wavelength"
		default:
			// The phase range rate value is rendered as the "doppler"
			// value - the rate in metres per second divided by the wavelength.
			phaseRangeRateMetresPerSecond = fmt.Sprintf("%8.3f", cell.PhaseRangeRate())
		}

		return fmt.Sprintf("%2d %2d %s, %s, %s, %s, %d, %v, %d, %.3f",
			cell.Satellite.ID, cell.ID, rangeMetres, phaseRangeMillisecs,
			phaseRangeRateDoppler, phaseRangeRateMetresPerSecond,
			cell.LockTimeIndicator, cell.HalfCycleAmbiguity,
			cell.CarrierToNoiseRatio, cell.Wavelength)
	}
}

// GetAggregateRange takes the range values from an MSM7 signal cell (including some
// copied from the MSM7satellite cell) and returns the range as a 37-bit scaled unsigned
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

	if cell.RangeDelta == InvalidRangeDelta {
		// The range is valid but the delta is not.
		return utils.GetScaledRange(cell.Satellite.RangeWholeMillis,
			cell.Satellite.RangeFractionalMillis, 0)
	}

	// The delta value is valid.
	return utils.GetScaledRange(cell.Satellite.RangeWholeMillis,
		cell.Satellite.RangeFractionalMillis, cell.RangeDelta)
}

// RangeInMetres gives the distance from the satellite to the GPS device derived from
// the values in the satellite and signal cell, converted to metres.
func (cell *Cell) RangeInMetres() float64 {

	// Get the range as a 37-bit scaled integer, 8 bits whole, 29 bits fractional
	// representing the transit time in milliseconds.
	scaledRange := cell.GetAggregateRange()

	// Convert to float and divide by two to the power 29 to restore the scale.
	rangeInMillis := float64(scaledRange) / float64(utils.TwoToThePower29)

	// Use the speed of light to convert that to the distance from the
	// satellite to the receiver.
	rangeInMetres := rangeInMillis * utils.OneLightMillisecond

	return rangeInMetres
}

// PhaseRange combines the range and the phase range from an MSM7
// message and returns the result in cycles. It returns zero if the input
// measurements are invalid and an error if the signal is not in use.
func (cell *Cell) PhaseRange() float64 {

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

	return phaseRangeCycles
}

// PhaseRangeRate combines the components of the phase range rate
// in an MSM7 message and returns the result in milliseconds.  If the rate
// value in the satellite cell is invalid, the result is zero.  If the delta
// in the signal cell is invalid, the result is based on the rate value in the
// satellite.
func (cell *Cell) PhaseRangeRate() float64 {

	// Blewitt's paper says that the phase range rate is the rate at which the
	// which the satellite is approaching or (if negative) receding from
	// the GPS device.

	aggregatePhaseRangeRate := cell.GetAggregatePhaseRangeRate()

	// The aggregate is metres per second  scaled up by 10,000.
	phaseRangeRateMillis := float64(aggregatePhaseRangeRate) / 10000

	return phaseRangeRateMillis
}

// PhaseRangeRateDoppler gets the doppler value in Hz from the phase
// range rate fields of a satellite and signal cell of an MSM7.
func (cell *Cell) PhaseRangeRateDoppler() float64 {
	// RTKLIB save_msm_obs calculates the phase range, multiplies it by
	// the wavelength of the signal, reverses the sign of the result and
	// calls it the Doppler:
	//
	// /* doppler (hz) */
	// if (rr&&rrf&&rrf[j]>-1E12&&wl>0.0) {
	//     rtcm->obs.data[index].D[ind[k]]=(float)(-(rr[i]+rrf[j])/wl);
	// }
	//
	// When an MSM7 is converted to RINEX format, this value appears as the
	// Doppler (in the third field) so we can test the handling using data
	// collected from a real device.

	phaseRangeRateMetresPerSecond := cell.PhaseRangeRate()

	return (phaseRangeRateMetresPerSecond / cell.Wavelength) * -1
}

// GetAggregatePhaseRangeRate returns the phase range rate as an int, scaled up
// by 10,000
func (cell *Cell) GetAggregatePhaseRangeRate() int64 {

	// This is similar to getAggregateRange  but for the phase range rate.

	if cell.Satellite == nil {
		return 0
	}

	if cell.Satellite.PhaseRangeRate == InvalidPhaseRangeRate {
		return 0
	}

	var delta int

	if cell.PhaseRangeRateDelta != InvalidPhaseRangeRateDelta {
		delta = cell.PhaseRangeRateDelta
	}

	return utils.GetScaledPhaseRangeRate(cell.Satellite.PhaseRangeRate, delta)
}

// GetSignalCells gets the data from the signal cells of an MSM7 message.
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
	// followed by all of the phase range delta values, and so on.
	//
	// It's more convenient to present these as a slice of slices of fields, one outer
	// slice for each satellite and one inner slice for each observed signal.
	//
	// If the multiple message flag is set in the header, the message is one of a set
	// with the same timestamp and station ID.  Each message in the set contains some
	// of the signals.  If the multiple message flag is not set then we expect the
	// message to contain all the signals.

	// Pos is the position within the message bitstream.
	pos := startOfSignalCells
	bitsInStream := uint(len(bitStream) * 8)
	bitsLeft := bitsInStream - pos

	// Find the number of signal cells, ignoring any padding.

	numSignalCells := utils.GetNumberOfSignalCells(bitStream, pos, bitsPerCell)

	if header.MultipleMessage {
		// The message doesn't contain all the signal cells but there should be
		// at least one.
		if bitsLeft < bitsPerCell {
			message := fmt.Sprintf("overrun - want at least one %d-bit signal cell when multiple message flag is set, got only %d bits left",
				bitsPerCell, bitsLeft)
			return nil, errors.New(message)
		}
	} else {
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
			// Beware!  We are cranking through the 1 bits in the cell mask and if
			// the multiple message flag is set, not all those cells are there.
			if c < numSignalCells {
				if header.Cells[i][j] {

					signalID := header.Signals[j]

					wavelength := utils.GetSignalWavelength(header.Constellation, signalID)

					cell := New(
						signalID,
						&(satCells[i]),
						rangeDelta[c],
						phaseRangeDelta[c],
						lockTimeIndicator[c],
						halfCycleAmbiguity[c],
						cnr[c],
						phaseRangeRateDelta[c],
						wavelength,
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

// GetAggregatePhaseRange takes a header, satellite cell and signal cell, extracts
// the phase range values, aggregates them and returns them as a 41-bit scaled unsigned
// unsigned integer, 8 bits whole part and 33 bits fractional part.  Use
// getPhaseRangeCycles to convert this to the phase range in cycles.
func (cell *Cell) GetAggregatePhaseRange() uint64 {

	// This is similar to GetAggregateRange  but for the phase range.  The phase
	// range value in the signal cell is merged with the range values fromthe
	// satellite cell.

	if cell.Satellite.RangeWholeMillis == utils.InvalidRange {
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
		cell.Satellite.RangeWholeMillis,
		cell.Satellite.RangeFractionalMillis,
		delta,
	)

	return scaledPhaseRange
}
