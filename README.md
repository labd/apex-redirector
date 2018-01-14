# apex-redirector


Apex-redirector is a simple utility which proxies tcp data (HTTP and HTTPS via SNI) to a target host which then can redirect the user to the correct subdomain. This solves the issue that DNS servers don't allow CNAME on the apex domain (also known as the "root domain" or "naked domain"). CNAME records are required for various cloud providers since e.g. AWS CloudFront or the ELB only have dynamic ip addresses. 

It is required to host this application on a server with a static ip address (or behind the AWS Network Load Balancer).
