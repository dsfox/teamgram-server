FROM golang:1.23.0 AS builder
WORKDIR /app
COPY . .
RUN ./build.sh

FROM ubuntu:latest
# ffmpeg не ставим: он нужен только для превью видео и рингтонов, а тянет
# за собой сотни мегабайт. Без него эти операции вернут ошибку, остальное работает.
# psmisc — для killall в скриптах запуска.
RUN apt update -y && apt install -y psmisc && apt-get clean
WORKDIR /app
COPY --from=builder /app/teamgramd/ /app/
RUN chmod +x /app/docker/entrypoint.sh
ENTRYPOINT /app/docker/entrypoint.sh
