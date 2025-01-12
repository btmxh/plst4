#!/bin/sh

ffmpeg -f lavfi -i testsrc=size=640x360:rate=15 -t 00:00:05 -pix_fmt yuv420p -c:v libx264 -crf 35 \
-metadata title="5 second 360p test video" -metadata artist="Artist 101" -y 5s360p.mp4

ffmpeg -f lavfi -i testsrc=size=640x360:rate=15 -t 00:00:10 -pix_fmt yuv420p -c:v libx264 -crf 35 \
-metadata title="10 second 360p test video" -metadata artist="Artist 101" -y 10s360p.mp4

ffmpeg -f lavfi -i testsrc=size=640x360:rate=15 -t 00:01:00 -pix_fmt yuv420p -c:v libx264 -crf 35 \
-metadata title="1 minute 360p test video" -metadata artist="Artist 101" -y 1m360p.mp4
