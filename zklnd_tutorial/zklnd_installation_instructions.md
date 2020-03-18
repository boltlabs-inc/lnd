# zkLND Installation

## Introduction

`zkLND` is an extension of `lnd`, which is uses the `libzkchannels` library to run the zkChannels protocol. For more information about the protocol, you can read the high level [overview](zklnd_overview.md).

This installation guide is divided into two parts: The first part is based on the [`lnd` installation guide](https://dev.lightning.community/guides/installation/), but has been edited specifically for the `zkLND` forked version of `lnd`. The second part covers the installation of the `libzkchannels` library.

Once these two parts have been completed, you will be ready to begin the [zkLND tutorial](zklnd_tutorial.md).


## Installing zkLND

While our extension is named `zkLND` the software uses the same name as the original `lnd` software. As such, the program is still named `lnd` and uses the same `lnd` command used when starting up the `zkLND` node. Likewise, the command for interacting with the `zkLND` node is `lncli`.

This section will walk through the installation of `zkLND`, a modified version of `lnd` designed to work with the libzkchannels library necessary for running `zkLND`.

### Preliminaries

In order to work with [`zkLND`](https://github.com/boltlabs-inc/lnd), the following build dependencies are required:

*   **Go:** `lnd` is written in Go. To install, run one of the following commands:

 **Note**: The minimum version of Go supported is Go 1.13. We recommend that users use the latest version of Go, which at the time of writing is [`1.13`](https://blog.golang.org/go1.13).

 On Linux:

 (x86-64)

     wget https://dl.google.com/go/go1.13.linux-amd64.tar.gz
     sha256sum go1.13.linux-amd64.tar.gz | awk -F " " '{ print $1 }'


 The final output of the command above should be `68a2297eb099d1a76097905a2ce334e3155004ec08cdea85f24527be3c48e856`. If it isn’t, then the target REPO HAS BEEN MODIFIED, and you shouldn’t install this version of Go. If it matches, then proceed to install Go:

     tar -C /usr/local -xzf go1.13.linux-amd64.tar.gz
     export PATH=$PATH:/usr/local/go/bin


 (ARMv6)

     wget https://dl.google.com/go/go1.13.linux-armv6l.tar.gz
     sha256sum go1.13.linux-armv6l.tar.gz | awk -F " " '{ print $1 }'


 The final output of the command above should be `931906d67cae1222f501e7be26e0ee73ba89420be0c4591925901cb9a4e156f0`. If it isn’t, then the target REPO HAS BEEN MODIFIED, and you shouldn’t install this version of Go. If it matches, then proceed to install Go:

     tar -C /usr/local -xzf go1.13.linux-armv6l.tar.gz
     export PATH=$PATH:/usr/local/go/bin


 On Mac OS X:

     brew install go@1.13


 On FreeBSD:

     pkg install go


 Alternatively, one can download the pre-compiled binaries hosted on the [Golang download page](https://golang.org/dl/). If one seeks to install from source, then more detailed installation instructions can be found [here](https://golang.org/doc/install).

 At this point, you should set your `$GOPATH` environment variable, which represents the path to your workspace. By default, `$GOPATH` is set to `~/go`. You will also need to add `$GOPATH/bin` to your `PATH`. This ensures that your shell will be able to detect the binaries you install.

     export GOPATH=~/gocode
     export PATH=$PATH:$GOPATH/bin


 We recommend placing the above in your .bashrc or in a setup script so that you can avoid typing this every time you open a new terminal window.

*   **Go modules:** This project uses [Go modules](https://github.com/golang/go/wiki/Modules) to manage dependencies as well as to provide _reproducible builds_.

 Usage of Go modules (with Go 1.12) means that you no longer need to clone `lnd` into your `$GOPATH` for development purposes. Instead, your `lnd` repo can now live anywhere!


### Installing lnd

With the preliminary steps completed, to install `lnd`, `lncli`, and all related dependencies run the following commands:

    go get -d github.com/boltlabs-inc/lnd
    cd $GOPATH/src/github.com/boltlabs-inc/lnd
    make && make install


**NOTE**: Our instructions still use the `$GOPATH` directory from prior versions of Go, but with Go 1.12, it’s now possible for `lnd` to live _anywhere_ on your file system.

For Windows WSL users, make will need to be referenced directly via /usr/bin/make/, or alternatively by wrapping quotation marks around make, like so:

   /usr/bin/make && /usr/bin/make install
   "make" && "make" install


On FreeBSD, use gmake instead of make.

Alternatively, if one doesn’t wish to use `make`, then the `go` commands can be used directly:

    GO111MODULE=on go install -v ./...


### Updating

To update your version of `lnd` to the latest version run the following commands:

   cd $GOPATH/src/github.com/boltlabs-inc/lnd
   git pull
   make clean && make && make install


On FreeBSD, use gmake instead of make.

Alternatively, if one doesn’t wish to use `make`, then the `go` commands can be used directly:

    cd $GOPATH/src/github.com/boltlabs-inc/lnd
    git pull
    GO111MODULE=on go install -v ./...


### Tests

To check that `lnd` was installed properly run the following command:

    make check


This command requires `bitcoind` (almost any version should do) to be available in the system’s `$PATH` variable. Otherwise some of the tests will fail.


### Installing btcd

On FreeBSD, use gmake instead of make.

To install btcd, run the following command:

    make btcd

Alternatively, you can install [`btcd` directly from its repo](https://github.com/btcsuite/btcd).

Note that for our initial release of `zkLND` assumes users are running a btcd node on simnet (rather than the public mainnet or testnet). Therefore, when starting up btcd, it will be necessary to pass in `--bitcoin.simnet`. This is covered in more detail in the tutorial.

## Installing libzkchannels

This section will guide you through the installation of [libzkchannels](https://github.com/boltlabs-inc/libzkchannels).

### Major Dependencies

* secp256k1
* ff
* pairing
* serde, serde_json
* sha2, ripemd160, hmac, hex
* wagyu-bitcoin and wagyu-zcash
* redis

Note that the above rust dependencies will be compiled and installed as a result of running the `make` command.

### Rust Nightly Setup

Please keep in mind we are currently working with nightly Rust for now which gives access to the nightly compiler and experimental features.

    rustup install nightly

To run a quick test of the nightly toolchain, run the following command:

    rustup run nightly rustc --version

Optionally, to make this the default globally, run the following command:

    rustup default nightly

We will switch to the stable release channel once libzkchannels (and dependencies) are ready for production use.

### Build & Install

To be able to build libzkchannels, we require that you install the EMP-toolkit and other dependencies. Make sure you are in your `lnd` directory and run the following commands:

    git clone https://github.com/boltlabs-inc/libzkchannels
    cd libzkchannels
  	. ./env
  	make deps
  	./test_emp.sh
    cargo build --release --manifest-path ./Cargo.toml
    export CGO_LDFLAGS="-L$(pwd)/target/release"
    export LD_LIBRARY_PATH="$(pwd)/target/release"
    cd ..
    go get github.com/boltlabs-inc/libzkchannels
    go test -v github.com/boltlabs-inc/libzkchannels

In addition, you'll need to start up the Redis database as follows:

    ./setup_redis.sh

To build libzkchannels for MPC/  and execute basic examples, run `make`

### Tests

To run libzkchannels unit tests, run `make test` and with MPC, run `make mpctest`

### Benchmarks

To run libzkchannels benchmarks, run `make bench`

### Usage

To use the libzkchannels library, add the `libzkchannels` crate to your dependency file in `Cargo.toml` as follows:

```toml
[dependencies]
zkchannels = "0.4.0"
```

Then add an extern declaration at the root of your crate as follows:
```rust
extern crate zkchannels;
```
