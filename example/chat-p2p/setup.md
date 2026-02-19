# Purpose

This script sets up the services, policies and identities for the sdk-golang chat-p2p example.

# Prerequisites

You need at least one controller and an edge router running. for this to work.
You can use the quick-start script found [here](https://github.com/hanzozt/zt/tree/release-next/quickstart).

# Setup

## Ensure we're logged into the controller

```action:zt-login allowRetry=true
zt edge login
```

<!--action:keep-session-alive interval=1m quiet=false-->

## Remove any entities from previous runs

```action:zt
zt edge delete service chat-p2p
zt edge delete identities user1 user2 user3 user4
zt edge delete service-policies chat-p2p-dial chat-p2p-bind
zt edge delete edge-router-policy chat-p2p
zt edge delete service-edge-router-policy chat-p2p
```

## Create and enroll the client app identity

```action:zt
zt edge create identity user user1 -a chat-p2p -o user1.jwt
zt edge enroll --rm user1.jwt

zt edge create identity user user2 -a chat-p2p -o user2.jwt
zt edge enroll --rm user2.jwt

zt edge create identity user user3 -a chat-p2p -o user3.jwt
zt edge enroll --rm user3.jwt

zt edge create identity user user4 -a chat-p2p -o user4.jwt
zt edge enroll --rm user4.jwt
```

## Configure the dial and bind service policies

```action:zt
zt edge create service-policy chat-p2p-dial Dial --service-roles '#chat-p2p' --identity-roles '#chat-p2p'
zt edge create service-policy chat-p2p-bind Bind --service-roles '#chat-p2p' --identity-roles '#chat-p2p'
```

## Configure the edge router policy

```action:zt
zt edge create edge-router-policy chat-p2p --edge-router-roles '#all' --identity-roles '#chat-p2p'
```

## Configure the service edge router policy

```action:zt
zt edge create service-edge-router-policy chat-p2p --edge-router-roles '#all' --service-roles '#chat-p2p'
```

## Create the service

```action:zt
zt edge create service chat-p2p -a chat-p2p
```

# Summary

After you've configured the service side, you should now be to run the chat-p2p client for
each of the four configured identities as follows.

```
chat-p2p -i user1.json
chat-p2p -i user2.json
chat-p2p -i user3.json
chat-p2p -i user4.json
```

Note that you will need to run each chat-p2p command in a separate terminal.