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

#ifndef RTCM3CODECODER_H
#define RTCM3CODECODER_H

#include <fstream>
#include <QByteArray>
#include <QList>
#include <QMap>
#include "GPSDecoder.h"
#include "../RTCM3/clock_and_orbit/clock_orbit_igs.h"
#include "../RTCM3/clock_and_orbit/clock_orbit_rtcm.h"

  class RTCM3coDecoder : public QObject, public GPSDecoder {
Q_OBJECT
 public:
  enum e_type {RTCMssr, IGSssr};
  RTCM3coDecoder(const QString& staID);
  virtual ~RTCM3coDecoder();
  virtual t_irc Decode(char* buffer, int bufLen, std::vector<std::string>& errmsg);
  virtual int corrGPSEpochTime() const {return int(_lastTime.gpssec());}
  virtual void initSsrFormatType(const e_type type) {
    _type = type;
    if      (_type == RTCMssr) {
      _ssrCorr = new SsrCorrRtcm();
    }
    else {
      _ssrCorr = new SsrCorrIgs();
    }
  }

 signals:
  void newOrbCorrections(QList<t_orbCorr>);
  void newClkCorrections(QList<t_clkCorr>);
  void newCodeBiases(QList<t_satCodeBias>);
  void newPhaseBiases(QList<t_satPhaseBias>);
  void newTec(t_vTec);
  void newMessage(QByteArray msg, bool showOnScreen);
  void providerIDChanged(QString staID);

 private:
  void reset();
  void setEpochTime();
  void sendResults();
  void reopen();
  void checkProviderID();

  std::ofstream*                        _out;
  QString                               _staID;
  QString                               _fileNameSkl;
  QString                               _fileName;
  QByteArray                            _buffer;
  SsrCorr::ClockOrbit                   _clkOrb;
  SsrCorr::CodeBias                     _codeBias;
  SsrCorr::PhaseBias                    _phaseBias;
  SsrCorr::VTEC                         _vTEC;
  int                                   _providerID[3];
  e_type                                _type;
  bncTime                               _lastTime;
  SsrCorr*                              _ssrCorr;
  QMap<t_prn, unsigned int>             _IODs;
  QMap<bncTime, QList<t_orbCorr> >      _orbCorrections;
  QMap<bncTime, QList<t_clkCorr> >      _clkCorrections;
  QMap<t_prn, t_clkCorr>                _lastClkCorrections;
  QMap<bncTime, QList<t_satCodeBias> >  _codeBiases;
  QMap<bncTime, QList<t_satPhaseBias> > _phaseBiases;
  QMap<bncTime, t_vTec>                 _vTecMap;
};

#endif
