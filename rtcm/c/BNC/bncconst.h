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

#include <string>

#ifndef BNCCONST_H
#define BNCCONST_H

enum t_irc {failure = -1, success, fatal}; // return code

class t_frequency {
 public:
  enum type {dummy = 0,
                        // GPS
                        G1, // L1 / 1575.42
                        G2, // L2 / 1227.60
                        G5, // L5 / 1176.45
                        // GLONASS
                        R1, // G1  / 1602 + k * 9/16 (k = -7 .. +12)
                        R4, // G1a / 1600.995
                        R2, // G2  / 1246 + k * 7/16 (k = -7 .. +12)
                        R6, // G2a / 1248.06
                        R3, // G3  / 1202.025
                        // Galileo
                        E1, // E1  / 1575.42
                        E5, // E5a / 1176.45
                        E7, // E5b / 1207.140
                        E8, // E5(E5a+E5b) / 1191.795
                        E6, // E6  / 1278.75
                        // QZSS
                        J1, // L1 / 1575.42
                        J2, // L2 / 1227.60
                        J5, // L5 / 1176.45
                        J6, // L6 / 1278.75
                        // BDS
                        C2, // B1 / 1561.098 (BDS 2/3 signals)
                        C1, // B1C, B1A / 1575.42 (BDS-3 signals)
                        C5, // B2a / 1176.45 (BDS-3 signals)
                        C7, // B2, B2b / 1207.14 (BDS-2 signals)
                        C8, // B2(B2a+B2b) / 1191.795 (BDS-3 signals)
                        C6, // B3,B3A / 1268.52
                        // IRNSS
                        I5, // L5 / 1176.45
                        I9, // S  / 2492.028
                        // SBAS
                        S1, // L1 / 1575.42
                        S5, // L5 / 1176.45
             max};

  static std::string toString(type tt) {
    // GPS
    if      (tt == G1) return "G1";
    else if (tt == G2) return "G2";
    else if (tt == G5) return "G5";
    // GLONASS
    else if (tt == R1) return "R1";
    else if (tt == R4) return "R4";
    else if (tt == R2) return "R2";
    else if (tt == R6) return "R6";
    else if (tt == R3) return "R3";
    // Galileo
    else if (tt == E1) return "E1";
    else if (tt == E5) return "E5";
    else if (tt == E6) return "E6";
    else if (tt == E7) return "E7";
    else if (tt == E8) return "E8";
    // QZSS
    else if (tt == J1) return "J1";
    else if (tt == J2) return "J2";
    else if (tt == J5) return "J5";
    else if (tt == J6) return "J6";
    // BDS
    else if (tt == C2) return "C2";
    else if (tt == C1) return "C1";
    else if (tt == C5) return "C5";
    else if (tt == C7) return "C7";
    else if (tt == C8) return "C8";
    else if (tt == C6) return "C6";
    // IRNSS
    else if (tt == I5) return "I5";
    else if (tt == I9) return "I9";
    // SBAS
    else if (tt == S1) return "S1";
    else if (tt == S5) return "S5";
    return std::string();
  }
  static enum type toInt(std::string s) {
    // GPS
    if      (s == "G1") return G1;
    else if (s == "G2") return G2;
    else if (s == "G5") return G5;
    // GLONASS
    else if (s == "R1") return R1;
    else if (s == "R2") return R2;
    else if (s == "R3") return R3;
    // Galileo
    else if (s == "E1") return E1;
    else if (s == "E5") return E5;
    else if (s == "E6") return E6;
    else if (s == "E7") return E7;
    else if (s == "E8") return E8;
    // QZSS
    else if (s == "J1") return J1;
    else if (s == "J2") return J2;
    else if (s == "J5") return J5;
    else if (s == "J6") return J6;
    // BDS
    else if (s == "C2") return C2;
    else if (s == "C1") return C1;
    else if (s == "C5") return C5;
    else if (s == "C7") return C7;
    else if (s == "C8") return C8;
    else if (s == "C6") return C6;
    // IRNSS
    else if (s == "I5") return I5;
    else if (s == "I9") return I9;
    // SBAS
    else if (s == "S1") return S1;
    else if (s == "S5") return S5;
    return type();
  }
};

class t_CST {
 public:
  static double freq(t_frequency::type fType, int slotNum);
  static double lambda(t_frequency::type fType, int slotNum);

  static const double c;
  static const double omega;
  static const double aell;
  static const double fInv;
  static const double rgeoc;
};


#endif
