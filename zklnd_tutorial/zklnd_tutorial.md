# zkchannel Tutorial

#### Disclaimer: zkLND is in a proof of concept stage and we are actively improving it and adding features. If you encounter any issues, please post them in GitHub issues. Thanks!

This tutorial is based on the [LND Tutorial](https://dev.lightning.community/tutorial/01-lncli) and has been edited to describe the usage of zkLND.

This tutorial assumes you have `zkLND` with `libzkchannels` installed. If not, please refer to the [zkLND installation guide](zklnd_installation_instructions.md).

You may also find it helpful to understand the transactions and Bitcoin scripts used in zkChannels by reading the high level [overview](zklnd_overview.md).

For more information about zkChannels, read the TODO: add link [zkchannels blog post](https://medium.com/boltlabs/zkchannels-for-bitcoin-f1bbf6e3570e).

### Introduction

This tutorial is designed to walk you through how to set up a zero-knowledge channel (zkchannel), using `zklnd`. `zklnd` is an extension of `lnd` that uses secure multi-party computation to allow customers to make unlinkable payments to a merchant.

Here, we will learn how to set up a local payment channel between a customer, `Alice`, and a merchant, `Bob`. We will have them talk to each other, set up a channel, and let `Alice` initiate payments. We will also establish a baseline understanding of the different components that must work together as part of developing on `zklnd`.

This tutorial assumes you have completed installation of Go, `btcd`, and `zklnd` on simnet. If not, please refer to the [installation instructions](zklnd_installation_instructions.md). Note that for the purposes of this tutorial it is not necessary to sync testnet, and the last section of the installation instructions you’ll need to complete is “Installing btcd.”

The schema will be the following.

    Customer                    Merchant
    + ----- +                   + --- +
    | Alice | <-- zkchannel --> | Bob |
    + ----- +                   + --- +
        |                          |
        |                          |
        + - - - -  - - - - - - - - +
                      |
             + --------------- +
            | BTC/LTC network |
             + --------------- +


### Understanding the components

#### zkLND

`zklnd` is the main component that we will interact with. `zklnd` is an extension of `lnd` (Lightning Network Daemon) that explores use of secure multi-party computation and zero-knowledge proof techniques to allow a customer to make unlinkable payments. `zklnd` handles channel opening/closing, sending payments, and managing all the state that is separate from the underlying Bitcoin network itself.

Running a `zklnd` node means that it is listening for payments, watching the blockchain, etc. By default it is awaiting user input.

`lncli` is the command line client used to interact with your `zklnd` nodes. Typically, each `zklnd` node will be running in its own terminal window, so that you can see its log outputs. `lncli` commands are thus run from a different terminal window.

Note that unlike regular Lightning Network payment channels, zkchannels are asymmetric. One side of the channel, referred to as the 'customer', will have full knowledge of the balance within the channel. The other side of the channel, referred to as the 'merchant', will only know the total amount held by the channel, but not the balance with in. In this tutorial, `Alice` plays the role of a customer, and `Bob` of a merchant.

#### BTCD

`btcd` represents the gateway that `zklnd` nodes will use to interact with the Bitcoin / Litecoin network. `zklnd` needs `btcd` for creating on-chain addresses or transactions, watching the blockchain for updates, and opening/closing channels. In our current schema, both of the nodes are connected to the same `btcd` instance. In a more realistic scenario, each `zklnd` node will be connected to their own instances of `btcd` or equivalent.

We will also be using `simnet` instead of `testnet`. Simnet is a development/test network run locally that allows us to generate blocks at will, so we can avoid the time-consuming process of waiting for blocks to arrive for any on-chain functionality.

### Setting up our environment

Developing on `zklnd` can be quite complex since there are many more moving pieces, so to simplify that process, we will walk through a recommended workflow.

#### Running btcd

Let’s start by running btcd, if you don’t have it up already. Open up a new terminal window and run:

    btcd --txindex --simnet --rpcuser=kek --rpcpass=kek  --minrelaytxfee=0

(Note: this tutorial requires opening quite a few terminal windows. It may be convenient to use multiplexers such as tmux or screen if you’re familiar with them.)

Breaking down the components:

*   `--txindex` is required so that the `lnd` client is able to query historical transactions from `btcd`.
*   `--simnet` specifies that we are using the `simnet` network. This can be changed to `--testnet`, or omitted entirely to connect to the actual Bitcoin / Litecoin network.
*   `--rpcuser` and `rpcpass` sets a default password for authenticating to the `btcd` instance.
*   ` --minrelaytxfee=0` is required so that btcd will process transactions that do not include a transaction fee. For simplicity, this initial release of `zklnd` does not include transaction fees for on chain transactions.

#### Starting zklnd (Alice’s node)

Now, let’s set up the two `zklnd` nodes. To keep things as clean and separate as possible, open up a new terminal window, and create a new directory called `dev` that will represent our development space. We will create separate folders to store the state for alice and bob, and run all of our `zklnd` nodes on different `localhost` ports instead of using [Docker](/guides/docker/) to make our networking a bit easier.

    # Create our development space
    mkdir dev
    cd dev

    # Create folders for each of our nodes
    mkdir alice bob

The directory structure should now look like this:

    $ tree . -L 2

    ├── bin
    │   └── ...
    ├── dev
    │   ├── alice
    │   └── bob
    ├── pkg
    │   └── ...
    ├── rpc
    │   └── ...
    └── src
        └── ...

Start up the Alice node from within the `alice` directory:

    cd dev/alice
    alice$ lnd --rpclisten=localhost:10001 --listen=localhost:10011 --restlisten=localhost:8001 --datadir=data --logdir=log --debuglevel=info --bitcoin.simnet --bitcoin.active --bitcoin.node=btcd --btcd.rpcuser=kek --btcd.rpcpass=kek

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

### Starting Bob’s node

Just as we did with Alice, start up the Bob node from within the `bob` directory. Doing so will configure the `datadir` and `logdir` to be in separate locations so that there is never a conflict.

Since Bob is running zklnd as a merchant, we will pass ` --zkmerchant` into `zklnd`:

    # In a new terminal window
    cd dev/bob
    bob$ lnd --rpclisten=localhost:10002 --listen=localhost:10012 --restlisten=localhost:8002 --datadir=data --logdir=log --debuglevel=info --bitcoin.simnet --bitcoin.active --bitcoin.node=btcd --btcd.rpcuser=kek --btcd.rpcpass=kek --zkmerchant

### Configuring lnd.conf

To skip having to type out a bunch of flags on the command line every time, we can instead modify our `lnd.conf`, and the arguments specified therein will be loaded into `lnd` automatically. Any additional configuration added as a command line argument will be applied _after_ reading from `lnd.conf`, and will overwrite the `lnd.conf` option if applicable.

*   On MacOS, `lnd.conf` is located at: `/Users/[username]/Library/Application\ Support/Lnd/lnd.conf`
*   On Linux: `~/.lnd/lnd.conf`

Here is an example `lnd.conf` that can save us from re-specifying a bunch of command line options:

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

Now, when we start nodes, we only have to type

    alice$ lnd --rpclisten=localhost:10001 --listen=localhost:10011 --restlisten=localhost:8001
    bob$ lnd --rpclisten=localhost:10002 --listen=localhost:10012 --restlisten=localhost:8002 --zkmerchant

etc.

### Working with lncli and authentication

Now that we have our `lnd` nodes up and running, let’s interact with them! To control `lnd`, we will need to use `lncli`, the command line interface.

`lnd` uses [macaroons](https://github.com/lightningnetwork/lnd/issues/20) for authentication to the rpc server. `lncli` typically looks for an `admin.macaroon` file in the Lnd home directory, but since we changed the location of our application data, we have to set `--macaroonpath` in the following command. To disable macaroons, pass the `--no-macaroons` flag into both `lncli` and `lnd`.

`lnd` will ask you to encrypt your wallet with a passphrase and optionally encrypt your cipher seed passphrase as well.

We will test our rpc connection to the Alice node. Notice that in the following command we specify the `--rpcserver` here, which corresponds to `--rpcport=10001` that we set when starting the Alice `lnd` node.

Open up a new terminal window. Let’s create Alice’s wallet and set her passphrase:

    cd dev/alice
    alice$ lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon create
    # Note that you'll have to enter an 8+ character password and "n" for the mnemonic.

You’ll be asked to input and confirm a wallet password for Alice, which must be longer than 8 characters. You also have the option to add a passphrase to your cipher seed. For now, just skip this step by entering “n” when prompted about whether you have an existing mnemonic, and pressing enter to proceed without the passphrase.

You can now request some basic information as follows:

    alice$ lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon getinfo

`lncli` just made an RPC call to the Alice `lnd` node. This is a good way to test if your nodes are up and running and `lncli` is functioning properly. Note that in future sessions you may need to call `lncli unlock` to unlock the node with the password you just set.

Open up new terminal windows and do the same for Bob. `alice$` or `bob$` denotes running the command from the Alice or Bob `lncli` window respectively.

    cd dev/bob
    bob$ lncli --rpcserver=localhost:10002 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon create
    # Note that you'll have to enter an 8+ character password and "n" for the mnemonic.

To avoid typing the `--rpcserver=localhost:1000X` and `--macaroonpath` flag every time, we can set some aliases. Add the following to your `.bashrc`:

    alias lncli-alice="lncli --rpcserver=localhost:10001 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon"
    alias lncli-bob="lncli --rpcserver=localhost:10002 --macaroonpath=data/chain/bitcoin/simnet/admin.macaroon"

To make sure this was applied to all of your current terminal windows, rerun your `.bashrc` file:

    alice$ source ~/.bashrc
    bob$ source ~/.bashrc

For simplicity, the rest of the tutorial will assume that this step was complete.

#### lncli options

To see all the commands available for `lncli`, simply type `lncli --help` or `lncli -h`.

### Setting up Bitcoin addresses

Let’s create a new Bitcoin address for Alice. This will be the address that stores Alice’s on-chain balance. `np2wkh` specifes the type of address and stands for Pay to Nested Witness Key Hash.

    alice$ lncli-alice newaddress np2wkh
    {
        "address": <ALICE_ADDRESS>
    }

### Funding Alice

That’s a lot of configuration! At this point, we’ve generated an onchain address for Alice. Now, we will get some practice working with `btcd` and fund these addresses with some `simnet` Bitcoin.

Quit btcd and re-run it, setting Alice as the recipient of all mining rewards:

    btcd --simnet --txindex --rpcuser=kek --rpcpass=kek --minrelaytxfee=0 --miningaddr=<ALICE_ADDRESS>

Generate 400 blocks, so that Alice gets the reward. We need at least 100 blocks because coinbase funds can’t be spent until after 100 confirmations, and we need about 300 to activate segwit. In a new window set:

    alice$ btcctl --simnet --rpcuser=kek --rpcpass=kek generate 400

Check that segwit is active:

    btcctl --simnet --rpcuser=kek --rpcpass=kek getblockchaininfo | grep -A 1 segwit

Check Alice’s wallet balance.

    alice$ lncli-alice walletbalance

### Setting up a zkchannel

Now that Alice has some simnet Bitcoin, let’s set up a zkchannel between Alice and Bob. Since Alice is playing the role of the customer, it's Alice who must initiate the zkchannel opening. 

Note that unlike Lightning Network payment channels in which the peers maintain persistent P2P connections with each other, in zkchannels the customer only maintains a P2P connection with a merchant for the duration of establishing the payment channel (or making a payment). Connecting and disconnecting from the merchant are handled automatically during channel establishment and payment sessions. 

In order to create a zkchannel with Bob, Alice will need to know his node `identity_pubkey`, ip address, and `MerchPubkey`:

    # Get Bob's identity pubkey:
    bob$ lncli-bob getinfo
    {
    --->"identity_pubkey": <BOB_ID_PUBKEY>,
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

    # Get Bob's merchant pubkey:
    bob$ lncli-bob zkinfo
    {
    --->"merch_pubkey": <BOB_MERCHPUBKEY>,
    }

Now, Alice is ready to initiate setting up a zkchannel.

    alice$ lncli-alice openzkchannel <BOB_ID_PUBKEY>@localhost:10012 --merch_pubkey=<BOB_MERCHPUBKEY> --cust_balance=50000 --merch_balance=0 --channel_name=<CHANNEL_NAME>
    
The first argument specifies Bob's node ID public key (used for communication over the P2P network), and IP address. Notice that `localhost:10012` corresponds to the `--listen=localhost:10012` flag we set when starting the Bob `lnd` node.

*   `--merch_pubkey` Specifies Bob's bitcoin public key which will be used in the escrow transaction.
*   `--cust_balance` The amount in satoshis to fund the the customer's (Alice's) channel with.
*   `--merch_balance` The amount in satoshis to fund the merchant's (Bob's) side of the channel with.
*   `--channel_name` A unique name set by the user to identify the channel.

We now need to mine three blocks so that the channel is considered valid:

    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 3

Check that Alice<–>Bob zkchannel was created:

    alice$ lncli-alice zkchannelbalance
    {
        "zk_channel": [
            {
                "channel_name": <CHANNEL_NAME>,
                "escrow_txid": <ESCROW_TCIX>
                "local_balance": "50000",
                "remote_balance": "0"
            }
        ]
    }

### Sending a payment

Finally, to the exciting part - sending unlinkable payments! Let’s send a payment from Alice to Bob.

    alice$ lncli-alice zkpay <BOB_ID_PUBKEY>@localhost:10012 --node_key=<BOB_PUBKEY> --channel_name=<CHANNEL_NAME> --amt=1000
    {

    }

    # Check that Alice's channel balance was decremented accordingly:
    alice$ lncli-alice zkchannelbalance

    # Check that Bob's channel was credited with the payment amount:
    bob$ lncli-bob totalreceived


### Closing channels

Let’s try closing a channel. There are two ways to close a channel, depending on whether it is being initiated by the customer, Alice, or the merchant, Bob. Note that if you close the channel from Alice's node, you would have to create another channel to try closing it from Bob's node.

For Alice to close the channel:

    alice$ lncli-alice closezkchannel --channel_name=<CHANNEL_NAME>
    {

    }

We now need to mine three blocks so that the channel is considered closed, run in a separate window as the above command will not finish until these blocks are mined:

    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 3

Alternatively, for Bob, the merchant to close the channel, he should get the `escrow_tx_id` from Alice, she can get it by running `lncli-alice zkchannelbalance`, next Bob should run:

    bob$ lncli-bob merchclose --escrowtxid=<ESCROW_TX_ID>
    {

    }

Again, we would need to mine three blocks so that the channel is considered closed, run in a separate window as the above command will not finish until these blocks are mined:

    btcctl --simnet --rpcuser=kek --rpcpass=kek generate 3
