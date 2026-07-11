# The C Library of the Abelian Go SDK

[![GitHub Release](https://img.shields.io/badge/Latest%20release-1.0.0-blue.svg)]()
[![Made with Java](https://img.shields.io/badge/Powered%20by-Go-lightblue.svg)](https://www.java.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-orange.svg)](https://opensource.org/licenses/MIT)

A dynamic library that wraps the Abelian Go SDK for use in programming languages other than Go.

It contains a set of functions that can be used to interact with the Abelian Blockchain.
The library can be used in any programming language (other than Go) that supports C bindings.
For Go, it is recommended to use the [Abelian Go SDK](https://github.com/pqabelian/abel-sdk-go) directly.

## 1. Install dependencies

### 1.1. Install Go

Go version 1.11 or higher is required. Please refer to [the official Go installation guide](https://go.dev/doc/install)
for details.

### 1.2. Install build tools

For Linux:

```shell
sudo apt install astyle cmake gcc ninja-build  pkg-config libssl-dev python3-pytest python3-pytest-xdist unzip xsltproc doxygen graphviz python3-yaml
```

For macOS:

```shell
brew install cmake ninja openssl@1.1 wget doxygen graphviz astyle pkg-config && pip3 install pytest pytest-xdist pyyaml
```

## 2. Build the library

To build the library, clone the repository and run `make` in the root directory of the repository.
To clear the build files, run `make clean`.

If the build is successful, the dynamic library file will be created in the `build` directory.
On Linux, the filename will be `libabelsdk.so` and on macOS, the filename will be `libabelsdk.dylib`.

The C header file `libabelsdk.h` will also be created in the `build` directory. This file contains the function
definitions of the library.

## 3. Use the library

The library can be used in any programming language that supports C bindings.

Note that when building the SDK library, the OpenSSL library is linked statically on macOS and dynamically on Linux.
Therefore, to run an application using the SDK library on another machine, it is required to have the OpenSSL library
installed on Linux while it is not required on macOS.

As the library uses [Protocol Buffers](https://protobuf.dev/) as the serialization format, you will need to generate the
corresponding code for your language using the protocol buffer compiler.
The definition files (`*.proto`) are located in the `resources/protobuf` directory.

You may refer to the [Abelian Java SDK](https://github.com/pqabelian/abelian-sdk-java-v2) as a concrete example of using
the library in real applications.
