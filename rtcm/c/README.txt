These files are from the RTKLIB package, which is written in C.
I used them to get useful clues for decoding RTCM messages.
For example:

* rtcm3.c line  346 decode_type1005() decodes a type 1005 (base position) message.
* rtcm3.c line 1747 decode_msm_head() decodes an MSM message header
* rtcm3.c line 2004 decode_msm7() decodes an MSM7 message

If you need to extend my rtcm package to handle other message types,
look at the corresponding functions in this C code.
