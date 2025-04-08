# Lantern Server Manager

## Introduction

Lantern Server Manager is a tool for managing your own Lantern servers. 
It will allow you to easily set up a server, configure it, and allow to share access to it with your friends.

## Features

1. Zero config bootstrap - if no parameters are provided, we'll automatically generate random certificates/access keys.
2. Runnable as a Docker container, single binary or a cloud Marketplace item.
3. Supports configuration via environment variables, command line arguments or a config file.
4. Creates easy access codes in its log by printing a QR code
5. Web UI
6. Console UI
7. REST API

## Flow

1. User starts the server
2. User gets a QR code in the logs
3. The QR code contains the URL to the server and the access key
4. User scans the QR code with the Lantern app
5. The app is now the MANAGER of the server
6. The app can now share access to the server with other users by calling a 'create share link' API and sending the resulting link to the user they want to grant access to
7. The link is in lantern://xxx.xxx.xxx.xxx/yyyyyy format, where 
   - xxx.xxx.xxx.xxx is the server's IP address
   - yyyyyy is the access key (the key is timestamped and expires after NN minutes)
8. The user clicks the link and is redirected to the Lantern app
9. Their app will create a private/public key pair and send the public key to the server together with the access key
10. The server will verify the access key and store the public key in its VPN 'peer' list
11. The new user can now connect to the VPN

## Notes

- The "root" access key is the one that is generated when the server is started.
- It's the only key that can be used to manage the server.
- It's stored in the server's config file and on the phone that scanned the initial QR
- If that key is lost, the server can no longer be managed and the only way to regain access is to delete the config file and start over.