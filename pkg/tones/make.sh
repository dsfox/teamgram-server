#!/bin/sh
# Makes the three notification sounds ice9 hands out, from nothing but ffmpeg's
# own oscillators: no third-party audio. Mono, 22050 Hz, 48 kbps mp3 - the
# iOS extension names a cloud sound <id>.mp3, and a few kilobytes each keeps
# the server binary small (#282). Run from this directory; commit the files.
set -eu
cd "$(dirname "$0")"

tone() {
	ffmpeg -hide_banner -loglevel error -y -f lavfi -i "aevalsrc=$2:s=22050:d=$3" \
		-ac 1 -ar 22050 -c:a libmp3lame -b:a 48k -write_xing 0 -id3v2_version 0 "$1"
}

# One soft high blip with a quick decay.
tone drop.mp3 '0.55*sin(2*PI*1046.5*t)*exp(-9*t)+0.2*sin(2*PI*2093*t)*exp(-14*t)' 0.45
# Two notes, a fifth apart, the second starting as the first fades.
tone chime.mp3 '0.5*sin(2*PI*1046.5*t)*exp(-6*t)*lt(t\,0.4)+0.5*sin(2*PI*1568*(t-0.28))*exp(-6*(t-0.28))*gte(t\,0.28)' 0.9
# Two short low taps.
tone knock.mp3 '0.8*sin(2*PI*180*t)*exp(-40*t)+0.8*sin(2*PI*180*(t-0.22))*exp(-40*(t-0.22))*gte(t\,0.22)' 0.55

ls -l drop.mp3 chime.mp3 knock.mp3
