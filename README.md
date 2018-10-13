# PDX chainmux, an HTTP-CONNECT based, whitelisted TCP multiplexer

This service enables a blockchain node to have all of its 
TCP endpoints behind this single "external" facing HTTP 
endpoint, which makes firewall traversal a easy task.

When a node belongs to multiple blockchain instances, only
this external facin HTTP endpoint needs to expose, not the 
detailed endpoints of each chain.

The whitelist approach effectively protects a node from 
inadvertently exposing unwanted services. 

Further, via redirect, a node's internal networking details
can be hidden from outside world, which ensures adquate 
flexibility on the on-node networking layout.

Any questions or suggestions, please contact info@pdx.ltd.
