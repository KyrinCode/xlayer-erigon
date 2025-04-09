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

COPY go.mod go.sum ./
COPY erigon-lib /app/erigon-lib
COPY cmd/utils/mdbx2rocksdb /app/cmd/utils/mdbx2rocksdb
COPY deps/rocksdb /app/deps/rocksdb

RUN go mod download

RUN cd deps/rocksdb && \
    make -j$(nproc) static_lib && \
    cd ../..

ENV CGO_CFLAGS="-I/app/deps/rocksdb/include"
ENV CGO_LDFLAGS="-L/app/deps/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd"

ENTRYPOINT ["go", "run", "/app/cmd/utils/mdbx2rocksdb/main.go"]

CMD ["--help"]