# Parallelcoin SPV Wallet

This is a light wallet client that implements a block sync protocol that indexes enough information to be able to verify new blocks and update an index of all address input and output cumulative totals, such that if needed the full block could be easily located and the relevant transaction data immediately extracted.

It implements multiple, configurable RPC endpoints for integration with a front end application, including a RESTful API that can be accessed with raw information through a web browser, once authenticated, primarily for monitoring and configuration in the absence of a front end.

Most of the time for the personal wallet use case it will not be necessary to send out queries to the network, but it always will reveal locations of transaction origins potentially when making transactions. So for this reason, the nodes find peers of the same type and relay outbound transaction messages via 3 hop location obfuscation using layers of encryption, though the first hop always knows they are first hop and progressively throttles input from nodes as their frequency of transaction sends increases, delaying the transmision of the messages, not stopping it entirely unless it is a flood, and ban times increasing the more times they misbehave.

This will be the first light blockchain wallet that fully preserves privacy and should be able to even run on mobile phones of current mid range capability.