rpi-cli
===

This is meant to be a cli interface to the Raspberry Pi Connect service.
It uses [connect.raspberrypi.com](https://connect.raspberrypi.com) as a signaling service to negotiate
a direct UDP connection to your raspberry pi device.
It will eventually perform UDP hole-punching on its own.

Motivation
---

I have some raspberry pi devices that I want to access from around the world, 
so I appreciate that Raspberry Pi Connect exists as a service, and that there 
is an option to connect directly to the shell.
What I find lacking is that I need to do all of this in the browser.
I much prefer to make an SSH connection, but it is difficult for networks 
where you need to traverse both Network Address Translation (NAT) and Carrier-grade NAT (CGNAT).
Therefore, I figured I would write `rpi-cli` to establish the WebRTC connection 
with my raspberry pi device.

Unlike the port forwarding required for running a VPN via [WireGuard](www.wireguard.com), 
`rpi-connect` relies on WebRTC to use Interactive Connectivity Establishment (ICE) to 
find the most direct peer-to-peer connection.
More importantly, it uses its own Session Traversal Utilities for NAT (STUN) and
Traversal Using Relays around NAT (TURN) servers to ensure a robust and secure connection.

Installation
---

A caution that `rpi-cli` relies on a private file relative to the home directory in order to 
manage cookies (unless you would like to continuously log into your RPI account).
This repo can be cloned and installed by running,

```bash
git clone https://github.com/CraigYanitski/rpi-cli
cd rpi-cli
./install
```

The `rpi-cli` binary will be built and moved to `~/.local/bin/`, which should be in your 
executable path.

Quick Start
---

`rpi-cli` is simple to use so long as you have a Raspberry Pi account.
The client will first ask you to sign in if it has not already done so.
I of course cannot show my login details, but below is an example of use provided you have 
already logged in using the `rpi-cli` client.

![quick start animation](./quick-start.gif)

Development
---

This is in the early stages at the moment.
I started with a rather lengthy bash script to make sure I get the appropriate 
responses, but I have since moved everything into a Golang client.
Now the client is functional, but there are many quality of life updates to be made.
Please feel free to fork this repository and contribute to its development.

Please also feel free suggest improvements that should be made.

