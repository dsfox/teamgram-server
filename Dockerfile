FROM golang:1.23.0 AS builder
WORKDIR /app
COPY . .
# The generated protocol package is the heaviest in the build: the compiler needs
# about 3.5 GB for it, and on a modest machine it gets killed by memory pressure.
# GOGC makes the compiler tidy up more often, lowering the peak; -p=1 prevents
# building several packages at once. The build takes longer but goes through
# where it used to break off.
ENV GOGC=30
ENV GOFLAGS=-p=1
RUN ./build.sh

FROM ubuntu:latest
# ffmpeg is mandatory: it produces the first video frame used as a thumbnail.
# Without a thumbnail the client does not show the video in a chat — verified in
# practice, it looks like "the message never arrived". We install only it and
# psmisc (killall in the startup scripts).
RUN apt update -y && apt install -y ffmpeg psmisc && apt-get clean
WORKDIR /app
COPY --from=builder /app/teamgramd/ /app/
RUN chmod +x /app/docker/entrypoint.sh
ENTRYPOINT /app/docker/entrypoint.sh
