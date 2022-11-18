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

#ifndef BNCUTILS_H
#define BNCUTILS_H

#include <vector>

#include <QString>
#include <QDateTime>

#include <newmat.h>
#include <bncconst.h>
#include <ephemeris.h>

class t_eph;

const double RHO_DEG = 180.0 / M_PI;
const double RHO_SEC = 3600.0 * 180.0 / M_PI;
const double MJD_J2000 = 51544.5;

static const QVector<int> ssrUpdateInt = QVector<int>()  << 1 << 2 << 5 << 10 << 15 << 30
                                                         << 60 << 120 << 240 << 300 << 600
                                                         << 900 << 1800 << 3600 << 7200
                                                         << 10800;

void         expandEnvVar(QString& str);

/**
 * Return GPS leap seconds for a given UTC time
 * @param year 4 digit year
 * @param month month in year (1-12)
 * @param day day in month (1-31)
 * @return number of leap seconds since 6.1.1980
 */
int          gnumleap(int year, int month, int day);

/**
 * Convert Moscow time into GPS or UTC. Note that parts of a second are not preserved
 * and must be handled separately.
 * @param week GPS week number (must be prefilled, contains fixed value afterwards)
 * @param secOfWeek seconds in GPS week (must be prefilled, contains fixed value afterwards)
 * @param mSecOfWeek milli seconds in GLONASS time
 * @param fixnumleap when <code>true</code> then result is UTC time, otherwise it is GPS
 * @return does not return a value, but updates first two arguments
 */
void         updatetime(int *week, int *secOfWeek, int mSecOfWeek, bool fixnumleap);

QDateTime    dateAndTimeFromGPSweek(int GPSWeek, double GPSWeeks);

void         currentGPSWeeks(int& week, double& sec);

QDateTime    currentDateAndTimeGPS();

bool         checkForWrongObsEpoch(bncTime obsEpoch);

bool         outDatedBcep(const t_eph *eph);

QByteArray   ggaString(const QByteArray& latitude, const QByteArray& longitude,
                       const QByteArray& height, const QString& ggaType);

void         RSW_to_XYZ(const ColumnVector& rr, const ColumnVector& vv,
                        const ColumnVector& rsw, ColumnVector& xyz);

void         XYZ_to_RSW(const ColumnVector& rr, const ColumnVector& vv,
                        const ColumnVector& xyz, ColumnVector& rsw);

t_irc        xyz2ell(const double* XYZ, double* Ell);

t_irc        xyz2geoc(const double* XYZ, double* Geoc);

void         xyz2neu(const double* Ell, const double* xyz, double* neu);

void         neu2xyz(const double* Ell, const double* neu, double* xyz);

void         jacobiXYZ_NEU(const double* Ell, Matrix& jacobi);

void         jacobiEll_XYZ(const double* Ell, Matrix& jacobi);

void         covariXYZ_NEU(const SymmetricMatrix& Qxyz, const double* Ell,
                           SymmetricMatrix& Qneu);

void         covariNEU_XYZ(const SymmetricMatrix& Qneu, const double* Ell,
                           SymmetricMatrix& Qxyz);

double       Frac(double x);

double       Modulo(double x, double y);

double       nint(double val);

ColumnVector rungeKutta4(double xi, const ColumnVector& yi, double dx, double* acc,
                         ColumnVector (*der)(double x, const ColumnVector& y, double* acc));

void         GPSweekFromDateAndTime(const QDateTime& dateTime, int& GPSWeek, double& GPSWeeks);

void         GPSweekFromYMDhms(int year, int month, int day, int hour, int min, double sec,
                               int& GPSWeek, double& GPSWeeks);

void         mjdFromDateAndTime(const QDateTime& dateTime, int& mjd, double& dayfrac);

bool         findInVector(const std::vector<QString>& vv, const QString& str);

int          readInt(const QString& str, int pos, int len, int& value);

int          readDbl(const QString& str, int pos, int len, double& value);

void         topos(double xRec, double yRec, double zRec, double xSat, double ySat, double zSat,
                   double& rho, double& eleSat, double& azSat);

void         deg2DMS(double decDeg, int& deg, int& min, double& sec);

QString      fortranFormat(double value, int width, int prec);

void         kalman(const Matrix& AA, const ColumnVector& ll, const DiagonalMatrix& PP,
                    SymmetricMatrix& QQ, ColumnVector& xx);

double       djul(long j1, long m1, double tt);

double       gpjd(double second, int nweek) ;

void         jdgp(double tjul, double & second, long & nweek);

void         jmt (double djul, long& jj, long& mm, double& dd);

void         stripWhiteSpace(std::string& str);

double       accuracyFromIndex(int index, t_eph::e_type type);

int          indexFromAccuracy(double accuracy, t_eph::e_type type);

double       fitIntervalFromFlag(int flag, double iodc, t_eph::e_type type);

double       associatedLegendreFunction(int n, int m, double t);

double       factorial(int n);

/** Convert RTCM3 lock-time indicator to lock time in seconds
* depending on input message format. Returns -1 if format is
* unknown or indicator is invalid
*/
double       lti2sec(int type, int lti);

// CRC24Q checksum calculation function (only full bytes supported).
///////////////////////////////////////////////////////////////////
unsigned long CRC24(long size, const unsigned char *buf);

// Extracts k bits from position p and returns the extracted value as integer
///////////////////////////////////////////////////////////////////
int bitExtracted(int number, int k, int p);

// RTCM3 GPS EPH encoding
//////////////////////////////////////////////////////////
#define GPSTOINT(type, value) static_cast<type>(round(value))

#define GPSADDBITS(a, b) {bitbuffer = (bitbuffer<<(a)) \
                       |(GPSTOINT(long long,b)&((1ULL<<a)-1)); \
                       numbits += (a); \
                       while(numbits >= 8) { \
                       buffer[size++] = bitbuffer>>(numbits-8);numbits -= 8;}}

#define GPSADDBITSFLOAT(a,b,c) {long long i = GPSTOINT(long long,(b)/(c)); \
                             GPSADDBITS(a,i)};

// RTCM3 GLONASS EPH encoding
//////////////////////////////////////////////////////////
#define GLONASSTOINT(type, value) static_cast<type>(round(value))
#define GLONASSADDBITS(a, b) {bitbuffer = (bitbuffer<<(a)) \
                       |(GLONASSTOINT(long long,b)&((1ULL<<(a))-1)); \
                       numbits += (a); \
                       while(numbits >= 8) { \
                       buffer[size++] = bitbuffer>>(numbits-8);numbits -= 8;}}
#define GLONASSADDBITSFLOATM(a,b,c) {int s; long long i; \
                       if(b < 0.0) \
                       { \
                         s = 1; \
                         i = GLONASSTOINT(long long,(-b)/(c)); \
                         if(!i) s = 0; \
                       } \
                       else \
                       { \
                         s = 0; \
                         i = GLONASSTOINT(long long,(b)/(c)); \
                       } \
                       GLONASSADDBITS(1,s) \
                       GLONASSADDBITS(a-1,i)}

// RTCM3 Galileo EPH encoding
//////////////////////////////////////////////////////////
#define GALILEOTOINT(type, value) static_cast<type>(round(value))
#define GALILEOADDBITS(a, b) {bitbuffer = (bitbuffer<<(a)) \
                       |(GALILEOTOINT(long long,b)&((1LL<<a)-1)); \
                       numbits += (a); \
                       while(numbits >= 8) { \
                       buffer[size++] = bitbuffer>>(numbits-8);numbits -= 8;}}
#define GALILEOADDBITSFLOAT(a,b,c) {long long i = GALILEOTOINT(long long,(b)/(c)); \
                             GALILEOADDBITS(a,i)};

// RTCM3 SBAS EPH encoding
//////////////////////////////////////////////////////////
#define SBASTOINT(type, value) static_cast<type>(round(value))
#define SBASADDBITS(a, b) {bitbuffer = (bitbuffer<<(a)) \
                       |(SBASTOINT(long long,b)&((1ULL<<a)-1)); \
                       numbits += (a); \
                       while(numbits >= 8) { \
                       buffer[size++] = bitbuffer>>(numbits-8);numbits -= 8;}}
#define SBASADDBITSFLOAT(a,b,c) {long long i = SBASTOINT(long long,(b)/(c)); \
                             SBASADDBITS(a,i)};

// RTCM3 BDS EPH encoding
//////////////////////////////////////////////////////////
#define BDSTOINT(type, value) static_cast<type>(round(value))
#define BDSADDBITS(a, b) {bitbuffer = (bitbuffer<<(a)) \
                       |(BDSTOINT(long long,b)&((1ULL<<a)-1)); \
                       numbits += (a); \
                       while(numbits >= 8) { \
                       buffer[size++] = bitbuffer>>(numbits-8);numbits -= 8;}}
#define BDSADDBITSFLOAT(a,b,c) {long long i = BDSTOINT(long long,(b)/(c)); \
                             BDSADDBITS(a,i)};

#endif
