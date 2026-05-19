#!/bin/bash
set -e

gstd --enable-http-protocol --http-address=0.0.0.0 --http-port=5000 &
GSTD_PID=$!
sleep 2

PIPELINE="v4l2src device=/dev/video0 \
! image/jpeg,width=1920,height=1080,framerate=30/1 \
! jpegdec \
! videoconvert \
! videoscale \
! capsfilter name=scaler caps=video/x-raw,width=${WIDTH:-1920},height=${HEIGHT:-1080} \
! x264enc name=encoder tune=zerolatency speed-preset=ultrafast bitrate=${BITRATE:-12000} key-int-max=30 \
! h264parse \
! rtspclientsink location=rtsp://mediamtx:8554/cam name=sink \
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
