These files are from the RTKLIB package, which is written in C.

You can download the latest version from https://github.com/tomojitakasu/RTKLIB.git.
This extract is from version 2.4.3 b34 committed 2020/12/29.
See https://rtklib.com/.

I used this source code to get useful clues for decoding RTCM messages.
For example:

* rtcm3.c decode_type1005() decodes a type 1005 (base position) message.
* rtcm3.c decode_msm_head() decodes an MSM message header
* rtcm3.c decode_msm7() decodes an MSM7 message

If you need to extend my rtcm package to handle other message types,
look at the corresponding functions in this C code.
