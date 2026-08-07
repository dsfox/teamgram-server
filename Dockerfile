FROM golang:1.23.0 AS builder
WORKDIR /app
COPY . .
RUN ./build.sh

FROM ubuntu:latest
# ffmpeg обязателен: из него берётся первый кадр видео для превью. Без превью
# клиент не показывает видео в переписке — проверено вживую, выглядит как
# «сообщение не дошло». Ставим только его и psmisc (killall в скриптах запуска).
RUN apt update -y && apt install -y ffmpeg psmisc && apt-get clean
WORKDIR /app
COPY --from=builder /app/teamgramd/ /app/
RUN chmod +x /app/docker/entrypoint.sh
ENTRYPOINT /app/docker/entrypoint.sh
