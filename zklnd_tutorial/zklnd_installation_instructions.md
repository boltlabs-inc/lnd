# zkLND Installation

#### Disclaimer: zkLND is in a proof of concept stage and we are actively improving it and adding features. If you encounter any issues, please post them in GitHub issues. Thanks!

## Introduction

`zkLND` is an extension of `lnd`, which is uses the [`libzkchannels`](https://github.com/boltlabs-inc/libzkchannels) library to run the zkChannels protocol. For more information about the protocol, you can read the high level [overview](zklnd_overview.md).

This installation guide is divided into two parts: The first part covers the installation of the `libzkchannels` library which will be done within the `zkLND` directory. The second part, based on the [`lnd` installation guide](https://dev.lightning.community/guides/installation/), but has been edited specifically for the `zkLND` version of `lnd`.

Once these two parts have been completed, you will be ready to begin the [zkLND tutorial](zklnd_tutorial.md).

## Preliminaries

### Installing Go

In order to work with [`zkLND`](https://github.com/boltlabs-inc/lnd), the following build dependencies are required:

*   **Go:** `lnd` is written in Go. To install, run one of the following commands:

 **Note**: The minimum version of Go supported is Go 1.14. We recommend that users use the latest version of Go, which at the time of writing is [`1.14`](https://blog.golang.org/go1.14).

 On Linux:

 (x86-64)

     wget https://dl.google.com/go/go1.14.linux-amd64.tar.gz
     sha256sum go1.14.linux-amd64.tar.gz | awk -F " " '{ print $1 }'


 The final output of the command above should be `08df79b46b0adf498ea9f320a0f23d6ec59e9003660b4c9c1ce8e5e2c6f823ca`. If it isn’t, then the target REPO HAS BEEN MODIFIED, and you shouldn’t install this version of Go. If it matches, then proceed to install Go:

     tar -C /usr/local -xzf go1.14.linux-amd64.tar.gz
     export PATH=$PATH:/usr/local/go/bin


 (ARMv6)

     wget https://dl.google.com/go/go1.14.linux-armv6l.tar.gz
     sha256sum go1.14.linux-armv6l.tar.gz | awk -F " " '{ print $1 }'


 The final output of the command above should be `b5e682176d7ad3944404619a39b585453a740a2f82683e789f4279ec285b7ecd`. If it isn’t, then the target REPO HAS BEEN MODIFIED, and you shouldn’t install this version of Go. If it matches, then proceed to install Go:

     tar -C /usr/local -xzf go1.14.linux-armv6l.tar.gz
     export PATH=$PATH:/usr/local/go/bin


 On Mac OS X:

     brew install go@1.14


 On FreeBSD:

     pkg install go


 Alternatively, one can download the pre-compiled binaries hosted on the [Golang download page](https://golang.org/dl/). If one seeks to install from source, then more detailed installation instructions can be found [here](https://golang.org/doc/install).

 At this point, you should set your `$GOPATH` environment variable, which represents the path to your workspace. By default, `$GOPATH` is set to `~/go`. You will also need to add `$GOPATH/bin` to your `PATH`. This ensures that your shell will be able to detect the binaries you install.

     export GOPATH=~/gocode
     export PATH=$PATH:$GOPATH/bin


 We recommend placing the above in your .bashrc or in a setup script so that you can avoid typing this every time you open a new terminal window.

*   **Go modules:** This project uses [Go modules](https://github.com/golang/go/wiki/Modules) to manage dependencies as well as to provide _reproducible builds_.

 Usage of Go modules (with Go 1.12) means that you no longer need to clone `lnd` into your `$GOPATH` for development purposes. Instead, your `lnd` repo can now live anywhere!

 ### Installing Rust

To install Rust, we recommend using [rustup](https://www.rustup.rs/). You can install `rustup` on macOS or Linux as follows:

   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   ```

Make sure the version of `rustc` is greater than 1.42. If you have an older version, update it by running:

   ```bash
   rustup update
   ```

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


### Build & Install

Currently, the libzkchannels is located in the `lnd` directory. So first we will clone `zkLND`, then install `libzkchannels` within it.

    git clone https://github.com/boltlabs-inc/lnd
    cd lnd

To be able to build `libzkchannels`, you will need to install the EMP-toolkit among other dependencies. Make sure you are in your `lnd` directory and run the following commands:

    git clone https://github.com/boltlabs-inc/libzkchannels
    ./scripts/install_libzkchannels.sh 
    . ./make/libzkchannels.mk
    go get github.com/boltlabs-inc/libzkchannels
    go test -v github.com/boltlabs-inc/libzkchannels

To build libzkchannels and execute all unit tests, run `make` inside the libzkchannels directory.

### Tests

To run just the libzkchannels unit tests, run `make test` and for MPC-only tests, run `make mpctest`.

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


## Installing zkLND

While our extension is named `zkLND` the software uses the same name as the original `lnd` software. As such, the program is still named `lnd` and uses the same `lnd` command used when starting up the `zkLND` node. Likewise, the command for interacting with the `zkLND` node is `lncli`.

To install `lnd`, `lncli`, and all related dependencies run the following commands from your `lnd` (`zkLND` fork) directory:

    make && make install


**NOTE**: Our instructions still use the `$GOPATH` directory from prior versions of Go, but with Go 1.12, it’s now possible for `lnd` to live _anywhere_ on your file system.

For Windows WSL users, make will need to be referenced directly via /usr/bin/make/, or alternatively by wrapping quotation marks around make, like so:
   ```bash
   /usr/bin/make && /usr/bin/make install
   "make" && "make" install
   ```

On FreeBSD, use `gmake` instead of `make`.

Alternatively, if one doesn’t wish to use `make`, then the `go` commands can be used directly:

    GO111MODULE=on go install -v ./...


### Updating

To update your version of `lnd` to the latest version run the following commands:

    cd $GOPATH/src/github.com/boltlabs-inc/lnd
    git pull
    make clean && make && make install


On FreeBSD, use `gmake` instead of `make`.

Alternatively, if one doesn’t wish to use `make`, then the `go` commands can be used directly:

    cd $GOPATH/src/github.com/boltlabs-inc/lnd
    git pull
    GO111MODULE=on go install -v ./...


### Tests

To check that `lnd` was installed properly run the following command:

    make check


### Installing btcd

On FreeBSD, use `gmake` instead of `make`.

To install `btcd`, run the following command:

    make btcd

Alternatively, you can install [`btcd` directly from its repo](https://github.com/btcsuite/btcd).

Note that for our initial release of `zkLND` assumes users are running a btcd node on simnet (rather than the public mainnet or testnet). Therefore, when starting up btcd, it will be necessary to pass in `--bitcoin.simnet`. This is covered in more detail in the [zkLND tutorial](zklnd_tutorial.md).
