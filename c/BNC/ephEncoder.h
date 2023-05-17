#ifndef EPHENCODER_H
#define EPHENCODER_H

#include "ephemeris.h"
#include "bncutils.h"

class t_ephEncoder {
 public:
  static int RTCM3(const t_ephGPS&  eph, unsigned char *);
  static int RTCM3(const t_ephGlo&  eph, unsigned char *);
  static int RTCM3(const t_ephGal&  eph, unsigned char *);
  static int RTCM3(const t_ephSBAS& eph, unsigned char *);
  static int RTCM3(const t_ephBDS&  eph, unsigned char *);
};

#endif
