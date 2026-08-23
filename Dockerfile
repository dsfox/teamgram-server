FROM golang:1.23.0 AS builder
# The context is the repository root, because the server is built against our
# fork of the schema module - the four MLS methods cannot be added from outside
# it - and go.mod reaches it as ../proto. Copying only the server would fail with
# "replacement directory ../proto does not exist", which names the symptom and
# not this.
WORKDIR /proto
COPY proto/ .
WORKDIR /app
COPY server/ .
# The generated protocol package is the heaviest in the build: the compiler needs
# about 3.5 GB for it, and on a modest machine it gets killed by memory pressure.
# GOGC makes the compiler tidy up more often, lowering the peak; -p=1 prevents
# building several packages at once. The build takes longer but goes through
# where it used to break off.
#
# The default stays one, because that is what a small machine survives. A build
# runner has four cores and sixteen gigabytes, and says so with --build-arg
# GO_PARALLELISM=4 rather than by editing this line.
ARG GO_PARALLELISM=1
ENV GOGC=30
ENV GOFLAGS=-p=${GO_PARALLELISM}
RUN ./build.sh

# The configs in etc2/ still name etcd, and deploy/deploy.sh rewrites them into
# direct addresses before shipping. Do it here too, so the image is enough on
# its own: whoever follows install.sh has no repository to rewrite from. Our own
# deploy goes on mounting its copy over this one, so nothing there changes.
# After the build, so that a config edit does not rebuild the whole server.
COPY deploy/etc/rewrite-configs.py /tmp/rewrite-configs.py
RUN python3 /tmp/rewrite-configs.py teamgramd/etc2 /tmp/etc2 \
 && rm -rf teamgramd/etc2 \
 && mv /tmp/etc2 teamgramd/etc2

FROM ubuntu:latest
# ffmpeg is mandatory: it produces the first video frame used as a thumbnail.
# Without a thumbnail the client does not show the video in a chat — verified in
# practice, it looks like "the message never arrived". We install only it and
# psmisc (killall in the startup scripts). python3-minimal is for the handshake
# check below: a running container is not a working server, and install.sh has
# to be able to tell the difference on a machine where nothing else is
# installed. The check is stdlib only, which is why the minimal package does.
RUN apt update -y && apt install -y ffmpeg psmisc python3-minimal && apt-get clean
WORKDIR /app
COPY --from=builder /app/teamgramd/ /app/
# How a database is brought up, carried inside the image so that the image is
# the whole delivery. install.sh copies these two out and mounts them into
# MySQL exactly as our own compose does; without them a stranger would need the
# repository, and the repository is not theirs to clone.
COPY deploy/mysql-init/ /app/deploy/mysql-init/
COPY deploy/sql-patches/ /app/deploy/sql-patches/
COPY deploy/check-mtproto.py /app/deploy/check-mtproto.py
RUN chmod +x /app/docker/entrypoint.sh /app/deploy/mysql-init/*.sh
ENTRYPOINT /app/docker/entrypoint.sh
