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
where you need to traverse NAT and CG-NAT.
Therefore, I figured I would write `rpi-cli` to establish the WebRTC connection 
with my raspberry pi device.

Development
---

This is in the early stages at the moment.
I started with a rather lengthy bash script to make sure I get the appropriate 
responses, but I have since moved everything into a Golang client.

