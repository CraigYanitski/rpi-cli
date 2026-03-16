rpi-cli
===

This is meant to be a cli interface to the Raspberry Pi Connect service.
It uses `connect.raspberrypi.com` as a signaling service to negotiate
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

Currently this app is setup to run in its own folder.
Until it is further developed, clone and run the client by running,

```bash
git clone https://github.com/CraigYanitski/rpi-cli
cd rpi-cli
rpi-cli
```

Development
---

This is in the early stages at the moment.
I started with a rather lengthy bash script to make sure I get the appropriate 
responses, but I have since moved everything into a Golang client.
Now the client is functional, but there are many quality of life updates to be made.
Please feel free to fork this repository and contribute to its development.

