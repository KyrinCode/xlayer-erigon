FROM golang:1.21-bookworm

RUN apt-get update && apt-get install -y \
    git \
    build-essential \
    libsnappy-dev \
    liblz4-dev \
    libzstd-dev \
    zlib1g-dev \
    libbz2-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

ADD go.mod go.mod
ADD go.sum go.sum
ADD erigon-lib/go.mod erigon-lib/go.mod
ADD erigon-lib/go.sum erigon-lib/go.sum

RUN go mod download
ADD . .


RUN cd deps/rocksdb && \
    make -j$(nproc) static_lib && \
    cd ../..

ENV CGO_CFLAGS="-I/app/deps/rocksdb/include"
ENV CGO_LDFLAGS="-L/app/deps/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"

ENTRYPOINT ["go", "run", "/app/cmd/utils/mdbx2rocksdb/main.go"]

CMD ["--help"]