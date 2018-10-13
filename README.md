# PDX multiplexer, an HTTP-CONNECT based TCP multiplexer

This service enables a blockchain node to have all of its 
TCP endpoints behind this single "external" facing HTTP 
endpoint, so that firewall traversal is less painful.

When a node belongs to multiple blockchain instances, only
this external facin HTTP endpoint needs to be exposed, not 
the detailed endpoints of each chain.

The whitelist approach effectively protects a node from 
inadvertently exposing unwanted services. 

Further, via redirect, a node's internal network and
endpoint details can be hidden from outside world, which
ensures flexibility on the node-side network implementation.

Any questions or suggestions, please contact info@pdx.ltd.
