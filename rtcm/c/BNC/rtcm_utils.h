#ifndef RTCM_UTILS_H
#define RTCM_UTILS_H

class t_eph;

const double c_light  = 299792458.0;
const double FRQ_L1   = 1575420000.0;
const double FRQ_L2   = 1227600000.0;
const double LAMBDA_1 = c_light / FRQ_L1;
const double LAMBDA_2 = c_light / FRQ_L2;
const double ZEROVALUE = 1e-100;

void resolveEpoch (double secsHour,
		   int  refWeek,   double  refSecs,  
		   int& epochWeek, double& epochSecs);

int cmpRho(const t_eph* eph,
	   double stax, double stay, double staz,
	   int GPSWeek, double GPSWeeks,
	   double& rho, int& GPSWeek_tot, double& GPSWeeks_tot,
	   double& xSat, double& ySat, double& zSat, double& clkSat);

#endif
