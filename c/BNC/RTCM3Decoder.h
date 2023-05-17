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

#ifndef RTCM3DECODER_H
#define RTCM3DECODER_H

#include <QtCore>
#include <map>

#include <stdint.h>
#include "GPSDecoder.h"
#include "RTCM3coDecoder.h"
#include "bncrawfile.h"
#include "ephemeris.h"

class RTCM3Decoder : public QObject, public GPSDecoder {
Q_OBJECT
 public:
  RTCM3Decoder(const QString& staID, bncRawFile* rawFile);
  virtual ~RTCM3Decoder();
  virtual t_irc Decode(char* buffer, int bufLen, std::vector<std::string>& errmsg);
  virtual int corrGPSEpochTime() const;
  /**
   * CRC24Q checksum calculation function (only full bytes supported).
   * @param size Size of the passed data
   * @param buf Data buffer containing the data to checksum
   * @return the CRC24Q checksum of the data
   */
  static uint32_t CRC24(long size, const unsigned char *buf);

 signals:
  void newMessage(QByteArray msg,bool showOnScreen);
  void newGPSEph(t_ephGPS eph);
  void newGlonassEph(t_ephGlo eph);
  void newSBASEph(t_ephSBAS eph);
  void newGalileoEph(t_ephGal eph);
  void newBDSEph(t_ephBDS eph);

 private:
  /**
   * Extract a RTCM3 message. Data is passed in the follow fields:<br>
   * {@link _Message}: contains the message bytes<br>
   * {@link _MessageSize}: contains to current amount of bytes in the buffer<br>
   * {@link _SkipBytes}: amount of bytes to skip at the beginning of the buffer
   *
   * The functions sets following variables:<br>
   * {@link _NeedBytes}: Minimum number of bytes needed on next call<br>
   * {@link _SkipBytes}: internal, Bytes to skip before next call (usually the amount of
   *   found bytes)<br>
   * {@link _MessageSize}: Updated size after processed bytes have been removed from buffer
   * @return message number when message found, 0 otherwise
   */
  int GetMessage(void);
  /**
   * Extract data from old 1001-1004 RTCM3 messages.
   * @param buffer the buffer containing an 1001-1004 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block is finished and transfered into
   * {@link GPSDecoder::_obsList} variable
   * @see DecodeRTCM3GLONASS()
   * @see DecodeRTCM3MSM()
   */
  bool DecodeRTCM3GPS(unsigned char* buffer, int bufLen);
  /**
   * Extract data from old 1009-1012 RTCM3 messages.
   * @param buffer the buffer containing an 1009-1012 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block is finished and transfered into
   * {@link GPSDecoder::_obsList} variable
   * @see DecodeRTCM3GPS()
   * @see DecodeRTCM3MSM()
   */
  bool DecodeRTCM3GLONASS(unsigned char* buffer, int bufLen);
  /**
   * Extract data from MSM 1070-1237 RTCM3 messages.
   * @param buffer the buffer containing an 1070-1237 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block is finished and transfered into
   * {@link GPSDecoder::_obsList} variable
   * @see DecodeRTCM3GPS()
   * @see DecodeRTCM3GLONASS()
   */
  bool DecodeRTCM3MSM(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 1019 RTCM3 messages.
   * @param buffer the buffer containing an 1019 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeGPSEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 1020 RTCM3 messages.
   * @param buffer the buffer containing an 1020 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeGLONASSEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 1043 RTCM3 messages.
   * @param buffer the buffer containing an 1043 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeSBASEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 1044 RTCM3 messages.
   * @param buffer the buffer containing an 1044 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeQZSSEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 29 (allocated for testing) RTCM3 messages.
   * @param buffer the buffer containing an 29 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeIRNSSEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from 1045 and 1046 RTCM3 messages.
   * @param buffer the buffer containing an 1045 and 1046 RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeGalileoEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract ephemeris data from BDS RTCM3 messages.
   * @param buffer the buffer containing an BDS RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeBDSEphemeris(unsigned char* buffer, int bufLen);
  /**
   * Extract antenna type from 1007, 1008 or 1033 RTCM3 messages
   * and extract receiver type from 1033 RTCM3 messages
   * @param buffer the buffer containing an antenna (and receiver) RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeAntennaReceiver(unsigned char* buffer, int bufLen);
  /**
   * Extract antenna type from 1005 or 1006 RTCM3 messages.
   * @param buffer the buffer containing an antenna RTCM block
   * @param bufLen the length of the buffer (the message length including header+crc)
   * @return <code>true</code> when data block was decodable
   */
  bool DecodeAntennaPosition(unsigned char* buffer, int bufLen);

  /** Current station description, dynamic in case of raw input file handling */
  QString                _staID;
  /** Raw input file for post processing, required to extract station ID */
  bncRawFile*            _rawFile;

  /** List of decoders for Clock and Orbit data */
  QMap<QByteArray, RTCM3coDecoder*> _coDecoders;

  /** Message buffer for input parsing */
  unsigned char _Message[2048];
  /** Current size of the message buffer */
  size_t _MessageSize;
  /** Minimum bytes required to have success during next {@link GetMessage()} call */
  size_t _NeedBytes;
  /** Bytes to skip in next {@link GetMessage()} call, intrnal to that function */
  size_t _SkipBytes;
  /** Size of the current RTCM3 block beginning at buffer start after a successful
   *  {@link GetMessage()} call
   */
  size_t _BlockSize;

  /**
   * Current observation epoch. Used to link together blocks in one epoch.
   */
  bncTime _CurrentTime;
  /** Current observation data block list, Filled by {@link DecodeRTCM3GPS()},
   * {@link DecodeRTCM3GLONASS()} and {@link DecodeRTCM3MSM()} functions.
   */
  QList<t_satObs> _CurrentObsList;
};

#endif

