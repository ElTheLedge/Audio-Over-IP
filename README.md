# Realtime Audio over IP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FLVH-IT%2FAudio-Over-IP.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FLVH-IT%2FAudio-Over-IP?ref=badge_shield)

This package uses the following external dependencies:
* [go-ole](https://github.com/go-ole/go-ole), which is licenced under an [MIT-like](https://github.com/go-ole/go-ole/blob/master/LICENSE) license.
* [go-wav](https://github.com/moutend/go-wav), which is licenced under an [MIT-like](https://github.com/moutend/go-wav/blob/master/LICENSE) license.
* [go-wca](https://github.com/moutend/go-wca), which is licenced under an [MIT-like](https://github.com/moutend/go-wca/blob/develop/LICENSE) license.

## What does it do?
This program captures the system Audio of a Windows Computer and sends it to another Windows Computer over the network in almost realtime.  
**BEWARE:** The Audiostream is not encrypted. If you plan using this Software to send an Audiostream over an untrusted Network, such as the internet for example, you should use something like an SSH tunnel to encrypt the stream.  


## Usage
Download the latest release and contine with "How to setup". There are no prerequisites when using the already compiled binary file.  
If you want to however, you can also compile the code yourself as explained below.  


### How to setup
You have to put the client executable on the computer on which you want to play an audiostream, and the server executable has to be on the computer from which you want to stream the system audio.  
You can then use the flags listed below to start the server and connect to it with a client.

### Flags you can use
Client:
* **-e** :   server address to connect to (ex: -e 127.0.0.1:4040), Has to be specified
* **-v** :   displays more info about the stream and the audio setup  

Server:  
* There are currently no flags for the server. It automatically starts listening on port 4040 and waits for a client to connect. Only one client is supported at a time per server.  

### Limitations
* Both devices have to use the same audio device settings (ex: 16 Bit, 48000 Hz), as there is no audio resampler implemented yet  
* When the connection gets instable, the audio gets delayed by the amount of time the connection dropped  

## Compilation
### Prerequisites for compilation
Go 1.15 (https://golang.org/dl/)  
You'll get the rest when trying to compile  


### How to compile
You need to follow the next steps for each the server and the client:  
Open a command prompt in the source directory and you should be able to install all dependencies by executing this command inside the source folder: 
```sh
go get -d ./...
```
Then simply type:
```sh
go build
```
It will then try to compile and tell you wether there are dependencies which are still missing.
If so, you need to install them each like this for example: 
```sh
go get github.com/go-ole/go-ole
```
Then rerun "go build" and your executable should be compiled in the source directory.


## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FLVH-IT%2FAudio-Over-IP.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FLVH-IT%2FAudio-Over-IP?ref=badge_large)