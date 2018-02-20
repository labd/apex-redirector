# apex-redirector

Apex-redirector is a simple utility which proxies tcp data (HTTP and HTTPS via
SNI) to a target host which then can redirect the user to the correct
subdomain. This solves the issue that DNS servers don't allow CNAME on the apex
domain (also known as the "root domain" or "naked domain"). CNAME records are
required for various cloud providers since e.g. AWS CloudFront or the ELB only
have dynamic ip addresses. 

It is required to host this application on a server with a static ip address
(or behind the AWS Network Load Balancer).


# How it works

The apex-redirector first checks the value of the TXT record
``_apex_redirector.<domain>`` to see if it is allowed to act as a proxy. It uses
hmac 256 to see if the value matches the apex domain + secret key. If it a
match it then opens a connection to ``www.<domain>`` and forwards the tcp request
there (so it acts as a proxy!). The HTTP hostname header is not altered so your
target application should always redirect the user manually to ``www.<domain>``

The TXT value is a base64 encoded sha256 hmac key, use for example
http://dinochiesa.github.io/hmachash.html to calculate it.
