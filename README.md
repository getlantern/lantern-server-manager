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


## Installation

### Docker

When running inside Docker container, we don't want to use random ports, so we need to specify the ports we want to use. 

```bash
docker run -d \
  --name lantern-server-manager \
  -p 8080:8080 \
  -p 1234:1234 \
  -v /path/to/config:/config \
  getlantern/lantern-server-manager -d /config --vpn-port 1234 --api-port 8080 serve
```

### Digital Ocean
1. Create a droplet using the Lantern Server Manager image from Marketplace
2. Make sure to add your SSH key and open all ports to the instance
3. When instance is up, use it's public IP address to fetch the initial token.
```shell
ssh root@xxxxxxxx sudo journalctl -u lantern-server-manager
```

### Google Cloud

1. Start the instance using Lantern Server Manager image
2. The image has its own firewall installed, so you need to open the ports you want to use.
```shell
gcloud compute firewall-rules create allow-all-to-instance \
    --direction=INGRESS \
    --priority=1000 \
    --network=lanternet \
    --action=ALLOW \
    --rules=all \
    --target-tags=allow-all-traffic \
    --source-ranges=0.0.0.0/0
gcloud compute instances add-tags instance-20250414-160036 --tags=allow-all-traffic     --zone=us-west1-c
``` 
3. Now you can fetch the initial token
```shell
gcloud compute instances add-tags instance-20250414-160036     --tags=allow-all-traffic     --zone=us-west1-c sudo journalctl -u lantern-server-manager
```

### AWS
1. Create an EC2 instance using the Lantern Server Manager image from Marketplace
2. When creating the instance, select "vendor suggested security group".
3. This will create a security group that allows all traffic to the instance.
4. When instance is up, use it's public IP address to fetch the initial token.
```shell
ssh ec2-user@xxxxxxxx sudo journalctl -u lantern-server-manager
```

## API Usage

1. Start the server. On startup, it will generate a random access key and print it in the logs. It will also let you know you public IP address and the API port.
2. `export API_KEY=your_access_key`
3. Request your own VPN configuration `curl -vk https://xxx.xxx.xxx.xxx:yyyy/api/v1/connect-config?token=$API_KEY`
4. This config can be used with SingBox or with LanternVPN
5. To share access with other users, you can create a share link by calling the `createShareLink` API. This will generate a link that you can send to the user you want to share access with. Each link has a unique name associated with that user.
6. `curl -vk https://xxx.xxx.xxx.xxx:yyyy/api/v1/share-link/unique-user-name?token=$API_KEY`
7. This will generate a link in the format `lantern://xxx.xxx.xxx.xxx/yyyyyy` where `xxx.xxx.xxx.xxx` is the server's IP address and `yyyyyy` is the access key. The access key is timestamped and expires after NN minutes.
8. You can send this link to the user you want to share access with. When they click the link, it will open the Lantern app and prompt them to connect to the server.
9. The user's Lantern VPN app will issue the same  `/connect-config` request but will use the access key from the link instead of the root access key.

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



