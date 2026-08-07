FROM golang:1.23.0 AS builder
WORKDIR /app
COPY . .
# Пакет сгенерированного протокола — самый тяжёлый в сборке: компилятору на нём
# нужно около 3.5 ГБ, и на скромной машине его убивает нехватка памяти. GOGC
# заставляет компилятор чаще прибираться за собой, снижая пик; -p=1 не даёт
# собирать несколько пакетов разом. Сборка от этого дольше, но проходит там,
# где иначе обрывалась.
ENV GOGC=30
ENV GOFLAGS=-p=1
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
