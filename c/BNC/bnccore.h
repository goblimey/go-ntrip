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

#ifndef BNCAPP_H
#define BNCAPP_H

#include "bnctime.h"
#include "bnccaster.h"
#include "bncrawfile.h"
#include "bncephuser.h"

class bncComb;
class bncTableItem;
namespace BNC_PPP {
  class t_pppMain;
}

class t_bncCore : public QObject {
Q_OBJECT
friend class bncSettings;

 public:
  enum e_mode {interactive, nonInteractive, batchPostProcessing};
  t_bncCore();
  ~t_bncCore();
  static t_bncCore* instance();
  e_mode            mode() const {return _mode;}
  void              setGUIenabled(bool GUIenabled) {_GUIenabled = GUIenabled;}
  void              setMode(e_mode mode) {_mode = mode;}
  void              setPortEph(int port);
  void              setPortCorr(int port);
  void              setCaster(bncCaster* caster) {_caster = caster;}
  const bncCaster*  caster() const {return _caster;}
  bool              dateAndTimeGPSSet() const;
  QDateTime         dateAndTimeGPS() const;
  void              setDateAndTimeGPS(QDateTime dateTime);
  void              setConfFileName(const QString& confFileName);
  QString           confFileName() const {return _confFileName;}
  void              writeRawData(const QByteArray& data, const QByteArray& staID,
                                 const QByteArray& format);
  void             initCombination();
  void             stopCombination();
  const QString&   pgmName() {return _pgmName;}
  const QString&   userName() {return _userName;}
  QWidget*         mainWindow() const {return _mainWindow;};
  void             setMainWindow(QWidget* mainWindow){_mainWindow = mainWindow;}
  bool             GUIenabled() const {return _GUIenabled;}
  void             startPPP();
  void             stopPPP();
  int              sigintReceived;

  QMap<int, bncTableItem*> _uploadTableItems;
  QMap<int, bncTableItem*> _uploadEphTableItems;

 public slots:
  void slotMessage(QByteArray msg, bool showOnScreen);
  void slotNewGPSEph(t_ephGPS);
  void slotNewGlonassEph(t_ephGlo);
  void slotNewGalileoEph(t_ephGal);
  void slotNewSBASEph(t_ephSBAS);
  void slotNewBDSEph(t_ephBDS);
  void slotNewOrbCorrections(QList<t_orbCorr>);
  void slotNewClkCorrections(QList<t_clkCorr>);
  void slotNewCodeBiases(QList<t_satCodeBias>);
  void slotNewPhaseBiases(QList<t_satPhaseBias>);
  void slotNewTec(t_vTec);
  void slotQuit();

 signals:
  void newMessage(QByteArray msg, bool showOnScreen);
  void newGPSEph(t_ephGPS eph);
  void newGlonassEph(t_ephGlo eph);
  void newSBASEph(t_ephSBAS eph);
  void newGalileoEph(t_ephGal eph);
  void newBDSEph(t_ephBDS eph);
  void newOrbCorrections(QList<t_orbCorr>);
  void newClkCorrections(QList<t_clkCorr>);
  void newCodeBiases(QList<t_satCodeBias>);
  void newPhaseBiases(QList<t_satPhaseBias>);
  void newTec(t_vTec);
  void providerIDChanged(QString);
  void newPosition(QByteArray staID, bncTime time, QVector<double> xx);
  void newNMEAstr(QByteArray staID, QByteArray str);
  void progressRnxPPP(int);
  void finishedRnxPPP();
  void mapSpeedSliderChanged(int);
  void stopRinexPPP();

 private slots:
  void slotNewConnectionEph();
  void slotNewConnectionCorr();

 private:
  t_irc checkPrintEph(t_eph* eph);
  void  printEphHeader();
  void  printEph(const t_eph& eph, bool printFile);
  void  printOutputEph(bool printFile, QTextStream* stream,
                       const QString& strV2, const QString& strV3,
                       const QString& strV4);
  void  messagePrivate(const QByteArray& msg);

  QSettings::SettingsMap _settings;
  QFile*                 _logFile;
  QTextStream*           _logStream;
  int                    _logFileFlag;
  QMutex                 _mutex;
  QMutex                 _mutexMessage;
  QString                _ephPath;
  QString                _ephFileNameGPS;
  int                    _rinexVers;
  QFile*                 _ephFileGPS;
  QTextStream*           _ephStreamGPS;
  QFile*                 _ephFileGlonass;
  QTextStream*           _ephStreamGlonass;
  QFile*                 _ephFileGalileo;
  QTextStream*           _ephStreamGalileo;
  QFile*                 _ephFileSBAS;
  QTextStream*           _ephStreamSBAS;
  QString                _userName;
  QString                _pgmName;
  int                    _portEph;
  QTcpServer*            _serverEph;
  QList<QTcpSocket*>*    _socketsEph;
  int                    _portCorr;
  QTcpServer*            _serverCorr;
  QList<QTcpSocket*>*    _socketsCorr;
  bncCaster*             _caster;
  QString                _confFileName;
  QDate                  _fileDate;
  bncRawFile*            _rawFile;
  bncComb*               _bncComb;
  e_mode                 _mode;
  QWidget*               _mainWindow;
  bool                   _GUIenabled;
  QDateTime*             _dateAndTimeGPS;
  mutable QMutex         _mutexDateAndTimeGPS;
  BNC_PPP::t_pppMain*    _pppMain;
  bncEphUser             _ephUser;
};

#define BNC_CORE (t_bncCore::instance())

#endif
