#! /bin/bash

cd header
testgo.sh

cd ../message1005
testgo.sh

cd ../utils
testgo.sh

cd ../msm4/satellite
testgo.sh

cd ../signal
testgo.sh

cd ../message
testgo.sh

cd ../../msm7/satellite
testgo.sh

cd ../signal
testgo.sh

cd ../message
testgo.sh

# rtcm
cd ../..
testgo.sh

