// Part of BNC, a utility for retrieving decoding and
// converting GNSS data streams from NTRIP broadcasters.
//
// Copyright (C) 2015
// German Federal Agency for Cartography and Geodesy (BKG)
// http://www.bkg.bund.de
// Alberding GmbH
// http://www.alberding.eu
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

#ifndef BITS_H
#define BITS_H

#define LOADBITS(a) { \
  while((a) > numbits) { \
    if(!size--) return false; \
    bitfield = (bitfield<<8)|*(data++); \
    numbits += 8; \
  } \
}

/* extract bits from data stream
   b = variable to store result, a = number of bits */
#define GETBITS64(b, a) { \
  if(((a) > 56) && ((a)-56) > numbits) { \
    uint64_t x; \
    GETBITS(x, 56) \
    LOADBITS((a)-56) \
    b = ((x<<((a)-56)) | (bitfield<<(sizeof(bitfield)*8-numbits)) \
    >>(sizeof(bitfield)*8-((a)-56))); \
    numbits -= ((a)-56); \
  } \
  else { \
    GETBITS(b, a) \
  } \
}

/* extract bits from data stream
   b = variable to store result, a = number of bits */
#define GETBITS(b, a) { \
  LOADBITS(a) \
  b = (bitfield<<(64-numbits))>>(64-(a)); \
  numbits -= (a); \
}

/* extract bits from data stream
   b = variable to store result, a = number of bits */
#define GETBITSFACTOR(b, a, c) { \
  LOADBITS(a) \
  b = ((bitfield<<(sizeof(bitfield)*8-numbits))>>(sizeof(bitfield)*8-(a)))*(c); \
  numbits -= (a); \
}

/* extract floating value from data stream
   b = variable to store result, a = number of bits */
#define GETFLOAT(b, a, c) { \
  LOADBITS(a) \
  b = ((double)((bitfield<<(64-numbits))>>(64-(a))))*(c); \
  numbits -= (a); \
}

/* extract signed floating value from data stream
   b = variable to store result, a = number of bits */
#define GETFLOATSIGN(b, a, c) { \
  LOADBITS(a) \
  b = ((double)(((int64_t)(bitfield<<(64-numbits)))>>(64-(a))))*(c); \
  numbits -= (a); \
}

/* extract bits from data stream
   b = variable to store result, a = number of bits */
#define GETBITSSIGN(b, a) { \
  LOADBITS(a) \
  b = ((int64_t)(bitfield<<(64-numbits)))>>(64-(a)); \
  numbits -= (a); \
}

#define GETFLOATSIGNM(b, a, c) { \
  int l; \
  LOADBITS(a) \
  l = (bitfield<<(64-numbits))>>(64-1); \
  b = ((double)(((bitfield<<(64-(numbits-1))))>>(64-(a-1))))*(c); \
  numbits -= (a); \
  if(l) b *= -1.0; \
}

#define SKIPBITS(b) { LOADBITS(b) numbits -= (b); }

/* extract byte-aligned byte from data stream,
   b = variable to store size, s = variable to store string pointer */
#define GETSTRING(b, s) { \
  b = *(data++); \
  s = (char *) data; \
  data += b; \
  size -= b+1; \
}

#endif /* BITS_H */
