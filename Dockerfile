# builder stage
FROM centos:7 

ENV PATH=${PATH}:/usr/local/go/bin \
    CGO_LDFLAGS=-L/usr/local/lklibs/ \
    GO111MODULE=on

WORKDIR /src

COPY . .

WORKDIR /usr/local

RUN set -ex  \
    && yum -y install make git gcc gcc-c++ libstdc++-static \
    && yum clean all 

RUN GOLANG_VERSION=1.12.7 \
    && curl -O https://dl.google.com/go/go${GOLANG_VERSION}.linux-amd64.tar.gz \
    && GOLANG_HASH=66d83bfb5a9ede000e33c6579a91a29e6b101829ad41fffb5c5bb6c900e109d9 \
    && echo "${GOLANG_HASH}  go${GOLANG_VERSION}.linux-amd64.tar.gz" | sha256sum -c \
    && tar zxf go${GOLANG_VERSION}.linux-amd64.tar.gz \
    && rm -vf go${GOLANG_VERSION}.linux-amd64.tar.gz 
    
RUN curl -L -O https://github.com/lianxiangcloud/monero/releases/download/libsxcrypto_v0.1.0/lklibs-centos7-x64.tar.gz \
    && LK_LIBS_HASH=a8827347fb372edbb1ab83b4ebcac034f009072a495174bfc3650397533f1c4c \ 
    && echo "${LK_LIBS_HASH}  lklibs-centos7-x64.tar.gz" | sha256sum -c \
    && tar zxf lklibs-centos7-x64.tar.gz \ 
    && rm -vf lklibs-centos7-x64.tar.gz

WORKDIR /src

RUN ./build.sh
