# Overview
This example illustrates how to embed zero trust connectivity into your server-side code. The server now listens on the 
Hanzo ZT overlay network and not on the layer 3, IP-based network.

This example demonstrates:
* Binding a service and listening for HTTP connections
* Accessing the service via a tunneler

## Requirements
* an Hanzo ZT network. If you do not have one, you can use one of the [quickstarts](https://netfoundry.io/docs/hanzozt/learn/quickstarts/) to set one up.
* Hanzo ZT CLI to create services and identities on the Hanzo ZT Network
* Have the appropriate [Ziti Desktop Edge](https://netfoundry.io/docs/hanzozt/reference/tunnelers/) for your operating system

## Build the examples
Refer to the [example README](../README.md) to build the SDK examples

## Setup using the Hanzo ZT CLI
These steps will configure the service using the Hanzo ZT CLI. At the end of these steps you will have created:
* a service called `simpleService`
* an identity to host (bind) the service
* an identity to connect to (dial) the service
* the service configs to connect the service to the overlay
* the service policies required to authorize the identities for bind and dial

Steps:
1. Log into Hanzo ZT. The host:port and username/password will vary depending on your network.

       zt edge login localhost:1280 -u admin -p admin
1. Run this script to create everything you need.

       cd <repo-root-dir>/example/build

       echo Create the service configs
       zt edge create config simple.hostv1 host.v1 '{"protocol":"tcp", "address":"localhost","port":'8080'}'
       zt edge create config simple.interceptv1 intercept.v1 '{"protocols":["tcp"],"addresses":["simpleService.zt"], "portRanges":[{"low":'8080', "high":'8080'}]}'

       echo Create the service
       zt edge create service simpleService --configs "simple.hostv1,simple.interceptv1" --role-attributes simple-service
       
       echo Create two identities and enroll the server
       zt edge create identity user simple-client -a simpleserver.clients -o simple-client.jwt
       zt edge create identity device simple-server -a simpleserver.servers -o simple-server.jwt
       zt edge enroll --jwt simple-server.jwt
       
       echo Create service policies
       zt edge create service-policy simple-client-dial Dial --identity-roles '#simpleserver.clients' --service-roles '#simple-service'
       zt edge create service-policy simple-client-bind Bind --identity-roles '#simpleserver.servers' --service-roles '#simple-service'
       
       echo Run policy advisor to check
       zt edge policy-advisor services
1. Run the server.

       ./simple-server simple-server.json simpleService

1. Enroll the `simple-client` client identity
   1. Refer to [enrolling documentation](https://netfoundry.io/docs/hanzozt/learn/core-concepts/identities/enrolling/) for details

1. Issue cURL commands to see the server side responses in action. There are two servers spun up by the `simple-server` 
   binary. One server is a simple HTTP server which is running on the local machine. The second server is a ztfied 
   HTTP server, this server should be accessible from the device running ZDE where you enrolled the `simple-client` 
   identity.

       # curl to the server listening on the underlay:
       curl http://localhost:8080?name=client
       
       # curl to the server listening on the overlay:
       curl http://simpleService.zt:8080?name=client

### Example output
The following is the output you'll see from the server and client side after running the previous commands.
**Server**
```
$ ./simple-server simple-server.json simpleService
listening for non-zt requests on localhost:8080
listening for requests for Ziti service simpleService
Saying hello to client, coming in from plain-internet
Saying hello to client, coming in from zt
```
**Client**
```
$ curl http://localhost:8080?name=client
Hello, client, from plain-internet

$ curl http://simpleService.zt:8080?name=client
Hello, client, from zt
```

## Teardown
Done with the example? This script will remove everything created during setup.
You will have to manually remove the identity from your Ziti Desktop Edge application.
```
zt edge login localhost:1280 -u admin -p admin

echo Removing service policies
zt edge delete service-policy simple-client-dial
zt edge delete service-policy simple-client-bind

echo Removing service configs
zt edge delete config simple.hostv1
zt edge delete config simple.interceptv1

echo Removing identities
zt edge delete identity simple-client
zt edge delete identity simple-server

echo Removing service
zt edge delete service simpleService
```
