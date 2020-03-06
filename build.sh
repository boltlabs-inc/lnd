cd libzkchannels
git pull
. ./env
# make update
# make deps
make mpcgotest
cargo build --release --manifest-path ./Cargo.toml --features mpc-bitcoin
export CGO_LDFLAGS="-L$(pwd)/target/release"
export LD_LIBRARY_PATH="$(pwd)/target/release"
cd ..
go get github.com/boltlabs-inc/libzkchannels
go test -v github.com/boltlabs-inc/libzkchannels
