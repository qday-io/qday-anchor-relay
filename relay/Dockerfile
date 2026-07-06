# ── Stage 1: build the cryptosuite liboqs fork ─────────────────────────────
# CGO_ENABLED=0 is NOT possible: abelian-sdk-go-v2 → cryptosuite/liboqs-go is a
# cgo binding, and it requires the cryptosuite FORK of liboqs (custom
# OQS_KEM_keypair_with_recovery API — upstream liboqs does not have it).
FROM golang:1.23-bookworm AS liboqs
RUN apt-get update && apt-get install -y --no-install-recommends \
        cmake ninja-build build-essential ca-certificates git \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 https://github.com/cryptosuite/liboqs.git /src/liboqs
# The fork predates modern compilers; -Werror trips on new warnings.
RUN sed -i 's/add_compile_options(-Werror)/# -Werror disabled for modern toolchains/' \
        /src/liboqs/.CMake/compiler_opts.cmake \
    && cmake -S /src/liboqs -B /src/liboqs/build -GNinja \
        -DBUILD_SHARED_LIBS=ON -DOQS_BUILD_ONLY_LIB=ON -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/opt/liboqs \
    && cmake --build /src/liboqs/build \
    && cmake --install /src/liboqs/build
RUN mkdir -p /opt/liboqs/lib/pkgconfig && printf '%s\n' \
        'prefix=/opt/liboqs' \
        'Name: liboqs' \
        'Description: cryptosuite fork of liboqs' \
        'Version: 0.7.5' \
        'Cflags: -I${prefix}/include' \
        'Libs: -L${prefix}/lib -Wl,-rpath,/opt/liboqs/lib -loqs' \
        > /opt/liboqs/lib/pkgconfig/liboqs.pc

# ── Stage 2: build the relay ────────────────────────────────────────────────
FROM golang:1.23-bookworm AS build
COPY --from=liboqs /opt/liboqs /opt/liboqs
ENV PKG_CONFIG_PATH=/opt/liboqs/lib/pkgconfig
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/relay ./cmd/relay

# ── Stage 3: runtime ────────────────────────────────────────────────────────
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=liboqs /opt/liboqs/lib /opt/liboqs/lib
COPY --from=build /out/relay /usr/local/bin/relay
USER nobody
ENTRYPOINT ["/usr/local/bin/relay"]
