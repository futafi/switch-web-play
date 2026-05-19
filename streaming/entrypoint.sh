#!/bin/bash
set -e

gstd --enable-http-protocol --http-address=0.0.0.0 --http-port=5000 &
GSTD_PID=$!
sleep 2

PIPELINE="v4l2src device=/dev/video0 \
! capsfilter name=capture caps=image/jpeg,width=1920,height=1080,framerate=30/1 \
! jpegdec \
! videoconvert \
! videoscale \
! capsfilter name=scaler caps=video/x-raw,width=${WIDTH:-1920},height=${HEIGHT:-1080} \
! vaapih265enc name=encoder tune=low-power rate-control=cbr bitrate=${BITRATE:-12000} keyframe-period=30 \
! h265parse config-interval=1 \
! rtspclientsink location=rtsp://${RTSP_HOST:-mediamtx}:8554/cam name=sink \
alsasrc device=${AUDIO_DEVICE} \
! audioconvert \
! audioresample \
! opusenc bitrate=128000 frame-size=10 \
! sink."

curl -sf -X POST -G \
  --data-urlencode "name=cam" \
  --data-urlencode "description=$PIPELINE" \
  http://127.0.0.1:5000/pipelines

curl -sf -X PUT 'http://127.0.0.1:5000/pipelines/cam/state?name=playing'

echo "gstd pipeline started"

wait $GSTD_PID
