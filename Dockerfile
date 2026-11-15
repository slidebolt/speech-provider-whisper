FROM golang:1.26-bookworm AS adapter
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/speech-provider-whisper ./cmd/speech-provider-whisper

FROM debian:bookworm AS whisper
ARG WHISPER_VERSION=v1.7.6
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends build-essential ca-certificates cmake curl git >/dev/null && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch "$WHISPER_VERSION" https://github.com/ggml-org/whisper.cpp /src && \
    cmake -S /src -B /src/build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF -DGGML_NATIVE=OFF -DWHISPER_BUILD_EXAMPLES=ON && \
    cmake --build /src/build --target whisper-server -j"$(nproc)"
RUN mkdir -p /out/models && \
    curl -fsSL -o /out/models/ggml-base.en.bin https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin && \
    echo 'a03779c86df3323075f5e796cb2ce5029f00ec8869eee3fdfb897afe36c6d002  /out/models/ggml-base.en.bin' | sha256sum -c -

FROM debian:bookworm-slim
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends ca-certificates libgomp1 libstdc++6 >/dev/null && rm -rf /var/lib/apt/lists/* && \
    useradd --system --uid 10001 --create-home speech
COPY --from=adapter /out/speech-provider-whisper /usr/local/bin/speech-provider-whisper
COPY --from=whisper /src/build/bin/whisper-server /usr/local/bin/whisper-server
COPY --from=whisper /out/models /models
ENV WHISPER_MODEL=/models/ggml-base.en.bin PROVIDER_ADDR=:8080
USER speech
EXPOSE 8080
ENTRYPOINT ["speech-provider-whisper"]
