# PDX chainmux, a lightweight whitelist-protected TCP & HTTP proxy 

In the PDX blockchain hypercloud, it enables a blockchain node to have
all of its HTTP & TCP endpoints behind this single "external" facing 
HTTP endpoint, so that firewall traversal is less painful.

The glob-matching whitelist approach effectively protects a node from 
inadvertently exposing unwanted services. 

With rewrite, a node's internal network and endpoint details can be 
hidden from the outside world, which ensures flexibility on the
node-side implementation.

Any questions or suggestions, please contact info@pdx.ltd.
