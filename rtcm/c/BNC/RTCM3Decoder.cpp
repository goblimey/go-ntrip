// Part of BNC, a utility for retrieving decoding and
// converting GNSS data streams from NTRIP broadcasters.
//
// Copyright (C) 2007
// German Federal Agency for Cartography and Geodesy (BKG)
// http://www.bkg.bund.de
// Czech Technical University Prague, Department of Geodesy
// http://www.fsv.cvut.cz
//
// Email: euref-ip@bkg.bund.de
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation, version 2.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program; if not, write to the Free Software
// Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA 02111-1307, USA.

/* -------------------------------------------------------------------------
 * BKG NTRIP Client
 * -------------------------------------------------------------------------
 *
 * Class:      RTCM3Decoder
 *
 * Purpose:    RTCM3 Decoder
 *
 * Author:     L. Mervart
 *
 * Created:    24-Aug-2006
 *
 * Changes:
 *
 * -----------------------------------------------------------------------*/

#include <iostream>
#include <iomanip>
#include <sstream>
#include <math.h>
#include <string.h>

#include "bits.h"
#include "gnss.h"
#include "RTCM3Decoder.h"
#include "rtcm_utils.h"
#include "bncconst.h"
#include "bnccore.h"
#include "bncutils.h"
#include "bncsettings.h"

using namespace std;

// Error Handling
////////////////////////////////////////////////////////////////////////////
void RTCM3Error(const char*, ...) {
}

// Constructor
////////////////////////////////////////////////////////////////////////////
RTCM3Decoder::RTCM3Decoder(const QString& staID, bncRawFile* rawFile) :
    GPSDecoder() {

  _staID = staID;
  _rawFile = rawFile;

  connect(this, SIGNAL(newGPSEph(t_ephGPS)), BNC_CORE,
      SLOT(slotNewGPSEph(t_ephGPS)));
  connect(this, SIGNAL(newGlonassEph(t_ephGlo)), BNC_CORE,
      SLOT(slotNewGlonassEph(t_ephGlo)));
  connect(this, SIGNAL(newGalileoEph(t_ephGal)), BNC_CORE,
      SLOT(slotNewGalileoEph(t_ephGal)));
  connect(this, SIGNAL(newSBASEph(t_ephSBAS)), BNC_CORE,
      SLOT(slotNewSBASEph(t_ephSBAS)));
  connect(this, SIGNAL(newBDSEph(t_ephBDS)), BNC_CORE,
      SLOT(slotNewBDSEph(t_ephBDS)));

  _MessageSize = _SkipBytes = _BlockSize = _NeedBytes = 0;
}

// Destructor
////////////////////////////////////////////////////////////////////////////
RTCM3Decoder::~RTCM3Decoder() {
  QMapIterator<QByteArray, RTCM3coDecoder*> it(_coDecoders);
  while (it.hasNext())
  {
    it.next();
    delete it.value();
  }
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeRTCM3GPS(unsigned char* data, int size) {
  bool decoded = false;
  bncTime CurrentObsTime;
  int i, numsats, syncf, type;
  uint64_t numbits = 0, bitfield = 0;

  data += 3; /* header */
  size -= 6; /* header + crc */

  GETBITS(type, 12)
  SKIPBITS(12)
  /* id */
  GETBITS(i, 30)

  CurrentObsTime.set(i);
  if (_CurrentTime.valid() && CurrentObsTime != _CurrentTime) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentObsList.clear();
  }

  _CurrentTime = CurrentObsTime;

  GETBITS(syncf, 1)
  /* sync */
  GETBITS(numsats, 5)
  SKIPBITS(4)
  /* smind, smint */

  while (numsats--) {
    int sv, code, l1range, amb = 0;
    t_satObs CurrentObs;
    CurrentObs._time = CurrentObsTime;
    CurrentObs._type = type;

    GETBITS(sv, 6)
    if (sv < 40)
      CurrentObs._prn.set('G', sv);
    else
      CurrentObs._prn.set('S', sv - 20);

    t_frqObs *frqObs = new t_frqObs;
    /* L1 */
    GETBITS(code, 1);
    (code) ?
        frqObs->_rnxType2ch.assign("1W") : frqObs->_rnxType2ch.assign("1C");
    GETBITS(l1range, 24);
    GETBITSSIGN(i, 20);
    if ((i & ((1 << 20) - 1)) != 0x80000) {
      frqObs->_code = l1range * 0.02;
      frqObs->_phase = (l1range * 0.02 + i * 0.0005) / GPS_WAVELENGTH_L1;
      frqObs->_codeValid = frqObs->_phaseValid = true;
    }
    GETBITS(frqObs->_lockTimeIndicator, 7);
    frqObs->_lockTime = lti2sec(type, frqObs->_lockTimeIndicator);
    frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0 && frqObs->_phaseValid);
    if (type == 1002 || type == 1004) {
      GETBITS(amb, 8);
      if (amb) {
        frqObs->_code += amb * 299792.458;
        frqObs->_phase += (amb * 299792.458) / GPS_WAVELENGTH_L1;
      }
      GETBITS(i, 8);
      if (i) {
        frqObs->_snr = i * 0.25;
        frqObs->_snrValid = true;
      }
    }
    CurrentObs._obs.push_back(frqObs);
    if (type == 1003 || type == 1004) {
      frqObs = new t_frqObs;
      /* L2 */
      GETBITS(code, 2);
      switch (code) {
        case 3:
          frqObs->_rnxType2ch.assign("2W"); /* or "2Y"? */
          break;
        case 2:
          frqObs->_rnxType2ch.assign("2W");
          break;
        case 1:
          frqObs->_rnxType2ch.assign("2P");
          break;
        case 0:
          frqObs->_rnxType2ch.assign("2X"); /* or "2S" or "2L"? */
          break;
      }
      GETBITSSIGN(i, 14);
      if ((i & ((1 << 14) - 1)) != 0x2000) {
        frqObs->_code = l1range * 0.02 + i * 0.02 + amb * 299792.458;
        frqObs->_codeValid = true;
      }
      GETBITSSIGN(i, 20);
      if ((i & ((1 << 20) - 1)) != 0x80000) {
        frqObs->_phase = (l1range * 0.02 + i * 0.0005 + amb * 299792.458)
            / GPS_WAVELENGTH_L2;
        frqObs->_phaseValid = true;
      }
      GETBITS(frqObs->_lockTimeIndicator, 7);
      frqObs->_lockTime = lti2sec(type, frqObs->_lockTimeIndicator);
      frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0 && frqObs->_phaseValid);
      if (type == 1004) {
        GETBITS(i, 8);
        if (i) {
          frqObs->_snr = i * 0.25;
          frqObs->_snrValid = true;
        }
      }
      CurrentObs._obs.push_back(frqObs);
    }
    _CurrentObsList.push_back(CurrentObs);
  }

  if (!syncf) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentTime.reset();
    _CurrentObsList.clear();
  }
  return decoded;
}

#define RTCM3_MSM_NUMSIG      32
#define RTCM3_MSM_NUMSAT      64
#define RTCM3_MSM_NUMCELLS    96 /* arbitrary limit */

/**
 * Frequency numbers of GLONASS with an offset of 100 to detect unset values.
 * Gets filled by ephemeris and data blocks and shared between different streams.
 */
static int GLOFreq[RTCM3_MSM_NUMSAT];

/*
 * Storage structure to store frequency and RINEX ID assignment for MSM
 * message */
struct CodeData {
  double wl;
  const char *code; /* currently unused */
};

/** MSM signal types for GPS and SBAS */
static struct CodeData gps[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {GPS_WAVELENGTH_L1, "1C"},
        {GPS_WAVELENGTH_L1, "1P"},
        {GPS_WAVELENGTH_L1, "1W"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L2, "2C"},
        {GPS_WAVELENGTH_L2, "2P"},
        {GPS_WAVELENGTH_L2, "2W"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L2, "2S"},
        {GPS_WAVELENGTH_L2, "2L"},
        {GPS_WAVELENGTH_L2, "2X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L5, "5I"},
        {GPS_WAVELENGTH_L5, "5Q"},
        {GPS_WAVELENGTH_L5, "5X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L1, "1S"},
        {GPS_WAVELENGTH_L1, "1L"},
        {GPS_WAVELENGTH_L1, "1X"}
    };

/**
 * MSM signal types for GLONASS
 *
 * NOTE: Uses 0.0, 1.0 for wavelength as sat index dependence is done later!
 */
static struct CodeData glo[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {0.0, "1C"},
        {0.0, "1P"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {1.0, "2C"},
        {1.0, "2P"},
        {GLO_WAVELENGTH_L1a, "4A"},
        {GLO_WAVELENGTH_L1a, "4B"},
        {GLO_WAVELENGTH_L1a, "4X"},
        {GLO_WAVELENGTH_L2a, "6A"},
        {GLO_WAVELENGTH_L2a, "6B"},
        {GLO_WAVELENGTH_L2a, "6X"},
        {GLO_WAVELENGTH_L3,  "3I"},
        {GLO_WAVELENGTH_L3,  "3Q"},
        {GLO_WAVELENGTH_L3,  "3X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0}
    };

/** MSM signal types for Galileo */
static struct CodeData gal[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {GAL_WAVELENGTH_E1,  "1C"},
        {GAL_WAVELENGTH_E1,  "1A"},
        {GAL_WAVELENGTH_E1,  "1B"},
        {GAL_WAVELENGTH_E1,  "1X"},
        {GAL_WAVELENGTH_E1,  "1Z"},
        {0.0, 0},
        {GAL_WAVELENGTH_E6,  "6C"},
        {GAL_WAVELENGTH_E6,  "6A"},
        {GAL_WAVELENGTH_E6,  "6B"},
        {GAL_WAVELENGTH_E6,  "6X"},
        {GAL_WAVELENGTH_E6,  "6Z"},
        {0.0, 0},
        {GAL_WAVELENGTH_E5B, "7I"},
        {GAL_WAVELENGTH_E5B, "7Q"},
        {GAL_WAVELENGTH_E5B, "7X"},
        {0.0, 0},
        {GAL_WAVELENGTH_E5AB,"8I"},
        {GAL_WAVELENGTH_E5AB,"8Q"},
        {GAL_WAVELENGTH_E5AB,"8X"},
        {0.0, 0},
        {GAL_WAVELENGTH_E5A, "5I"},
        {GAL_WAVELENGTH_E5A, "5Q"},
        {GAL_WAVELENGTH_E5A, "5X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0}
    };

/** MSM signal types for QZSS */
static struct CodeData qzss[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {GPS_WAVELENGTH_L1, "1C"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {QZSS_WAVELENGTH_L6, "6S"},
        {QZSS_WAVELENGTH_L6, "6L"},
        {QZSS_WAVELENGTH_L6, "6X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L2, "2S"},
        {GPS_WAVELENGTH_L2, "2L"},
        {GPS_WAVELENGTH_L2, "2X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L5, "5I"},
        {GPS_WAVELENGTH_L5, "5Q"},
        {GPS_WAVELENGTH_L5, "5X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {GPS_WAVELENGTH_L1, "1S"},
        {GPS_WAVELENGTH_L1, "1L"},
        {GPS_WAVELENGTH_L1, "1X"}
    };

/** MSM signal types for Beidou/BDS */
static struct CodeData bds[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {BDS_WAVELENGTH_B1, "2I"},
        {BDS_WAVELENGTH_B1, "2Q"},
        {BDS_WAVELENGTH_B1, "2X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {BDS_WAVELENGTH_B3, "6I"},
        {BDS_WAVELENGTH_B3, "6Q"},
        {BDS_WAVELENGTH_B3, "6X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {BDS_WAVELENGTH_B2, "7I"},
        {BDS_WAVELENGTH_B2, "7Q"},
        {BDS_WAVELENGTH_B2, "7X"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {BDS_WAVELENGTH_B2a, "5D"},
        {BDS_WAVELENGTH_B2a, "5P"},
        {BDS_WAVELENGTH_B2a, "5X"},
        {BDS_WAVELENGTH_B2b, "7D"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {BDS_WAVELENGTH_B1C, "1D"},
        {BDS_WAVELENGTH_B1C, "1P"},
        {BDS_WAVELENGTH_B1C, "1X"}
    };

/** MSM signal types for IRNSS */
static struct CodeData irn[RTCM3_MSM_NUMSIG] = {
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {IRNSS_WAVELENGTH_S, "9A"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {IRNSS_WAVELENGTH_L5, "5A"},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0},
        {0.0, 0}
    };

#define UINT64(c) c ## ULL

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeRTCM3MSM(unsigned char* data, int size) {
  bool decoded = false;
  int type, syncf, i;
  uint64_t numbits = 0, bitfield = 0;

  data += 3; /* header */
  size -= 6; /* header + crc */

  GETBITS(type, 12)
  SKIPBITS(12)
  /* id */
  char sys;
  if      (type >= 1131 && type <= 1137) {
    sys = 'I';
  }
  else if (type >= 1121 && type <= 1127) {
    sys = 'C';
  }
  else if (type >= 1111 && type <= 1117) {
    sys = 'J';
  }
  else if (type >= 1101 && type <= 1007) {
    sys = 'S';
  }
  else if (type >= 1091 && type <= 1097) {
    sys = 'E';
  }
  else if (type >= 1081 && type <= 1087) {
    sys = 'R';
  }
  else if (type >= 1071 && type <= 1077) {
    sys = 'G';
  }
  else {
    return decoded; // false
  }
  bncTime CurrentObsTime;
  if      (sys == 'C') /* BDS */ {
    GETBITS(i, 30)
    CurrentObsTime.setBDS(i);
  }
  else if (sys == 'R') /* GLONASS */ {
    SKIPBITS(3)
    GETBITS(i, 27)
    /* tk */
    CurrentObsTime.setTk(i);
  }
  else /* GPS style date */ {
    GETBITS(i, 30)
    CurrentObsTime.set(i);
  }
  if (_CurrentTime.valid() && CurrentObsTime != _CurrentTime) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentObsList.clear();
  }
  _CurrentTime = CurrentObsTime;

  GETBITS(syncf, 1)
  /**
   * Ignore unknown types except for sync flag
   *
   * We actually support types 1-3 in following code, but as they are missing
   * the full cycles and can't be used later we skip interpretation here already.
   */
  if (type <= 1137 && (type % 10) >= 4 && (type % 10) <= 7) {
    int sigmask, numsat = 0, numsig = 0;
    uint64_t satmask, cellmask, ui;
    // satellite data
    double rrmod[RTCM3_MSM_NUMSAT]; // GNSS sat rough ranges modulo 1 millisecond
    int    rrint[RTCM3_MSM_NUMSAT]; // number of integer msecs in GNSS sat rough ranges
    int    rdop[RTCM3_MSM_NUMSAT];  // GNSS sat rough phase range rates
    int    extsat[RTCM3_MSM_NUMSAT];// extended sat info
    // signal data
    int    ll[RTCM3_MSM_NUMCELLS];  // lock time indicator
    /*int    hc[RTCM3_MSM_NUMCELLS];*/  // half cycle ambiguity indicator
    double cnr[RTCM3_MSM_NUMCELLS]; // signal cnr
    double cp[RTCM3_MSM_NUMCELLS];  // fine phase range data
    double psr[RTCM3_MSM_NUMCELLS]; // fine psr
    double dop[RTCM3_MSM_NUMCELLS]; // fine phase range rates

    SKIPBITS(3 + 7 + 2 + 2 + 1 + 3)
    GETBITS64(satmask, RTCM3_MSM_NUMSAT)

    /* http://gurmeetsingh.wordpress.com/2008/08/05/fast-bit-counting-routines/ */
    for (ui = satmask; ui; ui &= (ui - 1) /* remove rightmost bit */)
      ++numsat;
    GETBITS(sigmask, RTCM3_MSM_NUMSIG)
    for (i = sigmask; i; i &= (i - 1) /* remove rightmost bit */)
      ++numsig;
    for (i = 0; i < RTCM3_MSM_NUMSAT; ++i)
      extsat[i] = 15;

    i = numsat * numsig;
    GETBITS64(cellmask, (unsigned )i)
    // satellite data
    switch (type % 10) {
      case 1:
      case 2:
      case 3:
        /* partial data, already skipped above, but implemented for future expansion ! */
        for (int j = numsat; j--;)
          GETFLOAT(rrmod[j], 10, 1.0 / 1024.0)
        break;
      case 4:
      case 6:
        for (int j = numsat; j--;)
          GETBITS(rrint[j], 8)
        for (int j = numsat; j--;)
          GETFLOAT(rrmod[j], 10, 1.0 / 1024.0)
        break;
      case 5:
      case 7:
        for (int j = numsat; j--;)
          GETBITS(rrint[j], 8)
        for (int j = numsat; j--;)
          GETBITS(extsat[j], 4)
        for (int j = numsat; j--;)
          GETFLOAT(rrmod[j], 10, 1.0 / 1024.0)
        for (int j = numsat; j--;)
          GETBITSSIGN(rdop[j], 14)
        break;
    }
    // signal data
    int numcells = numsat * numsig;
    /** Drop anything which exceeds our cell limit. Increase limit definition
     * when that happens. */
    if (numcells <= RTCM3_MSM_NUMCELLS) {
      switch (type % 10) {
        case 1:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 15, 1.0 / (1 << 24))
          break;
        case 2:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 22, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 4)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          break;
        case 3:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 15, 1.0 / (1 << 24))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 22, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 4)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          break;
        case 4:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 15, 1.0 / (1 << 24))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 22, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 4)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(cnr[count], 6)
          break;
        case 5:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 15, 1.0 / (1 << 24))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 22, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 4)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOAT(cnr[count], 6, 1.0)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(dop[count], 15, 0.0001)
          break;
        case 6:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 20, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 24, 1.0 / (1U << 31))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 10)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOAT(cnr[count], 10, 1.0 / (1 << 4))
          break;
        case 7:
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(psr[count], 20, 1.0 / (1 << 29))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(cp[count], 24, 1.0 / (1U << 31))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETBITS(ll[count], 10)
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              SKIPBITS(1)/*GETBITS(hc[count], 1)*/
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOAT(cnr[count], 10, 1.0 / (1 << 4))
          for (int count = numcells; count--;)
            if (cellmask & (UINT64(1) << count))
              GETFLOATSIGN(dop[count], 15, 0.0001)
          break;
      }
      i = RTCM3_MSM_NUMSAT;
      int j = -1;
      t_satObs CurrentObs;
      for (int count = numcells; count--;) {
        while (j >= 0 && !(sigmask & (1 << --j)))
          ;
        if (j < 0) {
          while (!(satmask & (UINT64(1) << (--i))))
            /* next satellite */
            ;
          if (CurrentObs._obs.size() > 0)
            _CurrentObsList.push_back(CurrentObs);
          CurrentObs.clear();
          CurrentObs._time = CurrentObsTime;
          CurrentObs._type = type;
          if (sys == 'S')
            CurrentObs._prn.set(sys, 20 - 1 + RTCM3_MSM_NUMSAT - i);
          else
            CurrentObs._prn.set(sys, RTCM3_MSM_NUMSAT - i);
          j = RTCM3_MSM_NUMSIG;
          while (!(sigmask & (1 << --j)))
            ;
          --numsat;
        }
        if (cellmask & (UINT64(1) << count)) {
          struct CodeData cd = {0.0, 0};
          switch (sys) {
            case 'J':
              cd = qzss[RTCM3_MSM_NUMSIG - j - 1];
              break;
            case 'C':
              cd = bds[RTCM3_MSM_NUMSIG - j - 1];
              break;
            case 'G':
            case 'S':
              cd = gps[RTCM3_MSM_NUMSIG - j - 1];
              break;
            case 'R':
              cd = glo[RTCM3_MSM_NUMSIG - j - 1];
              {
                int k = GLOFreq[RTCM3_MSM_NUMSAT - i - 1];
                if (extsat[numsat] < 14) { // channel number is available as extended info for MSM5/7
                  k = GLOFreq[RTCM3_MSM_NUMSAT - i - 1] = 100 + extsat[numsat] - 7;
                }
                if (k) {
                  if      (cd.wl == 0.0) {
                    cd.wl = GLO_WAVELENGTH_L1(k - 100);
                  }
                  else if (cd.wl == 1.0) {
                    cd.wl = GLO_WAVELENGTH_L2(k - 100);
                  }
                }
                else if (!k && cd.wl <= 1) {
                  cd.code = 0;
                }
              }
              break;
            case 'E':
              cd = gal[RTCM3_MSM_NUMSIG - j - 1];
              break;
            case 'I':
              cd = irn[RTCM3_MSM_NUMSIG - j - 1];
              break;
          }
          if (cd.code) {
            t_frqObs *frqObs = new t_frqObs;
            frqObs->_rnxType2ch.assign(cd.code);

            switch (type % 10) {
              case 1:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                break;
              case 2:
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + (rrmod[numsat]) * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }
                break;
              case 3:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + rrmod[numsat] * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }
                break;
              case 4:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat] +  rrint[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + (rrmod[numsat] +  rrint[numsat]) * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }
                frqObs->_snr = cnr[count];
                frqObs->_snrValid = true;
                break;
              case 5:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }
                frqObs->_snr = cnr[count];
                frqObs->_snrValid = true;
                if (dop[count] > -1.6384) {
                  frqObs->_doppler = -(dop[count] + rdop[numsat]) / cd.wl;
                  frqObs->_dopplerValid = true;
                }
                break;
              case 6:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }

                frqObs->_snr = cnr[count];
                frqObs->_snrValid = true;
                break;
              case 7:
                if (psr[count] > -1.0 / (1 << 10)) {
                  frqObs->_code = psr[count] * LIGHTSPEED / 1000.0
                                + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0;
                  frqObs->_codeValid = true;
                }
                if (cp[count] > -1.0 / (1 << 8)) {
                  frqObs->_phase = cp[count] * LIGHTSPEED / 1000.0 / cd.wl
                                 + (rrmod[numsat] + rrint[numsat]) * LIGHTSPEED / 1000.0 / cd.wl;
                  frqObs->_phaseValid = true;
                  frqObs->_lockTime = lti2sec(type,ll[count]);
                  frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0);
                  frqObs->_lockTimeIndicator = ll[count];
                }

                frqObs->_snr = cnr[count];
                frqObs->_snrValid = true;

                if (dop[count] > -1.6384) {
                  frqObs->_doppler = -(dop[count] + rdop[numsat]) / cd.wl;
                  frqObs->_dopplerValid = true;
                }
                break;
            }
            CurrentObs._obs.push_back(frqObs);
          }
        }
      }
      if (CurrentObs._obs.size() > 0) {
        _CurrentObsList.push_back(CurrentObs);
      }
    }
  }
  else if ((type % 10) < 4) {
    emit(newMessage(QString("%1: Block %2 contain partial data! Ignored!")
        .arg(_staID).arg(type).toLatin1(), true));
  }
  if (!syncf) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentTime.reset();
    _CurrentObsList.clear();
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeRTCM3GLONASS(unsigned char* data, int size) {
  bool decoded = false;
  bncTime CurrentObsTime;
  int i, numsats, syncf, type;
  uint64_t numbits = 0, bitfield = 0;

  data += 3; /* header */
  size -= 6; /* header + crc */

  GETBITS(type, 12)
  SKIPBITS(12)
  /* id */
  GETBITS(i, 27)
  /* tk */

  CurrentObsTime.setTk(i);
  if (_CurrentTime.valid() && CurrentObsTime != _CurrentTime) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentObsList.clear();
  }
  _CurrentTime = CurrentObsTime;

  GETBITS(syncf, 1)
  /* sync */
  GETBITS(numsats, 5)
  SKIPBITS(4)
  /* smind, smint */

  while (numsats--) {
    int sv, code, l1range, amb = 0, freq;
    t_satObs CurrentObs;
    CurrentObs._time = CurrentObsTime;
    CurrentObs._type = type;

    GETBITS(sv, 6)
    CurrentObs._prn.set('R', sv);
    GETBITS(code, 1)
    GETBITS(freq, 5)
    GLOFreq[sv - 1] = 100 + freq - 7; /* store frequency for other users (MSM) */

    t_frqObs *frqObs = new t_frqObs;
    /* L1 */
    (code) ?
        frqObs->_rnxType2ch.assign("1P") : frqObs->_rnxType2ch.assign("1C");
    GETBITS(l1range, 25);
    GETBITSSIGN(i, 20);
    if ((i & ((1 << 20) - 1)) != 0x80000) {
      frqObs->_code = l1range * 0.02;
      frqObs->_phase = (l1range * 0.02 + i * 0.0005) / GLO_WAVELENGTH_L1(freq - 7);
      frqObs->_codeValid = frqObs->_phaseValid = true;
    }
    GETBITS(frqObs->_lockTimeIndicator, 7);
    frqObs->_lockTime = lti2sec(type, frqObs->_lockTimeIndicator);
    frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0 && frqObs->_phaseValid);
    if (type == 1010 || type == 1012) {
      GETBITS(amb, 7);
      if (amb) {
        frqObs->_code += amb * 599584.916;
        frqObs->_phase += (amb * 599584.916) / GLO_WAVELENGTH_L1(freq - 7);
      }
      GETBITS(i, 8);
      if (i) {
        frqObs->_snr = i * 0.25;
        frqObs->_snrValid = true;
      }
    }
    CurrentObs._obs.push_back(frqObs);
    if (type == 1011 || type == 1012) {
      frqObs = new t_frqObs;
      /* L2 */
      GETBITS(code, 2);
      switch (code) {
        case 3:
          frqObs->_rnxType2ch.assign("2P");
          break;
        case 2:
          frqObs->_rnxType2ch.assign("2P");
          break;
        case 1:
          frqObs->_rnxType2ch.assign("2P");
          break;
        case 0:
          frqObs->_rnxType2ch.assign("2C");
          break;
      }
      GETBITSSIGN(i, 14);
      if ((i & ((1 << 14) - 1)) != 0x2000) {
        frqObs->_code = l1range * 0.02 + i * 0.02 + amb * 599584.916;
        frqObs->_codeValid = true;
      }
      GETBITSSIGN(i, 20);
      if ((i & ((1 << 20) - 1)) != 0x80000) {
        frqObs->_phase = (l1range * 0.02 + i * 0.0005 + amb * 599584.916)
            / GLO_WAVELENGTH_L2(freq - 7);
        frqObs->_phaseValid = true;
      }
      GETBITS(frqObs->_lockTimeIndicator, 7);
      frqObs->_lockTime = lti2sec(type, frqObs->_lockTimeIndicator);
      frqObs->_lockTimeValid = (frqObs->_lockTime >= 0.0 && frqObs->_phaseValid);
      if (type == 1012) {
        GETBITS(i, 8);
        if (i) {
          frqObs->_snr = i * 0.25;
          frqObs->_snrValid = true;
        }
      }
      CurrentObs._obs.push_back(frqObs);
    }
    _CurrentObsList.push_back(CurrentObs);
  }
  if (!syncf) {
    decoded = true;
    _obsList.append(_CurrentObsList);
    _CurrentTime.reset();
    _CurrentObsList.clear();
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeGPSEphemeris(unsigned char* data, int size) {
  bool decoded = false;

  if (size == 67) {
    t_ephGPS eph;
    int i, week;
    uint64_t numbits = 0, bitfield = 0;
    int fitIntervalFalg = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(i, 6)
    eph._prn.set('G', i);
    GETBITS(week, 10)
    GETBITS(i, 4)
    eph._ura = accuracyFromIndex(i, eph.type());
    GETBITS(eph._L2Codes, 2)
    GETFLOATSIGN(eph._IDOT, 14, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETBITS(eph._IODE, 8)
    GETBITS(i, 16)
    i <<= 4;
    eph._TOC.set(i * 1000);
    GETFLOATSIGN(eph._clock_driftrate, 8, 1.0 / (double )(1 << 30) / (double )(1 << 25))
    GETFLOATSIGN(eph._clock_drift,    16, 1.0 / (double )(1 << 30) / (double )(1 << 13))
    GETFLOATSIGN(eph._clock_bias,     22, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETBITS(eph._IODC, 10)
    GETFLOATSIGN(eph._Crs,            16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._Delta_n,        16, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._M0,             32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cuc,            16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._e,                  32, 1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETFLOATSIGN(eph._Cus,            16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._sqrt_A,             32, 1.0 / (double )(1 << 19))
    if (eph._sqrt_A < 1000.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3) SQRT_A %4 m!")
           .arg(_staID).arg(1019,4).arg(eph._prn.toString().c_str())
           .arg(eph._sqrt_A,10,'F',3).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(i, 16)
    i <<= 4;
    eph._TOEsec = i;
    bncTime t;
    t.set(i * 1000);
    eph._TOEweek = t.gpsw();
    int numOfRollOvers = int(floor(t.gpsw()/1024.0));
    week += (numOfRollOvers * 1024);
    /* week from HOW, differs from TOC, TOE week, we use adapted value instead */
    if (eph._TOEweek > week + 1 || eph._TOEweek < week - 1) /* invalid week */
      return false;
    GETFLOATSIGN(eph._Cic,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._OMEGA0,   32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cis,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._i0,       32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Crc,      16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._omega,    32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._OMEGADOT, 24, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._TGD,       8, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETBITS(eph._health, 6)
    GETBITS(eph._L2PFlag, 1)
    GETBITS(fitIntervalFalg, 1)
    eph._fitInterval = fitIntervalFromFlag(fitIntervalFalg, eph._IODC, eph.type());
    eph._TOT = 0.9999e9;
    eph._navType = t_eph::LNAV;

    emit newGPSEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeGLONASSEphemeris(unsigned char* data, int size) {
  bool decoded = false;

  if (size == 51) {
    t_ephGlo eph;
    int sv, i, tk;
    uint64_t numbits = 0, bitfield = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(sv, 6)
    eph._prn.set('R', sv);

    GETBITS(i, 5)
    eph._frequency_number = i - 7;
    GETBITS(eph._almanac_health, 1) /* almanac healthy */
    GETBITS(eph._almanac_health_availablility_indicator, 1) /* almanac health ok */
    if (eph._almanac_health_availablility_indicator == 0.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): ALM = %4: missing data!")
           .arg(_staID).arg(1020,4).arg(eph._prn.toString().c_str())
           .arg(eph._almanac_health_availablility_indicator).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(eph._P1, 2) /*  P1 */
    GETBITS(i, 5)
    tk = i * 60 * 60;
    GETBITS(i, 6)
    tk += i * 60;
    GETBITS(i, 1)
    tk += i * 30;
    eph._tki = tk - 3*60*60;
    if(eph._tki < 0.0) {
      eph._tki += 86400.0;
    }
    GETBITS(eph._health, 1) /* MSB of Bn*/
    GETBITS(eph._P2, 1)  /* P2 */
    GETBITS(i, 7)
    eph._TOC.setTk(i * 15 * 60 * 1000); /* tb */

    GETFLOATSIGNM(eph._x_velocity,    24, 1.0 / (double )(1 << 20))
    GETFLOATSIGNM(eph._x_pos,         27, 1.0 / (double )(1 << 11))
    GETFLOATSIGNM(eph._x_acceleration, 5, 1.0 / (double )(1 << 30))
    GETFLOATSIGNM(eph._y_velocity,    24, 1.0 / (double )(1 << 20))
    GETFLOATSIGNM(eph._y_pos,         27, 1.0 / (double )(1 << 11))
    GETFLOATSIGNM(eph._y_acceleration, 5, 1.0 / (double )(1 << 30))
    GETFLOATSIGNM(eph._z_velocity,    24, 1.0 / (double )(1 << 20))
    GETFLOATSIGNM(eph._z_pos,         27, 1.0 / (double )(1 << 11))
    GETFLOATSIGNM(eph._z_acceleration, 5, 1.0 / (double )(1 << 30))
    GETBITS(eph._P3, 1)    /* P3 */
    GETFLOATSIGNM(eph._gamma,      11, 1.0 / (double )(1 << 30) / (double )(1 << 10))
    GETBITS(eph._M_P,  2) /* GLONASS-M P, */
    GETBITS(eph._M_l3, 1) /* GLONASS-M ln (third string) */
    GETFLOATSIGNM(eph._tau,        22, 1.0 / (double )(1 << 30))  /* GLONASS tau n(tb) */
    GETFLOATSIGNM(eph._M_delta_tau, 5, 1.0 / (double )(1 << 30))  /* GLONASS-M delta tau n(tb) */
    GETBITS(eph._E, 5)
    GETBITS(eph._M_P4,  1) /* GLONASS-M P4 */
    GETBITS(eph._M_FT,  4) /* GLONASS-M Ft */
    GETBITS(eph._M_NT, 11) /* GLONASS-M Nt */
    if (eph._M_NT == 0.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): NT = %4: missing data!")
           .arg(_staID).arg(1020,4).arg(eph._prn.toString().c_str()).arg(eph._M_NT,4).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(eph._M_M,   2) /* GLONASS-M M */
    GETBITS(eph._additional_data_availability, 1) /* GLONASS-M The Availability of Additional Data */
    if (eph._additional_data_availability == 0.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): ADD = %4: missing data!")
           .arg(_staID).arg(1020,4).arg(eph._prn.toString().c_str())
           .arg(eph._additional_data_availability).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(eph._NA,  11) /* GLONASS-M Na */
    GETFLOATSIGNM(eph._tauC,       32, 1.0/(double)(1<<30)/(double)(1<<1)) /* GLONASS tau c */
    GETBITS(eph._M_N4, 5) /* GLONASS-M N4 */
    GETFLOATSIGNM(eph._M_tau_GPS,  22, 1.0/(double)(1<<30)) /* GLONASS-M tau GPS */
    GETBITS(eph._M_l5, 1) /* GLONASS-M ln (fifth string) */

    unsigned year, month, day;
    eph._TOC.civil_date(year, month, day);
    eph._gps_utc = gnumleap(year, month, day);
    eph._tt = eph._TOC;

    eph._xv(1) = eph._x_pos * 1.e3;
    eph._xv(2) = eph._y_pos * 1.e3;
    eph._xv(3) = eph._z_pos * 1.e3;
    if (eph._xv.Rows(1,3).NormFrobenius() < 1.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): zero position!")
           .arg(_staID).arg(1020,4).arg(eph._prn.toString().c_str()).toLatin1(), true));
#endif
      return false;
    }
    eph._xv(4) = eph._x_velocity * 1.e3;
    eph._xv(5) = eph._y_velocity * 1.e3;
    eph._xv(6) = eph._z_velocity * 1.e3;
    if (eph._xv.Rows(4,6).NormFrobenius() < 1.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): zero velocity!")
           .arg(_staID).arg(1020,4).arg(eph._prn.toString().c_str()).toLatin1(), true));
#endif
      return false;
    }
    GLOFreq[sv - 1] = 100 + eph._frequency_number ; /* store frequency for other users (MSM) */
    _gloFrq = QString("%1 %2").arg(eph._prn.toString().c_str()).arg(eph._frequency_number, 2, 'f', 0);

    eph._navType = t_eph::FDMA;

    emit newGlonassEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeQZSSEphemeris(unsigned char* data, int size) {
  bool decoded = false;

  if (size == 67) {
    t_ephGPS eph;
    int i, week;
    uint64_t numbits = 0, bitfield = 0;
    int fitIntervalFalg = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(i, 4)
    eph._prn.set('J', i);

    GETBITS(i, 16)
    i <<= 4;
    eph._TOC.set(i * 1000);

    GETFLOATSIGN(eph._clock_driftrate, 8, 1.0 / (double )(1 << 30) / (double )(1 << 25))
    GETFLOATSIGN(eph._clock_drift,    16, 1.0 / (double )(1 << 30) / (double )(1 << 13))
    GETFLOATSIGN(eph._clock_bias,     22, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETBITS(eph._IODE, 8)
    GETFLOATSIGN(eph._Crs,     16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._Delta_n, 16, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._M0,      32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cuc,     16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._e,           32, 1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETFLOATSIGN(eph._Cus,     16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._sqrt_A,      32, 1.0 / (double )(1 << 19))
    if (eph._sqrt_A < 1000.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3) SQRT_A %4 m!")
           .arg(_staID).arg(1044,4).arg(eph._prn.toString().c_str())
           .arg(eph._sqrt_A,10,'F',3).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(i, 16)
    i <<= 4;
    eph._TOEsec = i;
    bncTime t;
    t.set(i*1000);
    eph._TOEweek = t.gpsw();
    GETFLOATSIGN(eph._Cic,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._OMEGA0,   32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cis,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._i0,       32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Crc,      16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._omega,    32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._OMEGADOT, 24, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._IDOT,     14, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETBITS(eph._L2Codes, 2)
    GETBITS(week, 10)
    int numOfRollOvers = int(floor(t.gpsw()/1024.0));
    week += (numOfRollOvers * 1024);
    /* week from HOW, differs from TOC, TOE week, we use adapted value instead */
    if (eph._TOEweek > week + 1 || eph._TOEweek < week - 1) /* invalid week */
      return false;

    GETBITS(i, 4)
    if (i <= 6)
      eph._ura = ceil(10.0 * pow(2.0, 1.0 + i / 2.0)) / 10.0;
    else
      eph._ura = ceil(10.0 * pow(2.0, i / 2.0)) / 10.0;
    GETBITS(eph._health, 6)
    GETFLOATSIGN(eph._TGD,       8, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETBITS(eph._IODC, 10)
    GETBITS(fitIntervalFalg, 1)
    eph._fitInterval = fitIntervalFromFlag(fitIntervalFalg, eph._IODC, eph.type());
    eph._TOT = 0.9999e9;
    eph._navType = t_eph::LNAV;

    emit newGPSEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeIRNSSEphemeris(unsigned char* data, int size) {
  bool decoded = false;

  if (size == 67) {
    t_ephGPS eph;
    int i, week, L5Flag, SFlag;
    uint64_t numbits = 0, bitfield = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(i, 6)
    eph._prn.set('I', i);
    GETBITS(week, 10)
    GETFLOATSIGN(eph._clock_bias,     22, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETFLOATSIGN(eph._clock_drift,    16, 1.0 / (double )(1 << 30) / (double )(1 << 13))
    GETFLOATSIGN(eph._clock_driftrate, 8, 1.0 / (double )(1 << 30) / (double )(1 << 25))
    GETBITS(i, 4)
    eph._ura = accuracyFromIndex(i, eph.type());
    GETBITS(i, 16)
    i <<= 4;
    eph._TOC.set(i * 1000);
    GETFLOATSIGN(eph._TGD, 8, 1.0 / (double )(1 << 30) / (double )(1 <<  1))
    GETFLOATSIGN(eph._Delta_n, 22, R2R_PI/(double)(1<<30)/(double)(1 << 11))
    // IODCE
    GETBITS(eph._IODE, 8)
    eph._IODC = eph._IODE;
    SKIPBITS(10)
    GETBITS(L5Flag, 1)
    GETBITS(SFlag, 1)
    if      (L5Flag == 0 && SFlag == 0) {
      eph._health = 0.0;
    }
    else if (L5Flag == 0 && SFlag == 1) {
      eph._health = 1.0;
    }
    else if (L5Flag == 1 && SFlag == 0) {
      eph._health = 2.0;
    }
    else if (L5Flag == 1 && SFlag == 1) {
      eph._health = 3.0;
    }
    GETFLOATSIGN(eph._Cuc,  15, 1.0 / (double )(1 << 28))
    GETFLOATSIGN(eph._Cus,  15, 1.0 / (double )(1 << 28))
    GETFLOATSIGN(eph._Cic,  15, 1.0 / (double )(1 << 28))
    GETFLOATSIGN(eph._Cis,  15, 1.0 / (double )(1 << 28))
    GETFLOATSIGN(eph._Crc,  15, 1.0 / (double )(1 <<  4))
    GETFLOATSIGN(eph._Crs,  15, 1.0 / (double )(1 <<  4))
    GETFLOATSIGN(eph._IDOT, 14, R2R_PI/(double)(1<<30)/(double)(1<<13))
    SKIPBITS(2)
    GETFLOATSIGN(eph._M0,   32, R2R_PI/(double)(1<<30)/(double)(1<< 1))
    GETBITS(i, 16)
    i <<= 4;
    eph._TOEsec = i;
    bncTime t;
    t.set(i * 1000);
    eph._TOEweek = t.gpsw();
    int numOfRollOvers = int(floor(t.gpsw()/1024.0));
    week += (numOfRollOvers * 1024);
    /* week from HOW, differs from TOC, TOE week, we use adapted value instead */
    if (eph._TOEweek > week + 1 || eph._TOEweek < week - 1) /* invalid week */
      return false;
    GETFLOAT(eph._e,            32, 1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETFLOAT(eph._sqrt_A,       32, 1.0 / (double )(1 << 19))
    if (eph._sqrt_A < 1000.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3) SQRT_A %4 m!")
           .arg(_staID).arg(1041,4).arg(eph._prn.toString().c_str())
           .arg(eph._sqrt_A,10,'F',3).toLatin1(), true));
#endif
      return false;
    }
    GETFLOATSIGN(eph._OMEGA0,   32, R2R_PI/(double)(1<<30)/(double)(1<< 1))
    GETFLOATSIGN(eph._omega,    32, R2R_PI/(double)(1<<30)/(double)(1<< 1))
    GETFLOATSIGN(eph._OMEGADOT, 22, R2R_PI/(double)(1<<30)/(double)(1<<11))
    GETFLOATSIGN(eph._i0,       32, R2R_PI/(double)(1<<30)/(double)(1<< 1))
    SKIPBITS(2)
    eph._TOT = 0.9999e9;
    eph._navType = t_eph::LNAV;

    emit newGPSEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeSBASEphemeris(unsigned char* data, int size) {
  bool decoded = false;

  if (size == 35) {
    t_ephSBAS eph;
    int i;
    uint64_t numbits = 0, bitfield = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(i, 6)
    eph._prn.set('S', 20 + i);
    GETBITS(eph._IODN, 8)
    GETBITS(i, 13)
    i <<= 4;
    eph._TOC.setTOD(i * 1000);
    GETBITS(i, 4)
    eph._ura = accuracyFromIndex(i, eph.type());
    GETFLOATSIGN(eph._x_pos, 30, 0.08)
    GETFLOATSIGN(eph._y_pos, 30, 0.08)
    GETFLOATSIGN(eph._z_pos, 25, 0.4)
    ColumnVector pos(3);
    pos(1) = eph._x_pos; pos(2) = eph._y_pos; pos(3) = eph._z_pos;
    if (pos.NormFrobenius() < 1.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3): zero position!")
           .arg(_staID).arg(1043,4).arg(eph._prn.toString().c_str()).toLatin1(), true));
#endif
      return false;
    }
    GETFLOATSIGN(eph._x_velocity, 17, 0.000625)
    GETFLOATSIGN(eph._y_velocity, 17, 0.000625)
    GETFLOATSIGN(eph._z_velocity, 18, 0.004)
    GETFLOATSIGN(eph._x_acceleration, 10, 0.0000125)
    GETFLOATSIGN(eph._y_acceleration, 10, 0.0000125)
    GETFLOATSIGN(eph._z_acceleration, 10, 0.0000625)
    GETFLOATSIGN(eph._agf0, 12, 1.0 / (1 << 30) / (1 << 1))
    GETFLOATSIGN(eph._agf1, 8, 1.0 / (1 << 30) / (1 << 10))

    eph._TOT = 0.9999E9;
    eph._health = 0;
    eph._navType = t_eph::SBASL1;

    emit newSBASEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeGalileoEphemeris(unsigned char* data, int size) {
  bool decoded = false;
  uint64_t numbits = 0, bitfield = 0;
  int i;

  data += 3; /* header */
  size -= 6; /* header + crc */
  GETBITS(i, 12)

  if ((i == 1046 && size == 61) || (i == 1045 && size == 60)) {
    t_ephGal eph;

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    eph._inav = (i == 1046);
    eph._fnav = (i == 1045);
    GETBITS(i, 6)
    eph._prn.set('E', i, eph._inav ? 1 : 0);

    GETBITS(eph._TOEweek, 12) //FIXME: roll-over after week 4095!!
    GETBITS(eph._IODnav, 10)
    GETBITS(i, 8)
    eph._SISA = accuracyFromIndex(i, eph.type());
    GETFLOATSIGN(eph._IDOT, 14, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETBITSFACTOR(i, 14, 60)
    eph._TOC.set(1024 + eph._TOEweek, i);
    GETFLOATSIGN(eph._clock_driftrate, 6, 1.0 / (double )(1 << 30) / (double )(1 << 29))
    GETFLOATSIGN(eph._clock_drift,    21, 1.0 / (double )(1 << 30) / (double )(1 << 16))
    GETFLOATSIGN(eph._clock_bias,     31, 1.0 / (double )(1 << 30) / (double )(1 << 4))
    GETFLOATSIGN(eph._Crs,            16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._Delta_n,        16, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._M0,             32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cuc,            16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._e,                  32, 1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETFLOATSIGN(eph._Cus,            16, 1.0 / (double )(1 << 29))
    GETFLOAT(eph._sqrt_A,             32, 1.0 / (double )(1 << 19))
    GETBITSFACTOR(eph._TOEsec, 14, 60)
    /* FIXME: overwrite value, copied from old code */
    eph._TOEsec = eph._TOC.gpssec();
    GETFLOATSIGN(eph._Cic,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._OMEGA0,   32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cis,      16, 1.0 / (double )(1 << 29))
    GETFLOATSIGN(eph._i0,       32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Crc,      16, 1.0 / (double )(1 << 5))
    GETFLOATSIGN(eph._omega,    32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._OMEGADOT, 24, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._BGD_1_5A, 10, 1.0 / (double )(1 << 30) / (double )(1 << 2))
    if (eph._inav) {
      /* set unused F/NAV values */
      eph._E5aHS = 0.0;
      eph._e5aDataInValid = false;

      GETFLOATSIGN(eph._BGD_1_5B, 10, 1.0 / (double )(1 << 30) / (double )(1 << 2))
      GETBITS(eph._E5bHS, 2)
      GETBITS(eph._e5bDataInValid, 1)
      GETBITS(eph._E1_bHS, 2)
      GETBITS(eph._e1DataInValid, 1)
      if (eph._E5bHS != eph._E1_bHS) {
#ifdef BNC_DEBUG_BCEP
        emit(newMessage(QString("%1: Block %2 (%3) SHS E5b %4 E1B %5: inconsistent health!")
             .arg(_staID).arg(1046,4).arg(eph._prn.toString().c_str())
             .arg(eph._E5bHS).arg(eph._E1_bHS).toLatin1(), true));
#endif
        return false;
      }
      if ((eph._BGD_1_5A == 0.0 && fabs(eph._BGD_1_5B) > 1e-9) ||
          (eph._BGD_1_5B == 0.0 && fabs(eph._BGD_1_5A) > 1e-9)) {
#ifdef BNC_DEBUG_BCEP
        emit(newMessage(QString("%1: Block %2 (%3) BGD_15a = %4 BGD_15b = %5: inconsistent BGD!")
             .arg(_staID).arg(1046,4).arg(eph._prn.toString().c_str())
             .arg(eph._BGD_1_5A,10,'E',3).arg(eph._BGD_1_5B,10,'E',3).toLatin1(), true));
#endif
        return false;
      }
      eph._navType = t_eph::INAF;
    }
    else {
      /* set unused I/NAV values */
      eph._BGD_1_5B = 0.0;
      eph._E5bHS = 0.0;
      eph._E1_bHS = 0.0;
      eph._e1DataInValid = false;
      eph._e5bDataInValid = false;

      GETBITS(eph._E5aHS, 2)
      GETBITS(eph._e5aDataInValid, 1)
      eph._navType = t_eph::FNAV;
    }
    eph._TOT = 0.9999e9;

    if (eph._sqrt_A < 1000.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3) SQRT_A %4 m!")
           .arg(_staID).arg(eph._inav? 1046 : 1045,4).arg(eph._prn.toString().c_str())
           .arg(eph._sqrt_A,10,'F',3).toLatin1(), true));
#endif
      return false;
    }

    emit newGalileoEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeBDSEphemeris(unsigned char* data, int size) {
  bool decoded = false;
  const double iMaxGEO = 10.0 / 180.0 * M_PI;

  if (size == 70) {
    t_ephBDS eph;
    int i;
    uint64_t numbits = 0, bitfield = 0;

    data += 3; /* header */
    size -= 6; /* header + crc */
    SKIPBITS(12)

    eph._receptDateTime = currentDateAndTimeGPS();
    eph._receptStaID = _staID;

    GETBITS(i, 6)
    eph._prn.set('C', i);

    GETBITS(eph._BDTweek, 13)
    GETBITS(i, 4)
    eph._URA = accuracyFromIndex(i, eph.type());
    GETFLOATSIGN(eph._IDOT, 14, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETBITS(eph._AODE, 5)
    GETBITS(i, 17)
    i <<= 3;
    eph._TOC.setBDS(eph._BDTweek, i);
    GETFLOATSIGN(eph._clock_driftrate, 11,  1.0 / (double )(1 << 30) / (double )(1 << 30) / (double )(1 << 6))
    GETFLOATSIGN(eph._clock_drift,     22,  1.0 / (double )(1 << 30) / (double )(1 << 20))
    GETFLOATSIGN(eph._clock_bias,      24,  1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETBITS(eph._AODC, 5)
    GETFLOATSIGN(eph._Crs,     18, 1.0 / (double )(1 << 6))
    GETFLOATSIGN(eph._Delta_n, 16, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._M0,      32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cuc,     18, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETFLOAT(eph._e,           32, 1.0 / (double )(1 << 30) / (double )(1 << 3))
    GETFLOATSIGN(eph._Cus,     18, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETFLOAT(eph._sqrt_A,      32, 1.0 / (double )(1 << 19))
    if (eph._sqrt_A < 1000.0) {
#ifdef BNC_DEBUG_BCEP
      emit(newMessage(QString("%1: Block %2 (%3) SQRT_A %4 m!")
           .arg(_staID).arg(1042,4).arg(eph._prn.toString().c_str())
           .arg(eph._sqrt_A,10,'F',3).toLatin1(), true));
#endif
      return false;
    }
    GETBITS(i, 17)
    i <<= 3;
    eph._TOEsec = i;
    eph._TOE.setBDS(eph._BDTweek, i);
    GETFLOATSIGN(eph._Cic,      18, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETFLOATSIGN(eph._OMEGA0,   32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Cis,      18, 1.0 / (double )(1 << 30) / (double )(1 << 1))
    GETFLOATSIGN(eph._i0,       32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._Crc,      18, 1.0 / (double )(1 << 6))
    GETFLOATSIGN(eph._omega,    32, R2R_PI/(double)(1<<30)/(double)(1<<1))
    GETFLOATSIGN(eph._OMEGADOT, 24, R2R_PI/(double)(1<<30)/(double)(1<<13))
    GETFLOATSIGN(eph._TGD1,     10, 0.0000000001)
    GETFLOATSIGN(eph._TGD2,     10, 0.0000000001)
    GETBITS(eph._SatH1, 1)

    eph._TOT = 0.9999E9;
    if (eph._i0 > iMaxGEO) {
      eph._navType = t_eph::D1;
    }
    else {
      eph._navType = t_eph::D2;
    }

    emit newBDSEph(eph);
    decoded = true;
  }
  return decoded;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeAntennaReceiver(unsigned char* data, int size) {
  char *antenna;
  char *antserialnum;
  char *receiver;
  char *recfirmware;
  char *recserialnum;
  int type;
  int antsernum = -1;
  int antnum = -1;
  int recnum = -1;
  int recsernum = -1;
  int recfirnum = -1;
  uint64_t numbits = 0, bitfield = 0;

  data += 3; /* header*/
  size -= 6; /* header + crc */

  GETBITS(type, 12)
  SKIPBITS(12) /* reference station ID */
  GETSTRING(antnum, antenna)
  if ((antnum > -1 && antnum < 265) &&
      (_antType.empty() || strncmp(_antType.back().descriptor, antenna, recnum) != 0)) {
    _antType.push_back(t_antInfo());
    memcpy(_antType.back().descriptor, antenna, antnum);
    _antType.back().descriptor[antnum] = 0;
  }
  SKIPBITS(8) /* antenna setup ID */
  if (type == 1008 || type == 1033 ) {
    GETSTRING(antsernum, antserialnum)
    if ((antsernum > -1 && antsernum < 265)) {
      memcpy(_antType.back().serialnumber, antserialnum, antsernum);
      _antType.back().serialnumber[antsernum] = 0;
    }
  }

  if (type == 1033) {
    GETSTRING(recnum, receiver)
    GETSTRING(recfirnum, recfirmware)
    GETSTRING(recsernum, recserialnum)
    if ((recnum > -1 && recnum < 265) &&
        (_recType.empty() || strncmp(_recType.back().descriptor, receiver, recnum) != 0)) {
      _recType.push_back(t_recInfo());
      memcpy(_recType.back().descriptor, receiver, recnum);
      _recType.back().descriptor[recnum] = 0;
      if (recfirnum > -1 && recfirnum < 265) {
        memcpy(_recType.back().firmware, recfirmware, recfirnum);
        _recType.back().firmware[recfirnum] = 0;
      }
      if (recsernum > -1 && recsernum < 265) {
        memcpy(_recType.back().serialnumber, recserialnum, recsernum);
        _recType.back().serialnumber[recsernum] = 0;
      }
    }
  }
  return true;
}

//
////////////////////////////////////////////////////////////////////////////
bool RTCM3Decoder::DecodeAntennaPosition(unsigned char* data, int size) {
  int type;
  uint64_t numbits = 0, bitfield = 0;
  double x, y, z;

  data += 3; /* header */
  size -= 6; /* header + crc */

  GETBITS(type, 12)
  _antList.push_back(t_antRefPoint());
  _antList.back().type = t_antRefPoint::ARP;
  SKIPBITS(22)
  GETBITSSIGN(x, 38)
  _antList.back().xx = x * 1e-4;
  SKIPBITS(2)
  GETBITSSIGN(y, 38)
  _antList.back().yy = y * 1e-4;
  SKIPBITS(2)
  GETBITSSIGN(z, 38)
  _antList.back().zz = z * 1e-4;
  if (type == 1006)
      {
    double h;
    GETBITS(h, 16)
    _antList.back().height = h * 1e-4;
    _antList.back().height_f = true;
  }
  _antList.back().message = type;

  return true;
}

//
////////////////////////////////////////////////////////////////////////////
t_irc RTCM3Decoder::Decode(char* buffer, int bufLen, vector<string>& errmsg) {
  bool decoded = false;

  errmsg.clear();

  while (bufLen && _MessageSize < sizeof(_Message)) {
    int l = sizeof(_Message) - _MessageSize;
    if (l > bufLen)
      l = bufLen;
    memcpy(_Message + _MessageSize, buffer, l);
    _MessageSize += l;
    bufLen -= l;
    buffer += l;
    int id;
    while ((id = GetMessage())) {
      /* reset station ID for file loading as it can change */
      if (_rawFile)
        _staID = _rawFile->staID();
      /* store the id into the list of loaded blocks */
      _typeList.push_back(id);

      /* SSR I+II data handled in another function, already pass the
       * extracted data block. That does no harm, as it anyway skip everything
       * else. */
      if ((id >= 1057 && id <= 1068) ||
          (id >= 1240 && id <= 1270) ||
          (id == 4076)) {
        if (!_coDecoders.contains(_staID.toLatin1())) {
          _coDecoders[_staID.toLatin1()] = new RTCM3coDecoder(_staID);
          if (id == 4076) {
            _coDecoders[_staID.toLatin1()]->initSsrFormatType(RTCM3coDecoder::IGSssr);
          }
          else {
            _coDecoders[_staID.toLatin1()]->initSsrFormatType(RTCM3coDecoder::RTCMssr);
          }
        }
        RTCM3coDecoder* coDecoder = _coDecoders[_staID.toLatin1()];
        if (coDecoder->Decode(reinterpret_cast<char *>(_Message), _BlockSize, errmsg) == success) {
          decoded = true;
        }
      }
      else if (id >= 1070 && id <= 1237) { /* MSM */
        if (DecodeRTCM3MSM(_Message, _BlockSize))
          decoded = true;
      }
      else {
        switch (id) {
          case 1001:
          case 1003:
            emit(newMessage(QString("%1: Block %2 contain partial data! Ignored!")
                 .arg(_staID).arg(id).toLatin1(), true));
            break; /* no use decoding partial data ATM, remove break when data can be used */
          case 1002:
          case 1004:
            if (DecodeRTCM3GPS(_Message, _BlockSize))
              decoded = true;
            break;
          case 1009:
          case 1011:
            emit(newMessage(QString("%1: Block %2 contain partial data! Ignored!")
                 .arg(_staID).arg(id).toLatin1(), true));
            break; /* no use decoding partial data ATM, remove break when data can be used */
          case 1010:
          case 1012:
            if (DecodeRTCM3GLONASS(_Message, _BlockSize))
              decoded = true;
            break;
          case 1019:
            if (DecodeGPSEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1020:
            if (DecodeGLONASSEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1043:
            if (DecodeSBASEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1044:
            if (DecodeQZSSEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1041:
            if (DecodeIRNSSEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1045:
          case 1046:
            if (DecodeGalileoEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1042:
            if (DecodeBDSEphemeris(_Message, _BlockSize))
              decoded = true;
            break;
          case 1007:
          case 1008:
          case 1033:
            DecodeAntennaReceiver(_Message, _BlockSize);
            break;
          case 1005:
          case 1006:
            DecodeAntennaPosition(_Message, _BlockSize);
            break;
        }
      }
    }
  }
  return decoded ? success : failure;
}
;

//
////////////////////////////////////////////////////////////////////////////
uint32_t RTCM3Decoder::CRC24(long size, const unsigned char *buf) {
  uint32_t crc = 0;
  int ii;
  while (size--) {
    crc ^= (*buf++) << (16);
    for (ii = 0; ii < 8; ii++) {
      crc <<= 1;
      if (crc & 0x1000000)
        crc ^= 0x01864cfb;
    }
  }
  return crc;
}

//
////////////////////////////////////////////////////////////////////////////
int RTCM3Decoder::GetMessage(void) {
  unsigned char *m, *e;
  int i;

  m = _Message + _SkipBytes;
  e = _Message + _MessageSize;
  _NeedBytes = _SkipBytes = 0;
  while (e - m >= 3) {
    if (m[0] == 0xD3) {
      _BlockSize = ((m[1] & 3) << 8) | m[2];
      if (e - m >= static_cast<int>(_BlockSize + 6)) {
        if (static_cast<uint32_t>((m[3 + _BlockSize] << 16)
            | (m[3 + _BlockSize + 1] << 8)
            | (m[3 + _BlockSize + 2])) == CRC24(_BlockSize + 3, m)) {
          _BlockSize += 6;
          _SkipBytes = _BlockSize;
          break;
        }
        else
          ++m;
      }
      else {
        _NeedBytes = _BlockSize;
        break;
      }
    }
    else
      ++m;
  }
  if (e - m < 3)
    _NeedBytes = 3;

  /* copy buffer to front */
  i = m - _Message;
  if (i && m < e)
    memmove(_Message, m, static_cast<size_t>(_MessageSize - i));
  _MessageSize -= i;

  return !_NeedBytes ? ((_Message[3] << 4) | (_Message[4] >> 4)) : 0;
}

// Time of Corrections
//////////////////////////////////////////////////////////////////////////////
int RTCM3Decoder::corrGPSEpochTime() const {
  return
      _coDecoders.size() > 0 ?
          _coDecoders.begin().value()->corrGPSEpochTime() : -1;
}
