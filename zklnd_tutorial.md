<header class="site-header" role="banner">

<div class="wrapper">[Lightning Network Developers](/)

<nav class="site-nav"><input type="checkbox" id="nav-trigger" class="nav-trigger"> <label for="nav-trigger"><span class="menu-icon">  </span></label>

<div class="trigger">

<div class="dropdown">[<button class="dropbtn">Guides</button>](/guides)

<div class="dropdown-content">[Overview](/overview/) [Installation](/guides/installation/) [Tutorial](/tutorial/) [Python](/guides/python-grpc/) [Javascript](/guides/javascript-grpc/) [Docker](/guides/docker/)</div>

</div>

[API](//api.lightning.community)</div>

</nav>

</div>

</header>

<main class="page-content" aria-label="Content">

<div class="wrapper">

<article class="post">

<header class="post-header">

# Stage 1 - Setting up a local cluster

</header>

<div class="post-content">

### Introduction

In this stage of the tutorial, we will learn how to set up a local cluster of nodes `Alice`, `Bob`, and `Charlie`, have them talk to each other, set up channels, and route payments between one another. We will also establish a baseline understanding of the different components that must work together as part of developing on `lnd`.

This tutorial assumes you have completed installation of Go, `btcd`, and `lnd` on simnet. If not, please refer to the [installation instructions](/guides/installation/). Note that for the purposes of this tutorial it is not necessary to sync testnet, and the last section of the installation instructions you’ll need to complete is “Installing btcd.”

The schema will be the following. Keep in mind that you can easily extend this network to include additional nodes `David`, `Eve`, etc. by simply running more local `lnd` instances.

<div class="highlighter-rouge">

       (1)                        (1)                         (1)
    + ----- +                   + --- +                   + ------- +
    | Alice | <--- channel ---> | Bob | <--- channel ---> | Charlie |
    + ----- +                   + --- +                   + ------- +
        |                          |                           |
        |                          |                           |
        + - - - -  - - - - - - - - + - - - - - - - - - - - - - +
                                   |
                          + --------------- +
                          | BTC/LTC network | <--- (2)
                          + --------------- +

</div>

### Understanding the components

#### LND

`lnd` is the main component that we will interact with. `lnd` stands for Lightning Network Daemon, and handles channel opening/closing, routing and sending payments, and managing all the Lightning Network state that is separate from the underlying Bitcoin network itself.

Running an `lnd` node means that it is listening for payments, watching the blockchain, etc. By default it is awaiting user input.

`lncli` is the command line client used to interact with your `lnd` nodes. Typically, each `lnd` node will be running in its own terminal window, so that you can see its log outputs. `lncli` commands are thus run from a different terminal window.

#### BTCD

`btcd` represents the gateway that `lnd` nodes will use to interact with the Bitcoin / Litecoin network. `lnd` needs `btcd` for creating on-chain addresses or transactions, watching the blockchain for updates, and opening/closing channels. In our current schema, all three of the nodes are connected to the same `btcd` instance. In a more realistic scenario, each of the `lnd` nodes will be connected to their own instances of `btcd` or equivalent.

We will also be using `simnet` instead of `testnet`. Simnet is a development/test network run locally that allows us to generate blocks at will, so we can avoid the time-consuming process of waiting for blocks to arrive for any on-chain functionality.

### Setting up our environment

Developing on `lnd` can be quite complex since there are many more moving pieces, so to simplify that process, we will walk through a recommended workflow.

#### Running btcd

Let’s start by running btcd, if you don’t have it up already. Open up a new terminal window, ensure you have your `$GOPATH` set, and run:

<div class="language-bash highlighter-rouge">

    btcd --txindex --simnet --rpcuser=kek --rpcpass=kek

</div>

(Note: this tutorial requires opening quite a few terminal windows. It may be convenient to use multiplexers such as tmux or screen if you’re familiar with them.)

Breaking down the components:

*   `--txindex` is required so that the `lnd` client is able to query historical transactions from `btcd`.
*   `--simnet` specifies that we are using the `simnet` network. This can be changed to `--testnet`, or omitted entirely to connect to the actual Bitcoin / Litecoin network.
*   `--rpcuser` and `rpcpass` sets a default password for authenticating to the `btcd` instance.

#### Starting lnd (Alice’s node)

Now, let’s set up the three `lnd` nodes. To keep things as clean and separate as possible, open up a new terminal window, ensure you have `$GOPATH` set and `$GOPATH/bin` in your `PATH`, and create a new directory under `$GOPATH` called `dev` that will represent our development space. We will create separate folders to store the state for alice, bob, and charlie, and run all of our `lnd` nodes on different `localhost` ports instead of using [Docker](/guides/docker/) to make our networking a bit easier.

<div class="language-bash highlighter-rouge">

    # Create our development space
    cd $GOPATH
    mkdir dev
    cd dev

    # Create folders for each of our nodes
    mkdir alice bob charlie

</div>

The directory structure should now look like this:

<div class="language-bash highlighter-rouge">

    $ tree $GOPATH -L 2

    ├── bin
    │   └── ...
    ├── dev
    │   ├── alice
    │   ├── bob
    │   └── charlie
    ├── pkg
    │   └── ...
    ├── rpc
    │   └── ...
    └── src
        └── ...

</div>

Start up the Alice node from within the `alice` directory:

<div class="language-bash highlighter-rouge">

    cd $GOPATH/dev/alice
    alice$ lnd --rpclisten=localhost:10001 --listen=localhost:10011 --restlisten=localhost:8001 --datadir=data --logdir=log --debuglevel=info --bitcoin.simnet --bitcoin.active --bitcoin.node=btcd --btcd.rpcuser=kek --btcd.rpcpass=kek

</div>

The Alice node should now be running and displaying output ending with a line beginning with “Waiting for wallet encryption password.”

Breaking down the components:

*   `--rpclisten`: The host:port to listen for the RPC server. This is the primary way an application will communicate with `lnd`
*   `--listen`: The host:port to listen on for incoming P2P connections. This is at the networking level, and is distinct from the Lightning channel networks and Bitcoin/Litcoin network itself.
*   `--restlisten`: The host:port exposing a REST api for interacting with `lnd` over HTTP. For example, you can get Alice’s channel balance by making a GET request to `localhost:8001/v1/channels`. This is not needed for this tutorial, but you can see some examples [here](https://gist.github.com/Roasbeef/624c02cd5a90a44ab06ea90e30a6f5f0).
*   `--datadir`: The directory that `lnd`’s data will be stored inside
*   `--logdir`: The directory to log output.
*   `--debuglevel`: The logging level for all subsystems. Can be set to `trace`, `debug`, `info`, `warn`, `error`, `critical`.
*   `--bitcoin.simnet`: Specifies whether to use `simnet` or `testnet`
*   `--bitcoin.active`: Specifies that bitcoin is active. Can also include `--litecoin.active` to activate Litecoin.
*   `--bitcoin.node=btcd`: Use the `btcd` full node to interface with the blockchain. Note that when using Litecoin, the option is `--litecoin.node=btcd`.
*   `--btcd.rpcuser` and `--btcd.rpcpass`: The username and password for the `btcd` instance. Note that when using Litecoin, the options are `--ltcd.rpcuser` and `--ltcd.rpcpass`.

### Starting Bob’s node and Charlie’s node

Just as we did with Alice, start up the Bob node from within the `bob` directory, and the Charlie node from within the `charlie` directory. Doing so will configure the `datadir` and `logdir` to be in separate locations so that there is never a conflict.

Keep in mind that for each additional terminal window you set, you will need to set `$GOPATH` and include `$GOPATH/bin` in your `PATH`. Consider creating a setup script that includes the following lines:

<div class="language-bash highlighter-rouge">

    export GOPATH=~/gocode # if you exactly followed the install guide
    export PATH=$PATH:$GOPATH/bin

</div>

and run it every time you start a new terminal window working on `lnd`.

Run Bob and Charlie:

<div class="language-bash highlighter-rouge">

    # In a new terminal window
    cd $GOPATH/dev/bob
    bob$ lnd --rpclisten=localhost:10002 --listen=localhost:10012 --restlisten=localhost:8002 --datadir=data --logdir=log --debuglevel=info --bitcoin.simnet --bitcoin.active --bitcoin.node=btcd --btcd.rpcuser=kek --btcd.rpcpass=kek

    # In another terminal window
    cd $GOPATH/dev/charlie
    charlie$ lnd --rpclisten=localhost:10003 --listen=localhost:10013 --restlisten=localhost:8003 --datadir=data --logdir=log --debuglevel=info --bitcoin.simnet --bitcoin.active --bitcoin.node=btcd --btcd.rpcuser=kek --btcd.rpcpass=kek

</div>

### Configuring lnd.conf

To skip having to type out a bunch of flags on the command line every time, we can instead modify our `lnd.conf`, and the arguments specified therein will be loaded into `lnd` automatically. Any additional configuration added as a command line argument will be applied _after_ reading from `lnd.conf`, and will overwrite the `lnd.conf` option if applicable.

*   On MacOS, `lnd.conf` is located at: `/Users/[username]/Library/Application\ Support/Lnd/lnd.conf`
*   On Linux: `~/.lnd/lnd.conf`

Here is an example `lnd.conf` that can save us from re-specifying a bunch of command line options:

<div class="language-bash highlighter-rouge">

    [Application Options]
    datadir=data
    logdir=log
    debuglevel=info

    [Bitcoin]
    bitcoin.simnet=1
    bitcoin.active=1
    bitcoin.node=btcd

    [btcd]
    btcd.rpcuser=kek
    btcd.rpcpass=kek

</div>

Now, when we start nodes, we only have to type

<div class="language-bash highlighter-rouge">

    alice$ lnd --rpclisten=localhost:10001 --listen=localhost:10011 --restlisten=localhost:8001
    bob$ lnd --rpclisten=localhost:10002 --listen=localhost:10012 --restlisten=localhost:8002
    charlie$ lnd --rpclisten=localhost:10003 --listen=localhost:10013 --restlisten=localhost:8003

</div>

etc.

### Working with lncli and authentication

Now that we have our `lnd` nodes up and running, let’s interact with them! To control `lnd` we will need to use `lncli`, the command line interface.

`lnd` uses [macaroons](https://github.com/lightningnetwork/lnd/issues/20) for authentication to the rpc server. `lncli` typically looks for an `admin.macaroon` file in the Lnd home directory, but since we changed the location of our application data, we have to set `--macaroonpath` in the following command. To disable macaroons, pass the `--no-macaroons` flag into both `lncli` and `lnd`.

`lnd` allows you to encrypt your wallet with a passphrase and optionally encrypt your cipher seed passphrase as well. This can be turned off by passing `--noencryptwallet` into `lnd` or `lnd.conf`. We recommend going through this process at least once to familiarize yourself with the security and authentication features around `lnd`.

We will test our rpc connection to the Alice node. Notice that in the following command we specify the `--rpcserver` here, which corresponds to `--rpcport=10001` that we set when starting the Alice `lnd` node.

Open up a new terminal window, set `$GOPATH` and include `$GOPATH/bin` in your `PATH` as usual. Let’s create Alice’s wallet and set her passphrase:

<div class="language-bash highlighter-rouge">

    cd $GOPATH/dev/alice
    alice$ lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon create

</div>

You’ll be asked to input and confirm a wallet password for Alice, which must be longer than 8 characters. You also have the option to add a passphrase to your cipher seed. For now, just skip this step by entering “n” when prompted about whether you have an existing mnemonic, and pressing enter to proceed without the passphrase.

You can now request some basic information as follows:

<div class="language-bash highlighter-rouge">

    alice$ lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon getinfo

</div>

`lncli` just made an RPC call to the Alice `lnd` node. This is a good way to test if your nodes are up and running and `lncli` is functioning properly. Note that in future sessions you may need to call `lncli unlock` to unlock the node with the password you just set.

Open up new terminal windows and do the same for Bob and Charlie. `alice<div class="post-content" or `bob<div class="post-content" denotes running the command from the Alice or Bob `lncli` window respectively.

<div class="language-bash highlighter-rouge">

    # In a new terminal window, setting $GOPATH, etc.
    cd $GOPATH/dev/bob
    bob$ lncli --rpcserver=localhost:10002 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon create
    # Note that you'll have to enter an 8+ character password and "n" for the mnemonic.

    # In a new terminal window:
    cd $GOPATH/dev/charlie
    charlie$ lncli --rpcserver=localhost:10003 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon create
    # Note that you'll have to enter an 8+ character password and "n" for the mnemonic.

</div>

To avoid typing the `--rpcserver=localhost:1000X` and `--macaroonpath` flag every time, we can set some aliases. Add the following to your `.bashrc`:

<div class="language-bash highlighter-rouge">

    alias lncli-alice="lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon"
    alias lncli-bob="lncli --rpcserver=localhost:10002 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon"
    alias lncli-charlie="lncli --rpcserver=localhost:10003 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon"

</div>

To make sure this was applied to all of your current terminal windows, rerun your `.bashrc` file:

<div class="language-bash highlighter-rouge">

    alice$ source ~/.bashrc
    bob$ source ~/.bashrc
    charlie$ source ~/.bashrc

</div>

For simplicity, the rest of the tutorial will assume that this step was complete.

#### lncli options

To see all the commands available for `lncli`, simply type `lncli --help` or `lncli -h`.

### Setting up Bitcoin addresses

Let’s create a new Bitcoin address for Alice. This will be the address that stores Alice’s on-chain balance. `np2wkh` specifes the type of address and stands for Pay to Nested Witness Key Hash.

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice newaddress np2wkh
    {
        "address": <ALICE_ADDRESS>
    }

</div>

And for Bob and Charlie:

<div class="language-bash highlighter-rouge">

    bob$ lncli-bob newaddress np2wkh
    {
        "address": <BOB_ADDRESS>
    }

    charlie$ lncli-charlie newaddress np2wkh
    {
        "address": <CHARLIE_ADDRESS>
    }

</div>

### Funding Alice

That’s a lot of configuration! At this point, we’ve generated onchain addresses for Alice, Bob, and Charlie. Now, we will get some practice working with `btcd` and fund these addresses with some `simnet` Bitcoin.

Quit btcd and re-run it, setting Alice as the recipient of all mining rewards:

<div class="language-bash highlighter-rouge">

    btcd --simnet --txindex --rpcuser=kek --rpcpass=kek --miningaddr=<ALICE_ADDRESS>

</div>

Generate 400 blocks, so that Alice gets the reward. We need at least 100 blocks because coinbase funds can’t be spent until after 100 confirmations, and we need about 300 to activate segwit. In a new window with `$GOPATH` and `$PATH` set:

<div class="language-bash highlighter-rouge">

    alice$ btcctl --simnet --rpcuser=kek --rpcpass=kek generate 400

</div>

Check that segwit is active:

<div class="language-bash highlighter-rouge">

    btcctl --simnet --rpcuser=kek --rpcpass=kek getblockchaininfo | grep -A 1 segwit

</div>

Check Alice’s wallet balance.

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice walletbalance

</div>

It’s no fun if only Alice has money. Let’s give some to Charlie as well.

<div class="language-bash highlighter-rouge">

    # Quit the btcd process that was previously started with Alice's mining address,
    # and then restart it with:
    btcd --txindex --simnet --rpcuser=kek --rpcpass=kek --miningaddr=<CHARLIE_ADDRESS>

    # Generate more blocks
    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 100

    # Check Charlie's balance
    charlie$ lncli-charlie walletbalance

</div>

### Creating the P2P Network

Now that Alice and Charlie have some simnet Bitcoin, let’s start connecting them together.

Connect Alice to Bob:

<div class="language-bash highlighter-rouge">

    # Get Bob's identity pubkey:
    bob$ lncli-bob getinfo
    {
    --->"identity_pubkey": <BOB_PUBKEY>,
        "alias": "",
        "num_pending_channels": 0,
        "num_active_channels": 0,
        "num_peers": 0,
        "block_height": 450,
        "block_hash": "2a84b7a2c3be81536ef92cf382e37ab77f7cfbcf229f7d553bb2abff3e86231c",
        "synced_to_chain": true,
        "testnet": false,
        "chains": [
            "bitcoin"
        ],
        "uris": [
        ],
        "best_header_timestamp": "1533350134",
        "version":  "0.4.2-beta commit=7a5a824d179c6ef16bd78bcb7a4763fda5f3f498"
    }

    # Connect Alice and Bob together
    alice$ lncli-alice connect <BOB_PUBKEY>@localhost:10012
    {

    }

</div>

Notice that `localhost:10012` corresponds to the `--listen=localhost:10012` flag we set when starting the Bob `lnd` node.

Let’s check that Alice and Bob are now aware of each other.

<div class="language-bash highlighter-rouge">

    # Check that Alice has added Bob as a peer:
    alice$ lncli-alice listpeers
    {
        "peers": [
            {
                "pub_key": <BOB_PUBKEY>,
                "address": "127.0.0.1:10012",
                "bytes_sent": "7",
                "bytes_recv": "7",
                "sat_sent": "0",
                "sat_recv": "0",
                "inbound": false,
                "ping_time": "0"
            }
        ]
    }

    # Check that Bob has added Alice as a peer:
    bob$ lncli-bob listpeers
    {
        "peers": [
            {
                "pub_key": <ALICE_PUBKEY>,
                "address": "127.0.0.1:60104",
                "bytes_sent": "318",
                "bytes_recv": "318",
                "sat_sent": "0",
                "sat_recv": "0",
                "inbound": true,
                "ping_time": "5788"
            }
        ]
    }

</div>

Finish up the P2P network by connecting Bob to Charlie:

<div class="language-bash highlighter-rouge">

    charlie$ lncli-charlie connect <BOB_PUBKEY>@localhost:10012

</div>

### Setting up Lightning Network

Before we can send payment, we will need to set up payment channels from Alice to Bob, and Bob to Charlie.

First, let’s open the Alice<–>Bob channel.

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice openchannel --node_key=<BOB_PUBKEY> --local_amt=1000000

</div>

*   `--local_amt` specifies the amount of money that Alice will commit to the channel. To see the full list of options, you can try `lncli openchannel --help`.

We now need to mine six blocks so that the channel is considered valid:

<div class="language-bash highlighter-rouge">

    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 6

</div>

Check that Alice<–>Bob channel was created:

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice listchannels
    {
        "channels": [
            {
                "active": true,
                "remote_pubkey": <BOB_PUBKEY>,
                "channel_point": "2622b779a8acca471a738b0796cd62e4457b79b33265cbfa687aadccc329023a:0",
                "chan_id": "495879744192512",
                "capacity": "1000000",
                "local_balance": "991312",
                "remote_balance": "0",
                "commit_fee": "8688",
                "commit_weight": "600",
                "fee_per_kw": "12000",
                "unsettled_balance": "0",
                "total_satoshis_sent": "0",
                "total_satoshis_received": "0",
                "num_updates": "0",
                "pending_htlcs": [
                ],
                "csv_delay": 144,
                "private": false
            }
        ]
    }

</div>

### Sending single hop payments

Finally, to the exciting part - sending payments! Let’s send a payment from Alice to Bob.

First, Bob will need to generate an invoice:

<div class="language-bash highlighter-rouge">

    bob$ lncli-bob addinvoice --amt=10000
    {
            "r_hash": "<a_random_rhash_value>",
            "pay_req": "<encoded_invoice>",
            "add_index": 1
    }

</div>

Send the payment from Alice to Bob:

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice sendpayment --pay_req=<encoded_invoice>
    {
    	"payment_error": "",
    	"payment_preimage": "baf6929fc95b3824fb774a4b75f6c8a1ad3aaef04efbf26cc064904729a21e28",
    	"payment_route": {
    		"total_time_lock": 650,
    		"total_amt": 10000,
    		"hops": [
    			{
    				"chan_id": 495879744192512,
    				"chan_capacity": 1000000,
    				"amt_to_forward": 10000,
                    "expiry": 650,
                    "amt_to_forward_msat": 10000000
    			}
    		],
    		"total_amt_msat": 10000000
    	}
    }

    # Check that Alice's channel balance was decremented accordingly:
    alice$ lncli-alice listchannels

    # Check that Bob's channel was credited with the payment amount:
    bob$ lncli-bob listchannels

</div>

### Multi-hop payments

Now that we know how to send single-hop payments, sending multi hop payments is not that much more difficult. Let’s set up a channel from Bob<–>Charlie:

<div class="language-bash highlighter-rouge">

    charlie$ lncli-charlie openchannel --node_key=<BOB_PUBKEY> --local_amt=800000 --push_amt=200000

    # Mine the channel funding tx
    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 6

</div>

Note that this time, we supplied the `--push_amt` argument, which specifies the amount of money we want the other party to have at the first channel state.

Let’s make a payment from Alice to Charlie by routing through Bob:

<div class="language-bash highlighter-rouge">

    charlie$ lncli-charlie addinvoice --amt=10000
    alice$ lncli-alice sendpayment --pay_req=<encoded_invoice>

    # Check that Charlie's channel was credited with the payment amount (e.g. that
    # the `remote_balance` has been decremented by 10000)
    charlie$ lncli-charlie listchannels

</div>

### Closing channels

For practice, let’s try closing a channel. We will reopen it in the next stage of the tutorial.

<div class="language-bash highlighter-rouge">

    alice$ lncli-alice listchannels
    {
        "channels": [
            {
                "active": true,
                "remote_pubkey": "0343bc80b914aebf8e50eb0b8e445fc79b9e6e8e5e018fa8c5f85c7d429c117b38",
           ---->"channel_point": "3511ae8a52c97d957eaf65f828504e68d0991f0276adff94c6ba91c7f6cd4275:0",
                "chan_id": "1337006139441152",
                "capacity": "1005000",
                "local_balance": "990000",
                "remote_balance": "10000",
                "unsettled_balance": "0",
                "total_satoshis_sent": "10000",
                "total_satoshis_received": "0",
                "num_updates": "2"
            }
        ]
    }

</div>

The Channel point consists of two numbers separated by a colon, which uniquely identifies the channel. The first number is `funding_txid` and the second number is `output_index`.

<div class="language-bash highlighter-rouge">

    # Close the Alice<-->Bob channel from Alice's side.
    alice$ lncli-alice closechannel --funding_txid=<funding_txid> --output_index=<output_index>

    # Mine a block including the channel close transaction to close the channel:
    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 1

    # Check that Bob's on-chain balance was credited by his settled amount in the
    # channel. Recall that Bob previously had no on-chain Bitcoin:
    bob$ lncli-bob walletbalance
    {
        "total_balance": "20001",
        "confirmed_balance": "20001",
        "unconfirmed_balance": "0"
    }

</div>

At this point, you’ve learned how to work with `btcd`, `btcctl`, `lnd`, and `lncli`. In [Stage 2](/tutorial/02-web-client), we will learn how to set up and interact with `lnd` using a web GUI client.

_In the future, you can try running through this workflow on `testnet` instead of `simnet`, where you can interact with and send payments through the testnet Lightning Faucet node. For more information, see the “Connect to faucet lightning node” section in the [Docker guide](/guides/docker/) or check out the [Lightning Network faucet repository](https://github.com/lightninglabs/lightning-faucet)._

#### Navigation

*   [Proceed to Stage 2 - Web Client](/tutorial/02-web-client)
*   [Return to main tutorial page](/tutorial/)

### Questions

*   Join the #dev-help channel on our [Community Slack](https://join.slack.com/t/lightningcommunity/shared_invite/enQtMzQ0OTQyNjE5NjU1LWRiMGNmOTZiNzU0MTVmYzc1ZGFkZTUyNzUwOGJjMjYwNWRkNWQzZWE3MTkwZjdjZGE5ZGNiNGVkMzI2MDU4ZTE)
*   Join IRC: [![Irc](https://img.shields.io/badge/chat-on%20freenode-brightgreen.svg)](https://webchat.freenode.net/?channels=lnd)

</div>

</article>

</div>

</main>

<footer class="site-footer">

<div class="wrapper">

<div class="footer-col-wrapper">

<div class="footer-col footer-col-1">

*   Lightning Network Developers
*   [<span><span class="__cf_email__" data-cfemail="b6ded3dadad9f6dadfd1dec2d8dfd8d198d3d8d1dfd8d3d3c4dfd8d1">[email protected]</span></span>](/cdn-cgi/l/email-protection#deb6bbb2b2b19eb2b7b9b6aab0b7b0b9f0bbb0b9b7b0bbbbacb7b0b9)

</div>

<div class="footer-col footer-col-2">

*   [<span class="icon icon--github"></span><span class="username">lnd</span>](https://github.com/lightningnetwork/lnd)
*   [<span class="icon icon--twitter"></span><span class="username">lightning</span>](https://twitter.com/lightning)

</div>

<div class="footer-col footer-col-3">

Developer resources and documentation for the Lightning Network Daemon.

</div>

</div>

</div>

</footer>
